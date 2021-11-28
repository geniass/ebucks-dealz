package web

import (
	"embed"
	"html/template"
	"io"
	"time"

	"github.com/geniass/ebucks-dealz/pkg/scraper"
)

//go:embed templates
var templatesFs embed.FS

type BaseContext struct {
	PathPrefix string
}

type DealzContext struct {
	BaseContext
	Title       string
	LastUpdated time.Time
	Products    []scraper.Product
}

func (c DealzContext) FormattedLastUpdated() string {
	loc, err := time.LoadLocation("Africa/Johannesburg")
	if err != nil {
		loc = time.UTC
	}
	return c.LastUpdated.In(loc).Format("2006-01-02T15:04:05 MST")
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

func RenderHome(w io.Writer, c BaseContext) error {
	t, err := template.ParseFS(templatesFs, "templates/index.html.tpl")
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
