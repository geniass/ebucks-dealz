package scraper

import (
	"fmt"
	"math"
	"regexp"
	"sync"
	"time"

	"github.com/gocolly/colly/v2"
)

type ProductPageCallbackFunc func(p Product)

// cacheDir can be empty to disable caching.
func NewScraper(cacheDir string, threads int, callback ProductPageCallbackFunc) Scraper {
	async := false
	if threads > 1 {
		async = true
	}

	options := []colly.CollectorOption{
		colly.AllowedDomains("www.ebucks.com"),
		colly.URLFilters(
			regexp.MustCompile(`https://www\.ebucks\.com/web/shop/shopHome\.do`),
			regexp.MustCompile(`https://www\.ebucks\.com/web/shop/categorySelected\.do.*`),
			regexp.MustCompile(`https://www\.ebucks\.com/web/shop/productSelected\.do.*`),
		),
		colly.UserAgent("Mozilla/5.0 (Windows NT x.y; Win64; x64; rv:10.0) Gecko/20100101 Firefox/10.0"),
	}

	if async {
		options = append(options, colly.Async())
	}

	if cacheDir != "" {
		options = append(options, colly.CacheDir(cacheDir))
	}

	s := Scraper{
		colly:       colly.NewCollector(options...),
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

	s.colly.OnError(func(r *colly.Response, err error) {
		// exponential backoff
		s.mutex.Lock()
		s.urlBackoffs[r.Request.URL.String()]++
		numRetries := s.urlBackoffs[r.Request.URL.String()]
		s.mutex.Unlock()

		duration := time.Duration(math.Pow(2, float64(numRetries))) * time.Second
		fmt.Println("ERROR: Request URL:", r.Request.URL, "failed with response:", r, "\nError:", err, "\nRetrying after %v", duration)
		time.Sleep(duration)
		if err := r.Request.Retry(); err != nil {
			fmt.Println(err)
		}
	})

	s.colly.OnHTML("a[href]", func(e *colly.HTMLElement) {
		link := e.Attr("href")
		s.colly.Visit(e.Request.AbsoluteURL(link))
	})

	s.colly.OnHTML(".product-detail-frame", func(e *colly.HTMLElement) {
		p := Product{
			URL:        e.Request.URL.String(),
			Name:       e.ChildText("h2.product-name"),
			Price:      e.ChildText("#randPrice"),
			Savings:    e.ChildText(".was-price > strong:nth-child(1) > span:nth-child(1)"),
			Percentage: e.ChildText("table#discount-table tr:last-child td.discount-icon p.percentage"),
		}

		fmt.Println("Found product:", p.Name, p.URL)

		callback(p)
	})

	s.colly.OnRequest(func(r *colly.Request) {
		fmt.Println("Visiting", r.URL.String())
	})

	return s
}

func (s Scraper) Start() error {
	if err := s.colly.Visit("https://www.ebucks.com/web/shop/shopHome.do"); err != nil {
		return err
	}
	s.colly.Wait()
	return nil
}
