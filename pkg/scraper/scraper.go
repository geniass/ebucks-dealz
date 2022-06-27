package scraper

import (
	"errors"
	"fmt"
	"log"
	"math"
	"net"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gocolly/colly/v2"
	"github.com/gocolly/colly/v2/queue"
)

const maxNumRetries int = 5

var categorySelectedUrlCleanerRegex = regexp.MustCompile(`(.*categorySelected\.do).*(catId=\d+).*`)

type ProductPageCallbackFunc func(p Product)

var ErrRedirectToErrorPage = errors.New("redirected to error page")

const ctxScrapedDataKey string = "scraped"

var urlProdIdAndCatIdRegex = regexp.MustCompile(`productSelected\.do\?prodId=(\d+)&catId=(\d+)`)
var randsRegex = regexp.MustCompile(`R([\d\s]+(\.\d+)?)`)
var whitespaceRegex = regexp.MustCompile(`\s`)

// cacheDir can be empty to disable caching.
func NewScraper(cacheDir string, threads int, callback ProductPageCallbackFunc) Scraper {

	options := []colly.CollectorOption{
		colly.AllowedDomains("www.ebucks.com"),
		colly.URLFilters(
			regexp.MustCompile(`.*/web/shop/shopHome\.do`),
			regexp.MustCompile(`.*/web/shop/categorySelected\.do.*`),
			regexp.MustCompile(`.*/web/shop/productSelected(Discount)?\.do.*`),
		),
		colly.UserAgent("Mozilla/5.0 (Windows NT x.y; Win64; x64; rv:10.0) Gecko/20100101 Firefox/10.0"),
	}

	if cacheDir != "" {
		options = append(options, colly.CacheDir(cacheDir))
	}

	// InMemoryQueueStorage Init can't fail
	q, _ := queue.New(
		threads,
		&StackQueueStorage{},
	)
	s := Scraper{
		startingURL: "https://www.ebucks.com/web/shop/shopHome.do",
		colly:       colly.NewCollector(options...),
		q:           q,
		mutex:       &sync.Mutex{},
		urlBackoffs: make(map[string]int),
		links:       make(map[string]int),
		scraped:     make(map[string]int),
	}

	// somehow cookies are causing weird concurrency issues where the wrong response body gets used
	s.colly.DisableCookies()

	s.colly.WithTransport(&http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   300 * time.Second,
			KeepAlive: 300 * time.Second,
		}).DialContext,
		MaxIdleConns:          100,
		IdleConnTimeout:       900 * time.Second,
		TLSHandshakeTimeout:   300 * time.Second,
		ExpectContinueTimeout: 100 * time.Second,
		ResponseHeaderTimeout: 300 * time.Second,
	})

	// the ebucks website redirects to a generic error page on error (including "not found" and "service unavailable")
	s.colly.SetRedirectHandler(func(req *http.Request, via []*http.Request) error {
		if strings.Contains(req.URL.Path, "globalExceptionPage.jsp") {
			return fmt.Errorf("not following redirect (implies error) %q : %+v : %w", req.URL.String(), req.Header, ErrRedirectToErrorPage)
		}

		vias := []string{}
		for _, v := range via {
			vias = append(vias, v.URL.String())
		}
		fmt.Fprintf(os.Stderr, "Redirecting %s -> %s (%d redirects)\n", strings.Join(vias, " -> "), req.URL.String(), len(via))

		if len(via) >= 10 {
			return errors.New("stopped after 10 redirects")
		}
		return nil
	})

	s.colly.OnError(func(r *colly.Response, err error) {
		// exponential backoff
		s.mutex.Lock()
		s.urlBackoffs[r.Request.URL.String()]++
		numRetries := s.urlBackoffs[r.Request.URL.String()]
		s.mutex.Unlock()

		if errors.Is(err, ErrRedirectToErrorPage) {
			// no need to retry because when we get redirected to the error page it means that page is completely broken
			log.Println("Ignoring page due to redirect error: %w", err)
			return
		}

		if r.StatusCode >= 400 && r.StatusCode < 500 {
			log.Printf("Ignoring page due to Not Found (Page=%q): %s\n", r.Request.URL, err)
			return
		}

		if numRetries > maxNumRetries {
			log.Fatalf("Max retries (%d) exceeded for URL %q\n", maxNumRetries, r.Request.URL.String())
			return
		}

		duration := time.Duration(math.Pow(2, float64(numRetries))) * time.Second
		fmt.Fprintf(os.Stderr, "ERROR: Request %q [%d] failed, retrying after %.0f s: %v\n", r.Request.URL.String(), r.StatusCode, duration.Seconds(), err)
		time.Sleep(duration)
		if err := r.Request.Retry(); err != nil {
			fmt.Fprintln(os.Stderr, "ERROR while retrying:", err)
		}

		r.Ctx.Put("attempts", numRetries)
	})

	s.urlChan = make(chan string)
	go func() {
		f, err := os.Create("urls.txt")
		if err != nil {
			log.Fatal(err)
		}
		defer f.Close()
		for url := range s.urlChan {
			f.WriteString(url + "\n")
			f.Sync()
		}
	}()

	s.colly.OnHTML("a[href]", func(e *colly.HTMLElement) {
		link := e.Request.AbsoluteURL(e.Attr("href"))
		link = cleanCategorySelectedUrl(link)
		err := s.visit(link)

		if err == nil {
			fmt.Println("FOUND LINK", e.Request.AbsoluteURL(link), e.ChildAttr("img", "alt"))
			s.mutex.Lock()
			s.links[link] = 1
			s.mutex.Unlock()

			s.urlChan <- link + " " + e.Request.URL.String()
		} else if !(errors.Is(err, colly.ErrAlreadyVisited) || errors.Is(err, colly.ErrNoURLFiltersMatch) || errors.Is(err, colly.ErrMissingURL)) {
			fmt.Fprintln(os.Stderr, "ERROR", err, link)
		}
	})

	s.colly.OnHTML("form[name=productOptionsBean]", func(e *colly.HTMLElement) {

		// sanity check: URL IDs must match hidden form inputs otherwise we somehow ended up with the wrong page (?!)
		urlProdId := e.Request.URL.Query().Get("prodId")
		urlCatId := e.Request.URL.Query().Get("catId")
		pid := e.ChildAttr("input[name=prodId]", "value")
		cid := e.ChildAttr("input[name=catId]", "value")
		if pid != urlProdId || cid != urlCatId {
			log.Fatalf("prodId or catId mismatch! pid: (formPID=%q urlPID=%q) cid: (formCID=%q urlCID=%q)\n", pid, urlProdId, cid, urlCatId)
		}

		fmt.Printf("Found product: URL=%q NAME=%q\n", e.Request.URL.String(), e.ChildText("h2.product-name"))
		// if e.Request.URL.String() == "https://www.ebucks.com/web/shop/productSelected.do?prodId=496816900&catId=1158501813" {
		// 	log.Fatal(e.Response.Save("/tmp/index.html"))
		// }
		// https://www.ebucks.com/web/shop/productSelected.do?prodId=496816900&catId=1158501813

		if e.Request.URL.String() == "https://www.ebucks.com/web/shop/productSelected.do?prodId=1173295004&catId=704981826" {
			fmt.Println("HERE")
		}

		a := e.Response.Ctx.GetAny("attempts")
		if a != nil && a.(int) != 0 {
			fmt.Println("RETRYING:", e.Request.URL.String())
		}

		price := float64(-1)
		{
			priceString := e.ChildText("#randPrice")
			if priceString != "" {
				f, err := parseRands(priceString)
				if err != nil {
					fmt.Printf("Error parsing price (%q): %s\n", priceString, err)
				} else {
					price = f
				}
			}
		}

		savings := float64(0)
		{
			savingsString := e.ChildText(".was-price .randValue")
			if savingsString != "" {
				f, err := parseRands(savingsString)
				if err != nil {
					fmt.Printf("Error parsing savings (%q): %s\n", savingsString, err)
				} else {
					savings = f
				}
			}
		}

		p := Product{
			URL:     e.Request.URL.String(),
			Name:    e.ChildText("h2.product-name"),
			ProdID:  e.Request.URL.Query().Get("prodId"),
			CatID:   e.Request.URL.Query().Get("catId"),
			Price:   price,
			Savings: savings,
		}

		fmt.Printf("Found product: Name=%q URL=%q\n", p.Name, p.URL)

		// its no longer json, they now sometimes return html fragments
		// so we have to make a request to the below url and if stuff is returned its on discount
		// which means we always have to make the request, so just do it here
		// queue fetching the HTML table page fragment for this product (for prices etc.) if the product is discounted
		link := e.Request.URL.String()
		if urlProdIdAndCatIdRegex.MatchString(link) {
			replaced := urlProdIdAndCatIdRegex.ReplaceAllString(link, "productSelectedDiscount.do?prodId=$1&catId=$2")
			log.Printf("Fetching discounts: Name=%q URL=%q\n", p.Name, replaced)
			e.Request.Ctx.Put(ctxScrapedDataKey, p)
			e.Request.Visit(replaced)
		}
	})

	s.colly.OnRequest(func(r *colly.Request) {
		fmt.Println("Visiting", r.URL.String())

		// these headers are very important for some reason
		r.Headers.Add("Cookie", "js=1637881630272")
		r.Headers.Add("Referer", r.URL.String())
	})

	s.colly.OnHTML("table#discount-table", func(e *colly.HTMLElement) {
		// try get the partial product info that was scraped and continue parsing the partial HTML response
		c, ok := e.Request.Ctx.GetAny(ctxScrapedDataKey).(Product)
		if !ok {
			log.Printf("WARNING: OnHTML(table#discount-table): could not get partial product info from ctx: URL=%q\n", e.Request.URL)
			return
		}

		discounts := []ebucksDiscount{}
		e.ForEach("div > table > tbody", func(i int, h *colly.HTMLElement) {
			d := ebucksDiscount{
				Level:         i + 1,
				Percent:       parsePercentage(h.ChildText("p.percentage")),
				EbucksPrice:   parseEbucksValue(h.ChildText("td.col2 > span.eBucksValue")),
				EbucksSavings: parseEbucksValue(h.ChildText("td.col4 > span.eBucksValue")),
			}
			discounts = append(discounts, d)
		})

		discount := discounts[len(discounts)-1]
		c.Percentage = float64(discount.Percent)
		c.Price = discount.RandPrice()
		c.Savings = discount.RandSavings()

		callback(c)
	})

	s.colly.OnResponse(func(r *colly.Response) {
		if !strings.Contains(r.Request.URL.Path, "productSelectedDiscount.do") {
			return
		}

		// need to test if the response contains an HTML table containing discount info
		// if there is, we don't do the callback here because it will be called by the HTML handler for the discount table
		if !strings.Contains(string(r.Body), "table") {
			fmt.Println(string(r.Body))
			// no discount
			// try get the partial product info that was scraped
			c, ok := r.Ctx.GetAny(ctxScrapedDataKey).(Product)
			if !ok {
				log.Printf("WARNING: OnResponse: could not get partial product info from ctx: URL=%q\n", r.Request.URL)
				return
			}
			callback(c)
			return
		}
		log.Println("DISCOUNT!", r.Request.URL)
	})

	return s
}

