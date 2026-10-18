// Package escpos sits between any POS and its kitchen printer (TCP 9100), forwards the bytes unchanged
// and parses each bon (table, items, mods) into an order. Works with any POS without an API.
package escpos

import (
	"bytes"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/charmap"
)

const (
	esc = 0x1B
	gs  = 0x1D
	fs  = 0x1C
	dle = 0x10
)

// Ticket is one parsed bon.
type Ticket struct {
	Table string
	Lines []Line
	Raw   string // text after stripping control codes (pro ladění parseru)
}

type Line struct {
	Qty    int
	Name   string
	Mods   []string
	Course int
}

type Parser struct {
	charset  string
	tableRe  *regexp.Regexp
	itemRe   *regexp.Regexp
	modRe    *regexp.Regexp
	courseRe *regexp.Regexp
	priceRe  *regexp.Regexp
	ignoreRe *regexp.Regexp
}

const (
	DefaultTableRe  = `(?i)\b(?:st[ůu]l|table|stul)\s*(?:č\.|c\.|#|:)?\s*(?P<table>[0-9A-Za-z/-]+)`
	DefaultItemRe   = `^\s*(?P<qty>\d{1,3})\s*(?:x|×|ks|\*)?\s+(?P<name>\S.*?)\s*$`
	DefaultModRe    = `^\s*(?:[-+*>]|\.\.)\s*(?P<mod>\S.*?)\s*$`
	DefaultCourseRe = `(?i)^\W*(?:(?P<course>\d)\.?\s*chod|chod\s*(?P<course2>\d)|course\s*(?P<course3>\d))\W*$`
)

func NewParser(charset, tableRe, itemRe, modRe, courseRe string) (*Parser, error) {
	or := func(s, d string) string {
		if s == "" {
			return d
		}
		return s
	}
	p := &Parser{charset: or(charset, "cp852")}
	var err error
	for _, c := range []struct {
		dst **regexp.Regexp
		src string
	}{
		{&p.tableRe, or(tableRe, DefaultTableRe)},
		{&p.itemRe, or(itemRe, DefaultItemRe)},
		{&p.modRe, or(modRe, DefaultModRe)},
		{&p.courseRe, or(courseRe, DefaultCourseRe)},
	} {
		if *c.dst, err = regexp.Compile(c.src); err != nil {
			return nil, fmt.Errorf("escpos regex %q: %w", c.src, err)
		}
	}
	p.priceRe = regexp.MustCompile(`\s+\d+(?:[ .]\d{3})*[,.]\d{2}\s*(?:Kč|Kc|CZK|,-)?\s*$`)
	p.ignoreRe = regexp.MustCompile(`(?i)^\s*(?:celkem|suma|účet|ucet|číšník|cisnik|obsluha|datum|čas|cas|bon|objednávka|objednavka|tel|ič|dič|děkujeme|dekujeme)\b|^[\s=\-_*#.]+$`)
	return p, nil
}

// Split cuts a raw printer stream into bons at paper-cut commands (GS V).
func Split(raw []byte) [][]byte {
	var out [][]byte
	start := 0
	for i := 0; i+1 < len(raw); i++ {
		if raw[i] == gs && raw[i+1] == 'V' {
			end := i + 3
			if i+2 < len(raw) && (raw[i+2] == 65 || raw[i+2] == 66) {
				end++ // GS V m n
			}
			if end > len(raw) {
				end = len(raw)
			}
			out = append(out, raw[start:end])
			start = end
			i = end - 1
		}
	}
	if start < len(raw) && len(bytes.TrimSpace(raw[start:])) > 0 {
		out = append(out, raw[start:])
	}
	return out
}

