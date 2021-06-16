package main

import (
	"encoding/json"
	"flag"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/geniass/ebucks-dealz/pkg/scraper"
)

var safeFilenameReplaceRegex = regexp.MustCompile(`[^a-zA-Z0-9-]+`)

func main() {

	dirNameArg := flag.String("dir", "./data", "directory in which to write scraped data files")
	cacheDirArg := flag.String("cache", "", "cache directory")
	overwriteArg := flag.Bool("overwrite", false, "when false, a new directory is created within the data dir named as the current date and time; otherwise the data dir is cleaned and replaced.")
	asyncArg := flag.Bool("async", false, "enable async scraping")

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

	s := scraper.NewScraper(*cacheDirArg, *asyncArg, func(p scraper.Product) {
		if err := writeJSON(p, dirname); err != nil {
			log.Fatal(err)
		}
		if err := writeMarkdown(p, dirname); err != nil {
			log.Fatal(err)
		}

		if p.Percentage != "" {
			func() {
				percentDirname := filepath.Join(dirname, p.Percentage)
				if err := os.MkdirAll(percentDirname, os.ModeDir|0755); err != nil {
					log.Fatal(err)
				}

				if err := writeJSON(p, percentDirname); err != nil {
					log.Fatal(err)
				}
				if err := writeMarkdown(p, percentDirname); err != nil {
					log.Fatal(err)
				}
			}()
		}
	})

	if err := s.Start(); err != nil {
		log.Fatal(err)
	}
}

func writeJSON(p scraper.Product, path string) error {
	name := safeFilenameReplaceRegex.ReplaceAllString(p.Name, "-")
	f, err := os.Create(filepath.Join(path, name+".json"))
	if err != nil {
		return err
	}
	defer f.Close()

	encoder := json.NewEncoder(f)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(p); err != nil {
		return err
	}

	return nil
}

func writeMarkdown(p scraper.Product, path string) error {
	name := safeFilenameReplaceRegex.ReplaceAllString(p.Name, "-")
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
