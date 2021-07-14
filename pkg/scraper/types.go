package scraper

import (
	"sync"

	"github.com/gocolly/colly/v2"
)

type Scraper struct {
	colly *colly.Collector

	mutex       *sync.Mutex
	urlBackoffs map[string]int
}

type Product struct {
	URL        string
	Name       string
	Price      string
	Savings    string
	Percentage string
}
