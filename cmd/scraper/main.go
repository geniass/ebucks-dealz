package main

import (
	"encoding/json"
	"flag"
	"fmt"
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

	s := scraper.NewScraper(*cacheDirArg, *threadsArg, func(p scraper.Product) {
		if p.Percentage == 0 {
			dir := filepath.Join(dirname, "other")
			if err := os.MkdirAll(dir, os.ModeDir|0755); err != nil {
				log.Fatal(err)
			}
			if err := writeMarkdown(p, dir); err != nil {
				log.Fatal(err)
			}

			jsonDir := filepath.Join(dir, "raw")
			if err := os.MkdirAll(jsonDir, os.ModeDir|0755); err != nil {
				log.Fatal(err)
			}
			if err := writeJSON(p, jsonDir); err != nil {
				log.Fatal(err)
			}
		} else {
			dir := filepath.Join(dirname, fmt.Sprintf("%.0f%%", p.Percentage))
			if err := os.MkdirAll(dir, os.ModeDir|0755); err != nil {
				log.Fatal(err)
			}
			if err := writeMarkdown(p, dir); err != nil {
				log.Fatal(err)
			}

			jsonDir := filepath.Join(dir, "raw")
			if err := os.MkdirAll(jsonDir, os.ModeDir|0755); err != nil {
				log.Fatal(err)
			}
			if err := writeJSON(p, jsonDir); err != nil {
				log.Fatal(err)
			}
		}
	})

	if err := s.Start(); err != nil {
		log.Fatal(err)
	}

	log.Println("Done!")
}

func writeJSON(p scraper.Product, path string) error {
	name := sanitiseFilename(p.Name)
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
	name := sanitiseFilename(p.Name)
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

func sanitiseFilename(name string) string {
	return safeFilenameReplaceRegex.ReplaceAllString(name, "-")
}
