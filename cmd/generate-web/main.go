package main

import (
	"flag"
	"io"
	"log"
	"os"
	"path/filepath"

	dataio "github.com/geniass/ebucks-dealz/pkg/io"
	"github.com/geniass/ebucks-dealz/pkg/web"
)

func main() {
	dataDirNameArg := flag.String("data-dir", "./data", "directory that contains scraped data files")
	ouputDirArg := flag.String("output-dir", "docs", "data to write rendered HTML content to")

	flag.Parse()

	if err := os.MkdirAll(*ouputDirArg, os.ModeDir|0775); err != nil {
		log.Fatal(err)
	}

	// Home page
	err := renderToFile(*ouputDirArg, "index.html", web.RenderHome)
	if err != nil {
		log.Fatal(err)
	}

	// Discounted products
	ps, err := dataio.LoadFromDir(filepath.Join(*dataDirNameArg, "40%/raw"))
	if err != nil {
		log.Fatal(err)
	}
	err = renderToFile(*ouputDirArg, "discount.html", func(w io.Writer) error {
		return web.RenderDealz(w, web.DealzContext{Title: "Discounted (40%)", Products: ps})
	})
	if err != nil {
		log.Fatal(err)
	}

	// Other products
	ps, err = dataio.LoadFromDir(filepath.Join(*dataDirNameArg, "other/raw"))
	if err != nil {
		log.Fatal(err)
	}
	err = renderToFile(*ouputDirArg, "other.html", func(w io.Writer) error {
		return web.RenderDealz(w, web.DealzContext{Title: "Other Products", Products: ps})
	})
	if err != nil {
		log.Fatal(err)
	}

}

func renderToFile(dir string, filename string, renderFunc func(w io.Writer) error) error {
	f, err := os.Create(filepath.Join(dir, filename))
	if err != nil {
		return err
	}
	defer f.Close()

	if err := renderFunc(f); err != nil {
		return err
	}
	return nil
}
