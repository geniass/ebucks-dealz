package main

import (
	"log"
	"net/http"
	"time"

	"github.com/geniass/ebucks-dealz/pkg/io"
	"github.com/geniass/ebucks-dealz/pkg/web"
)

func main() {

	lastUpdated := time.Now()

	http.HandleFunc("/", func(rw http.ResponseWriter, r *http.Request) {
		if err := web.RenderHome(rw, web.BaseContext{}); err != nil {
			log.Println(err)
			rw.WriteHeader(http.StatusInternalServerError)
			return
		}
	})

	http.HandleFunc("/discount", func(rw http.ResponseWriter, r *http.Request) {
		ps, err := io.LoadFromDir("data/raw")
		if err != nil {
			log.Println(err)
			rw.WriteHeader(http.StatusInternalServerError)
			return
		}

		if err := web.RenderDealz(rw, web.DealzContext{
			Title:       "Discounted (40%)",
			LastUpdated: lastUpdated,
			Products:    ps,
		}); err != nil {
			log.Println(err)
			rw.WriteHeader(http.StatusInternalServerError)
			return
		}
	})

	http.HandleFunc("/other", func(rw http.ResponseWriter, r *http.Request) {
		ps, err := io.LoadFromDir("data/raw")
		if err != nil {
			log.Println(err)
			rw.WriteHeader(http.StatusInternalServerError)
			return
		}

		if err := web.RenderDealz(rw, web.DealzContext{
			Title:       "Other Products",
			LastUpdated: lastUpdated,
			Products:    ps,
		}); err != nil {
			log.Println(err)
			rw.WriteHeader(http.StatusInternalServerError)
			return
		}
	})

	http.ListenAndServe(":8080", nil)
}