func (s Scraper) EnableLimits() {
	s.colly.Limit(&colly.LimitRule{
		DomainGlob:  "*",
		Parallelism: s.q.Threads,
		Delay:       2 * time.Second,
		RandomDelay: 5 * time.Second,
	})
}

func (s Scraper) Start() error {
	if err := s.visit(s.startingURL); err != nil {
		return err
	}

	if err := s.q.Run(s.colly); err != nil {
		return err
	}

	s.colly.Wait()
	close(s.urlChan)
	time.Sleep(10 * time.Second)
	f, err := os.Create("links.txt")
	if err != nil {
		log.Fatal(err)
	}
	for k := range s.links {
		f.WriteString(k + "\n")
	}
	f.Close()

	f, err = os.Create("scraped.txt")
	if err != nil {
		log.Fatal(err)
	}
	for k := range s.scraped {
		f.WriteString(k + "\n")
	}
	f.Close()

	return nil
}

func (s Scraper) visit(url string) error {
	if visited, err := s.colly.HasVisited(url); err != nil {
		return err
	} else if visited {
		return colly.ErrAlreadyVisited
	}

	for _, f := range s.colly.URLFilters {
		if f.MatchString(url) {
			return s.q.AddURL(url)
		}
	}
	return colly.ErrNoURLFiltersMatch
}

func parsePercentage(p string) int {
	s := strings.ReplaceAll(p, "%", "")
	i, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return i
}

func parseEbucksValue(s string) int {
	s = strings.ReplaceAll(s, "eB", "")
	s = strings.ReplaceAll(s, " ", "")
	i, err := strconv.Atoi(s)
	if err != nil {
		return -1
	}
	return i
}

// categorySelected.do URLs sometimes contain random cruft that break the already-visited list and/or cause bad results to be returned
// e.g. https://www.ebucks.com/web/shop/categorySelected.do;jsessionid=E1FECBC2B41C4EBBE86854E78CD8A882?catId=300&extraInfo=cellphone_number
func cleanCategorySelectedUrl(url string) string {
	matches := categorySelectedUrlCleanerRegex.FindStringSubmatch(url)
	if len(matches) != 3 {
		return url
	}
	return matches[1] + "?" + matches[2]
}

func parseRands(s string) (float64, error) {
	matches := randsRegex.FindStringSubmatch(s)
	if len(matches) < 2 {
		return 0, fmt.Errorf("does not match the rands parsing regex")
	}
	s = matches[1]
	s = whitespaceRegex.ReplaceAllLiteralString(s, "")
	return strconv.ParseFloat(s, 64)
}
