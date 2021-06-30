package io

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/geniass/ebucks-dealz/pkg/scraper"
)

func LoadFromDir(dir string) ([]scraper.Product, error) {
	var ps []scraper.Product
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()

		dec := json.NewDecoder(f)
		var p scraper.Product
		if err := dec.Decode(&p); err != nil {
			return err
		}
		ps = append(ps, p)
		return nil
	})

	if err != nil {
		return ps, err
	}
	return ps, nil
}
