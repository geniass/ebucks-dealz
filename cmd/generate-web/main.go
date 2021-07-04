package main

import (
	"errors"
	"flag"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"

	dataio "github.com/geniass/ebucks-dealz/pkg/io"
	"github.com/geniass/ebucks-dealz/pkg/web"
)

func main() {
	dataDirNameArg := flag.String("data-dir", "./data", "directory that contains scraped data files")
	ouputDirArg := flag.String("output-dir", "docs", "data to write rendered HTML content to")
	pagePathPrefixArg := flag.String("path-prefix", "", "prefix page link URLs (in case pages are hosted at a subpath); should start with '/'")

	flag.Parse()

	if err := os.MkdirAll(*ouputDirArg, os.ModeDir|0775); err != nil {
		log.Fatal(err)
	}

	// Home page
	err := renderToFile(*ouputDirArg, "index.html", func(w io.Writer) error {
		return web.RenderHome(w, web.BaseContext{PathPrefix: *pagePathPrefixArg})
	})
	if err != nil {
		log.Fatal(err)
	}

	{
		// Discounted products
		dataDir := filepath.Join(*dataDirNameArg, "40%/raw")
		ps, err := dataio.LoadFromDir(dataDir)
		if errors.Is(err, fs.ErrNotExist) {
			log.Printf("WARNING: data dir %q does not exist, assuming no deals...\n", dataDir)
		} else if err != nil {
			log.Fatal(err)
		}
		err = renderToFile(*ouputDirArg, "discount.html", func(w io.Writer) error {
			c := web.DealzContext{
				BaseContext: web.BaseContext{PathPrefix: *pagePathPrefixArg},
				Title:       "Discounted (40%)",
				Products:    ps,
			}
			return web.RenderDealz(w, c)
		})
		if err != nil {
			log.Fatal(err)
		}
	}

	{
		// Other products
		dataDir := filepath.Join(*dataDirNameArg, "other/raw")
		ps, err := dataio.LoadFromDir(dataDir)
		if errors.Is(err, fs.ErrNotExist) {
			log.Printf("WARNING: data dir %q does not exist, assuming no deals...\n", dataDir)
		} else if err != nil {
			log.Fatal(err)
		}
		err = renderToFile(*ouputDirArg, "other.html", func(w io.Writer) error {
			c := web.DealzContext{
				BaseContext: web.BaseContext{PathPrefix: *pagePathPrefixArg},
				Title:       "Other Products",
				Products:    ps,
			}
			return web.RenderDealz(w, c)
		})
		if err != nil {
			log.Fatal(err)
		}
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
