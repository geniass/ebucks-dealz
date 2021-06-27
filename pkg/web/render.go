package web

import (
	"embed"
	"html/template"
	"io"

	"github.com/geniass/ebucks-dealz/pkg/scraper"
)

//go:embed templates
var templatesFs embed.FS

type DealzContext struct {
	Title    string
	Products []scraper.Product
}

func RenderDealz(w io.Writer, c DealzContext) error {
	t, err := template.ParseFS(templatesFs, "templates/dealz.html.tpl")
	if err != nil {
		return err
	}
	t, err = t.ParseFS(templatesFs, "templates/common/*")
	if err != nil {
		return err
	}

	err = t.Execute(w, c)
	if err != nil {
		return err
	}
	return nil
}

func RenderHome(w io.Writer) error {
	t, err := template.ParseFS(templatesFs, "templates/index.html.tpl")
	if err != nil {
		return err
	}
	t, err = t.ParseFS(templatesFs, "templates/common/*")
	if err != nil {
		return err
	}

	err = t.Execute(w, nil)
	if err != nil {
		return err
	}
	return nil
}
