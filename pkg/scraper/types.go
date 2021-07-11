package scraper

import (
	"sync"

	"github.com/gocolly/colly/v2"
	"github.com/gocolly/colly/v2/queue"
)

type Scraper struct {
	colly *colly.Collector
	q     *queue.Queue

	mutex       *sync.Mutex
	urlBackoffs map[string]int
}

type Product struct {
	URL        string
	Name       string
	Price      float64
	Savings    float64
	Percentage float64
}

type ebucksProductDetail struct {
	ProductDetail struct {
		ID       int              `json:"id"`
		Discount []ebucksDiscount `json:"discount"`
	} `json:"productDetail"`
}

type ebucksDiscount struct {
	Level         int `json:"discountLevel"`
	Percent       int `json:"discountPercent"`
	EbucksPrice   int `json:"discountPrice"`  // The price is given in ebucks, not rands
	EbucksSavings int `json:"discountSaving"` // The price is given in ebucks, not rands
}

func (d ebucksDiscount) RandPrice() float64 {
	return ebucksToRands(d.EbucksPrice)
}

func (d ebucksDiscount) RandSavings() float64 {
	return ebucksToRands(d.EbucksSavings)
}

func ebucksToRands(ebucks int) float64 {
	return float64(ebucks) / 10
}