// Text strips ESC/POS control sequences and decodes the code page.
// ESC t n (výběr kódové stránky) přepíše nastavený charset: 18 → CP852, 45/47 → CP1250, 16 → CP1252.
func (p *Parser) Text(raw []byte) string {
	var b []byte
	cs := p.charset
	for i := 0; i < len(raw); i++ {
		c := raw[i]
		switch c {
		case esc:
			if i+1 >= len(raw) {
				i = len(raw)
				continue
			}
			cmd := raw[i+1]
			switch cmd {
			case '@', '2', '<': // init, default spacing
				i++
			case 'p': // cash drawer: m t1 t2
				i += 4
			case 't':
				if i+2 < len(raw) {
					switch raw[i+2] {
					case 18:
						cs = "cp852"
					case 45, 47:
						cs = "cp1250"
					case 16:
						cs = "cp1252"
					}
				}
				i += 2
			case '*': // bit image: m nL nH data
				if i+4 < len(raw) {
					n := int(raw[i+3]) | int(raw[i+4])<<8
					w := n
					if raw[i+2] >= 32 {
						w = n * 3
					}
					i += 4 + w
				} else {
					i = len(raw)
				}
			default: // většina ESC příkazů má 1 parametr (ESC ! E - a d J M G R 3 a …)
				i += 2
			}
		case gs:
			if i+1 >= len(raw) {
				i = len(raw)
				continue
			}
			cmd := raw[i+1]
			switch cmd {
			case 'V':
				if i+2 < len(raw) && (raw[i+2] == 65 || raw[i+2] == 66) {
					i += 3
				} else {
					i += 2
				}
			case 'k': // barcode
				if i+2 < len(raw) && raw[i+2] <= 6 {
					j := bytes.IndexByte(raw[i+3:], 0)
					if j < 0 {
						i = len(raw)
					} else {
						i += 3 + j
					}
				} else if i+3 < len(raw) {
					i += 3 + int(raw[i+3])
				} else {
					i = len(raw)
				}
			case 'v': // raster: GS v 0 m xL xH yL yH data
				if i+7 < len(raw) {
					x := int(raw[i+4]) | int(raw[i+5])<<8
					y := int(raw[i+6]) | int(raw[i+7])<<8
					i += 7 + x*y
				} else {
					i = len(raw)
				}
			case '(': // GS ( fn pL pH data
				if i+4 < len(raw) {
					i += 4 + (int(raw[i+3]) | int(raw[i+4])<<8)
				} else {
					i = len(raw)
				}
			default: // GS ! B h w H f L … — 1 parametr
				i += 2
			}
		case fs:
			if i+1 < len(raw) && (raw[i+1] == '&' || raw[i+1] == '.') {
				i++
			} else {
				i += 2
			}
		case dle: // DLE EOT n / DLE ENQ n — dotaz na stav
			i += 2
		case '\r':
		case '\n', '\t':
			b = append(b, c)
		default:
			if c >= 0x20 {
				b = append(b, c)
			}
		}
	}
	return decode(b, cs)
}

func decode(b []byte, cs string) string {
	var enc encoding.Encoding
	switch strings.ToLower(cs) {
	case "cp852", "pc852", "ibm852":
		enc = charmap.CodePage852
	case "cp1250", "windows-1250", "wpc1250":
		enc = charmap.Windows1250
	case "cp1252", "windows-1252":
		enc = charmap.Windows1252
	default:
		return string(b) // utf8
	}
	s, err := enc.NewDecoder().Bytes(b)
	if err != nil {
		return string(b)
	}
	return string(s)
}

// Parse turns one bon into a ticket. Returns ok=false when it does not look like a kitchen bon.
func (p *Parser) Parse(raw []byte) (Ticket, bool) {
	text := p.Text(raw)
	t := Ticket{Raw: text}
	course := 0
	for _, ln := range strings.Split(text, "\n") {
		if strings.TrimSpace(ln) == "" {
			continue
		}
		if t.Table == "" {
			if m := p.tableRe.FindStringSubmatch(ln); m != nil {
				t.Table = group(p.tableRe, m, "table")
				continue
			}
		}
		if m := p.courseRe.FindStringSubmatch(ln); m != nil {
			for _, g := range []string{"course", "course2", "course3"} {
				if v := group(p.courseRe, m, g); v != "" {
					course, _ = strconv.Atoi(v)
				}
			}
			continue
		}
		if p.ignoreRe.MatchString(ln) {
			continue
		}
		if m := p.itemRe.FindStringSubmatch(ln); m != nil {
			qty, _ := strconv.Atoi(group(p.itemRe, m, "qty"))
			name := strings.TrimSpace(p.priceRe.ReplaceAllString(group(p.itemRe, m, "name"), ""))
			if qty > 0 && name != "" {
				t.Lines = append(t.Lines, Line{Qty: qty, Name: name, Course: course})
			}
			continue
		}
		if m := p.modRe.FindStringSubmatch(ln); m != nil && len(t.Lines) > 0 {
			last := &t.Lines[len(t.Lines)-1]
			last.Mods = append(last.Mods, group(p.modRe, m, "mod"))
		}
	}
	return t, t.Table != "" && len(t.Lines) > 0
}

func group(re *regexp.Regexp, m []string, name string) string {
	if i := re.SubexpIndex(name); i >= 0 && i < len(m) {
		return m[i]
	}
	return ""
}
