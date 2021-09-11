package main

import (
	"encoding/json"
	"flag"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"time"

	"github.com/geniass/ebucks-dealz/pkg/scraper"
)

// NOTES
// * things that randomly disappear: instead of deleting everything upfront, fetch new products, then check (set(allProducts) - set(newProducts)) to see if they still exist. Delete if not.
//   this would also help with randomly changing urls which can then be ignored.
// * compare url -> name map between single-thread and multi-thread

var safeFilenameReplaceRegex = regexp.MustCompile(`[^a-zA-Z0-9-]+`)

func main() {

	dirNameArg := flag.String("dir", "./data", "directory in which to write scraped data files")
	cacheDirArg := flag.String("cache", "", "cache directory")
	overwriteArg := flag.Bool("overwrite", false, "when false, a new directory is created within the data dir named as the current date and time; otherwise the data dir is cleaned and replaced.")
	threadsArg := flag.Int("threads", 1, "number of async goroutines to use (1 to disable async)")

	flag.Parse()

	dirname := *dirNameArg

	if !*overwriteArg {
		runDate := time.Now()
		dirname = filepath.Join(dirname, runDate.Format("2006-01-02T15-04-05Z-0700"))
	}

	if *overwriteArg {
		if err := os.RemoveAll(dirname); err != nil {
			log.Fatal(err)
		}
	}

	if err := os.MkdirAll(dirname, os.ModeDir|0755); err != nil {
		log.Fatal(err)
	}

	fileLock := &sync.Mutex{}

	s := scraper.NewScraper(*cacheDirArg, *threadsArg, func(p scraper.Product) {

		// silly I know
		fileLock.Lock()
		defer fileLock.Unlock()

		dir := filepath.Join(dirname, "raw")
		if err := os.MkdirAll(dir, os.ModeDir|0755); err != nil {
			log.Fatal(err)
		}

		if err := writeJSON(p, dir); err != nil {
			log.Fatal(err)
		}
	})
	s.EnableLimits()

	if err := s.Start(); err != nil {
		log.Fatal(err)
	}

	log.Println("Done!")
}

func writeJSON(p scraper.Product, path string) error {
	name := sanitiseFilename(p)
	f, err := os.Create(filepath.Join(path, name+".json"))
	if err != nil {
		return err
	}
	defer f.Close()

	encoder := json.NewEncoder(f)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(p); err != nil {
		return err
	}

	return nil
}

func writeMarkdown(p scraper.Product, path string) error {
	name := sanitiseFilename(p)
	f, err := os.Create(filepath.Join(path, name+".md"))
	if err != nil {
		return err
	}
	defer f.Close()

	if err := markdownTemplate.Execute(f, p); err != nil {
		return err
	}

	return nil
}

func sanitiseFilename(p scraper.Product) string {
	return safeFilenameReplaceRegex.ReplaceAllString(p.Name+"-"+p.CatID+"-"+p.ProdID, "-")
}
