package main

import (
	"log"
	"net/http"

	"github.com/geniass/ebucks-dealz/pkg/io"
	"github.com/geniass/ebucks-dealz/pkg/web"
)

func main() {
	http.HandleFunc("/", func(rw http.ResponseWriter, r *http.Request) {
		if err := web.RenderHome(rw, web.BaseContext{PathPrefix: "/"}); err != nil {
			log.Println(err)
			rw.WriteHeader(http.StatusInternalServerError)
			return
		}
	})

	http.HandleFunc("/discount", func(rw http.ResponseWriter, r *http.Request) {
		ps, err := io.LoadFromDir("data/40%/raw")
		if err != nil {
			log.Println(err)
			rw.WriteHeader(http.StatusInternalServerError)
			return
		}

		if err := web.RenderDealz(rw, web.DealzContext{Title: "Discounted (40%)", Products: ps}); err != nil {
			log.Println(err)
			rw.WriteHeader(http.StatusInternalServerError)
			return
		}
	})

	http.HandleFunc("/other", func(rw http.ResponseWriter, r *http.Request) {
		ps, err := io.LoadFromDir("data/other/raw")
		if err != nil {
			log.Println(err)
			rw.WriteHeader(http.StatusInternalServerError)
			return
		}

		if err := web.RenderDealz(rw, web.DealzContext{Title: "Other Products", Products: ps}); err != nil {
			log.Println(err)
			rw.WriteHeader(http.StatusInternalServerError)
			return
		}
	})

	http.ListenAndServe(":8080", nil)
}
