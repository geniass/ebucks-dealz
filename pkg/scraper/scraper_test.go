package scraper

import (
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
)

func TestScraperFindsAllProducts(t *testing.T) {
	products := makeProducts("0", 100000) // TODO WAT: with high numbers of products it sometimes "looses" some
	ts := newTestServer(products)
	defer ts.Close()

	m := sync.Mutex{}
	scrapedProducts := []Product{}
	s := newTestScraper(ts.URL+"/web/shop/shopHome.do", 15, func(p Product) {
		m.Lock()
		scrapedProducts = append(scrapedProducts, p)
		m.Unlock()
	})
	if err := s.Start(); err != nil {
		t.Fatal(err)
	}

	if len(products) != len(scrapedProducts) {
		t.Errorf("wrong number of scraped products: got %d expected %d", len(scrapedProducts), len(products))
	}
}

func newTestScraper(startingURL string, threads int, cb ProductPageCallbackFunc) Scraper {
	s := NewScraper("", threads, cb)
	s.colly.AllowedDomains = nil
	s.startingURL = startingURL
	return s
}

func newTestServer(ps []Product) *httptest.Server {

	catIdProdIdMap := make(map[string]map[string]Product)
	for _, p := range ps {
		if catIdProdIdMap[p.CatID] == nil {
			catIdProdIdMap[p.CatID] = make(map[string]Product)
		}
		catIdProdIdMap[p.CatID][p.ProdID] = p
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/web/shop/shopHome.do", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")

		catTags := []string{}
		for c := range catIdProdIdMap {
			catTags = append(catTags, c)
		}

		page := fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
	<body>
		<a href="/web/shop/shopHome.do" class="active header-top-shop">SHOP</a>
		%s
	</body>
</html>
		`,
			categoryATags(catTags),
		)
		w.Write([]byte(page))
	})

	mux.HandleFunc("/web/shop/categorySelected.do", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")

		catId := r.URL.Query().Get("catId")
		switch catId {
		case "":
			w.WriteHeader(http.StatusBadRequest)

		default:
			products := []Product{}
			if catIdProdIdMap[catId] == nil {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			for _, p := range catIdProdIdMap[catId] {
				products = append(products, p)
			}

			page := fmt.Sprintf(`<!DOCTYPE html>
			<html lang="en">
				<body>
					<a href="/web/shop/shopHome.do" class="active header-top-shop">SHOP</a>
					%s
			</body>
			</html>
					`, productATags(products))
			w.Write([]byte(page))
		}
	})

	mux.HandleFunc("/web/shop/productSelected.do", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")

		q := r.URL.Query()
		catId := q.Get("catId")
		prodId := q.Get("prodId")
		switch {
		case catId == "" || prodId == "":
			w.WriteHeader(http.StatusBadRequest)

		default:
			p := catIdProdIdMap[catId][prodId]
			page := fmt.Sprintf(`<!DOCTYPE html>
			<html lang="en">
				<body>
					<form name="productOptionsBean" method="post" action="/web/shop/productOptionSelected.do">
						<div class="product-detail-frame">
							<div class="info-container-frame">
								<h2 id="product-name" class="product-name " data-maincat="842815916"
									data-currentcat="842823972">%s</h2>
								<div class="product-price holiday">
									<p class="was-price">Save: <strong><span class="randValue"></span></strong></p>
									<p>Pay in Rands: <strong><span id="randPrice" class="randValue">R%.2f</span></strong>
									</p>
									<p>Pay in eBucks: <strong><span id="eBPrice" class="eBucksValue">eB%d</span></strong>
									</p>
								</div>
							</div>
						</div> <input type="hidden" name="prodId" value="%s"> <input type="hidden" name="catId"
							   value="%s"> <input type="hidden" name="skuId" value="1211817758"> <input type="hidden"
							   id="prodName" value="%s" /> <input
							   type="hidden" id="catName" value="Huawei " /> <input type="hidden" id="subCatName"
							   value="[{name=Shop Home, uri=/web/shop/shopHome.do}, {name=Wearables, uri=/web/shop/categorySelected.do?catId=842815916}, {name=Huawei , uri=/web/shop/categorySelected.do?catId=842823972}]" />
						<input type="hidden" id="fromRandPrice" value="%[2].2f" /> <input type="hidden" id="fromEBucksPrice"
							   value="%[3]d" />
					</form>
				</body>
			</html>
					`,
				p.Name,
				p.Price,
				int(p.Price*10),
				p.ProdID,
				p.CatID,
				p.Name,
			)
			w.Write([]byte(page))
		}
	})

	mux.HandleFunc("/web/eBucks/errors/globalExceptionPage.jsp", func(rw http.ResponseWriter, r *http.Request) {
		log.Fatal("error page visited unexpectedly")
	})

	s := httptest.NewUnstartedServer(mux)
	s.Start()
	return s
}

func makeProducts(catId string, n uint) []Product {
	ps := []Product{}
	for i := uint(0); i < n; i++ {
		ps = append(ps, makeProduct(catId, int(i)))
	}
	return ps
}

func makeProduct(catId string, i int) Product {
	prodId := strconv.Itoa(i)
	return Product{
		CatID:      catId,
		ProdID:     prodId,
		Name:       "Product " + prodId,
		URL:        productURL(catId, prodId),
		Price:      float64(i * 1000),
		Savings:    100,
		Percentage: 2.0,
	}
}

func productATags(ps []Product) string {
	tags := []string{}
	for _, p := range ps {
		tags = append(tags, productATag(p))
	}
	return strings.Join(tags, "\n")
}

func productATag(p Product) string {
	return fmt.Sprintf(
		`<a href="%s">%s</a>`,
		productURL(p.CatID, p.ProdID),
		p.Name,
	)
}

func categoryATag(c string) string {
	return fmt.Sprintf(
		`<a href="/web/shop/categorySelected.do?catId=%[1]s">Category %[1]s</a>`,
		c,
	)
}

func categoryATags(cs []string) string {
	tags := []string{}
	for _, c := range cs {
		tags = append(tags, categoryATag(c))
	}
	return strings.Join(tags, "\n")
}

func productURL(catId string, prodId string) string {
	return fmt.Sprintf(`/web/shop/productSelected.do?prodId=%s&amp;catId=%s`, prodId, catId)
}
