package scraper

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"math"
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
			regexp.MustCompile(`https://www\.ebucks\.com/web/eBucks$`),
			regexp.MustCompile(`https://www\.ebucks\.com/web/shop/shopHome\.do`),
			regexp.MustCompile(`https://www\.ebucks\.com/web/shop/categorySelected\.do.*`),
			regexp.MustCompile(`https://www\.ebucks\.com/web/shop/productSelected(Json)?\.do.*`),
		),
		colly.UserAgent("Mozilla/5.0 (Windows NT x.y; Win64; x64; rv:10.0) Gecko/20100101 Firefox/10.0"),
	}

	if cacheDir != "" {
		options = append(options, colly.CacheDir(cacheDir))
	}

	// InMemoryQueueStorage Init can't fail
	q, _ := queue.New(
		threads,
		&queue.InMemoryQueueStorage{MaxSize: 10000},
	)
	s := Scraper{
		colly:       colly.NewCollector(options...),
		q:           q,
		mutex:       &sync.Mutex{},
		urlBackoffs: make(map[string]int),
	}

	// somehow cookies are causing weird concurrency issues where the wrong response body gets used
	s.colly.DisableCookies()

	s.colly.Limit(&colly.LimitRule{
		DomainGlob:  "*",
		Parallelism: threads,
		Delay:       1 * time.Second,
		RandomDelay: 1 * time.Second,
	})

	// the ebucks website redirects to a generic error page on error (including "not found" and "service unavailable")
	s.colly.SetRedirectHandler(func(req *http.Request, via []*http.Request) error {
		if strings.Contains(req.URL.Path, "globalExceptionPage.jsp") {
			return fmt.Errorf("not following redirect (implies error) %q : %+v : %w", req.URL.String(), req.Header, ErrRedirectToErrorPage)
		}
		fmt.Fprintf(os.Stderr, "Redirecting %s -> %s (%d redirects)\n", via[0].URL.String(), req.URL.String(), len(via))

		for _, v := range via {
			if v.Response != nil {
				body, err := ioutil.ReadAll(v.Response.Body)
				if err != nil {
					body = []byte("<ERROR READING BODY>")
				}
				return fmt.Errorf("redirect means something is wrong: %+v\n%s", v.Response.Header, string(body))
			}
		}

		return fmt.Errorf("redirect but Response is nil for some reason: %+v", via)
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

		if numRetries > maxNumRetries {
			log.Fatalf("Max retries (%d) exceeded for URL %q\n", maxNumRetries, r.Request.URL.String())
			return
		}

		duration := time.Duration(math.Pow(2, float64(numRetries))) * time.Second
		fmt.Fprintf(os.Stderr, "ERROR: Request %q [%d] failed, retrying after %.0f s: %v", r.Request.URL.String(), r.StatusCode, duration.Seconds(), err)
		time.Sleep(duration)
		if err := r.Request.Retry(); err != nil {
			fmt.Fprintln(os.Stderr, "ERROR while retrying:", err)
		}
	})

	s.colly.OnHTML("a[href]", func(e *colly.HTMLElement) {
		link := e.Request.AbsoluteURL(e.Attr("href"))
		link = cleanCategorySelectedUrl(link)
		err := s.visit(link)
		if err != nil && !(errors.Is(err, colly.ErrAlreadyVisited) || errors.Is(err, colly.ErrNoURLFiltersMatch) || errors.Is(err, colly.ErrMissingURL)) {
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
			log.Fatalf("prodId or catId mismatch! pid=%s (%s) cid=%s (%s)\n", pid, cid, urlProdId, urlCatId)
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
			Price:   price,
			Savings: savings,
		}

		// the ebucks site now splits the data into 2 sections
		// the static html contains the product name and description etc.
		// it also sometimes contains an empty HTML table for the discount prices
		// in this case there is also a JSON page available containing the actual discount prices
		hasDiscount := false
		e.ForEach(`table#discount-table`, func(i int, h *colly.HTMLElement) {
			hasDiscount = true
		})

		fmt.Printf("Found product: Name=%q Discounted=%v URL=%q\n", p.Name, hasDiscount, p.URL)

		if hasDiscount {
			// queue fetching the JSON page for this product (for prices etc.) if the product is discounted
			fmt.Println("Fetching discounts...")
			link := e.Request.URL.String()
			if urlProdIdAndCatIdRegex.MatchString(link) {
				replaced := urlProdIdAndCatIdRegex.ReplaceAllString(link, "productSelectedJson.do?prodId=$1&catId=$2")
				e.Request.Ctx.Put(ctxScrapedDataKey, p)
				e.Request.Visit(replaced)
			}
		} else {
			// no more data to fetch, we are done
			callback(p)
		}
	})

	s.colly.OnRequest(func(r *colly.Request) {
		fmt.Println("Visiting", r.URL.String())
		r.Headers.Add("Cookie", "js=1637881630272")
	})

	s.colly.OnResponse(func(r *colly.Response) {
		// try get the partial product info that was scraped and continue parsing the JSON response
		c, ok := r.Ctx.GetAny(ctxScrapedDataKey).(Product)
		if !ok {
			return
		}

		pd := ebucksProductDetail{}
		if err := json.Unmarshal(r.Body, &pd); err != nil {
			fmt.Println("ERROR: fetching json product detail:", err)
			r.Request.Abort()
		}

		discount := pd.ProductDetail.Discount[len(pd.ProductDetail.Discount)-1]
		c.Percentage = float64(discount.Percent)
		c.Price = discount.RandPrice()
		c.Savings = discount.RandSavings()

		callback(c)
	})

	return s
}

func (s Scraper) Start() error {
	if err := s.visit("https://www.ebucks.com/web/shop/shopHome.do"); err != nil {
		return err
	}

	if err := s.q.Run(s.colly); err != nil {
		return err
	}

	s.colly.Wait()

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
