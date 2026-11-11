// Package web embeds the browser displays (kitchen, pass, floor, tablet, admin)
// and their UI translations (i18n/<language>.json, 45 languages).
package web

import "embed"

//go:embed *.html *.css *.js i18n/*.json
var FS embed.FS
