package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/gocolly/colly"
)

var safeFilenameReplaceRegex = regexp.MustCompile(`[^a-zA-Z0-9-]+`)

type Product struct {
	Name       string
	Price      string
	Savings    string
	Percentage string
	RunDate    time.Time
}

func main() {

	c := colly.NewCollector(
		colly.AllowedDomains("www.ebucks.com"),
		colly.URLFilters(
			regexp.MustCompile(`https://www\.ebucks\.com/web/shop/shopHome\.do`),
			regexp.MustCompile(`https://www\.ebucks\.com/web/shop/categorySelected\.do.*`),
			regexp.MustCompile(`https://www\.ebucks\.com/web/shop/productSelected\.do.*`),
		),
		colly.CacheDir("./cache"),
	)

	c.OnHTML("a[href]", func(e *colly.HTMLElement) {
		link := e.Attr("href")
		c.Visit(e.Request.AbsoluteURL(link))
	})

	runDate := time.Now()
	c.OnHTML(".product-detail-frame", func(e *colly.HTMLElement) {
		p := Product{
			Name:       e.ChildText("h2.product-name"),
			Price:      e.ChildText("#randPrice"),
			Savings:    e.ChildText(".was-price > strong:nth-child(1) > span:nth-child(1)"),
			Percentage: e.ChildText("table.discount-table tr:last-child td.discount-icon p.percentage"),
			RunDate:    runDate,
		}

		dirname := filepath.Join("data", runDate.Format("2006-01-02T15-04-05Z-0700"))
		if err := os.MkdirAll(dirname, os.ModeDir|0755); err != nil {
			log.Fatal(err)
		}

		name := safeFilenameReplaceRegex.ReplaceAllString(p.Name, "-")
		fmt.Println(name)
		f, err := os.Create(filepath.Join(dirname, name+".json"))
		if err != nil {
			log.Fatal(err)
		}
		defer f.Close()

		encoder := json.NewEncoder(f)
		if err := encoder.Encode(p); err != nil {
			log.Fatal(err)
		}
	})

	c.OnRequest(func(r *colly.Request) {
		fmt.Println("Visiting", r.URL.String())
	})

	if err := c.Visit("https://www.ebucks.com/web/shop/shopHome.do"); err != nil {
		log.Fatal(err)
	}
}
