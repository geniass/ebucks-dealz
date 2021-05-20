package scraper

import (
	"github.com/gocolly/colly"
)

type Scraper struct {
	colly *colly.Collector
}

type Product struct {
	URL        string
	Name       string
	Price      string
	Savings    string
	Percentage string
}
