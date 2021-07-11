package main

import "text/template"

var markdownTemplate = template.Must(template.New("markdownTemplate").Parse(
	`
# Ebucks Dealz
## {{ .Name }}
[Product Page]({{ .URL }})

Price: {{printf "R %.2f" .Price }}

Savings: {{printf "R %.2f" .Savings }}

{{ if ne .Percentage 0. -}}
Percentage off: {{ .Percentage }}%
{{- end }}
	`,
))
