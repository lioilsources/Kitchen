// Package web embeds the browser displays (kitchen, pass, floor, tablet, admin).
package web

import "embed"

//go:embed *.html *.css *.js
var FS embed.FS
