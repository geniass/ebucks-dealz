package scraper

import (
	"fmt"
	"regexp"
	"time"

	"github.com/gocolly/colly"
)

type ProductPageCallbackFunc func(p Product)

// cacheDir can be empty to disable caching.
func NewScraper(cacheDir string, threads int, callback ProductPageCallbackFunc) Scraper {
	async := false
	if threads > 1 {
		async = true
	}

	c := colly.NewCollector(
		colly.AllowedDomains("www.ebucks.com"),
		colly.URLFilters(
			regexp.MustCompile(`https://www\.ebucks\.com/web/shop/shopHome\.do`),
			regexp.MustCompile(`https://www\.ebucks\.com/web/shop/categorySelected\.do.*`),
			regexp.MustCompile(`https://www\.ebucks\.com/web/shop/productSelected\.do.*`),
		),
		colly.CacheDir(cacheDir),
		colly.Async(async),
	)

	c.Limit(&colly.LimitRule{
		DomainGlob:  "*",
		Parallelism: threads,
		Delay:       1 * time.Second,
		RandomDelay: 1 * time.Second,
	})

	c.OnHTML("a[href]", func(e *colly.HTMLElement) {
		link := e.Attr("href")
		c.Visit(e.Request.AbsoluteURL(link))
	})

	c.OnHTML(".product-detail-frame", func(e *colly.HTMLElement) {
		p := Product{
			URL:        e.Request.URL.String(),
			Name:       e.ChildText("h2.product-name"),
			Price:      e.ChildText("#randPrice"),
			Savings:    e.ChildText(".was-price > strong:nth-child(1) > span:nth-child(1)"),
			Percentage: e.ChildText("table#discount-table tr:last-child td.discount-icon p.percentage"),
		}

		callback(p)
	})

	c.OnRequest(func(r *colly.Request) {
		fmt.Println("Visiting", r.URL.String())
	})

	return Scraper{colly: c}
}

func (s Scraper) Start() error {
	if err := s.colly.Visit("https://www.ebucks.com/web/shop/shopHome.do"); err != nil {
		return err
	}
	s.colly.Wait()
	return nil
}
