package tts

import (
	"strconv"
	"strings"
	"unicode"
)

var ones = []string{"nula", "jedna", "dva", "tři", "čtyři", "pět", "šest", "sedm", "osm", "devět",
	"deset", "jedenáct", "dvanáct", "třináct", "čtrnáct", "patnáct", "šestnáct", "sedmnáct", "osmnáct", "devatenáct"}
var tens = []string{"", "", "dvacet", "třicet", "čtyřicet", "padesát", "šedesát", "sedmdesát", "osmdesát", "devadesát"}
var hundreds = []string{"", "sto", "dvě stě", "tři sta", "čtyři sta", "pět set", "šest set", "sedm set", "osm set", "devět set"}

// Number spells n (0–999) in Czech. gender m|f|n affects 1 and 2 ("jeden/jedna/jedno", "dva/dvě");
// "" gives the counting form used for table numbers ("stůl jedna", "stůl dva").
func Number(n int, gender string) string {
	if n < 0 || n > 999 {
		return strconv.Itoa(n)
	}
	if n < 20 {
		switch {
		case n == 1 && gender == "m":
			return "jeden"
		case n == 1 && gender == "n":
			return "jedno"
		case n == 2 && (gender == "f" || gender == "n"):
			return "dvě"
		}
		return ones[n]
	}
	var parts []string
	if h := n / 100; h > 0 {
		parts = append(parts, hundreds[h])
		n %= 100
	}
	if n >= 20 {
		parts = append(parts, tens[n/10])
		n %= 10
		if n > 0 {
			parts = append(parts, Number(n, gender))
		}
	} else if n > 0 {
		parts = append(parts, Number(n, gender))
	}
	return strings.Join(parts, " ")
}

// Plural picks the Czech form for a count: 1 → one, 2–4 → few, 0 and 5+ → many.
func Plural(n int, one, few, many string) string {
	switch {
	case n == 1:
		return one
	case n >= 2 && n <= 4:
		return few
	}
	return many
}

// Table speaks a table label: digit runs become words ("12" → "dvanáct", "T3" → "T tři", "bar 2" → "bar dva").
func Table(label string) string {
	var b strings.Builder
	rs := []rune(label)
	for i := 0; i < len(rs); {
		if unicode.IsDigit(rs[i]) {
			j := i
			for j < len(rs) && unicode.IsDigit(rs[j]) {
				j++
			}
			n, _ := strconv.Atoi(string(rs[i:j]))
			if b.Len() > 0 && !strings.HasSuffix(b.String(), " ") {
				b.WriteByte(' ')
			}
			b.WriteString(Number(n, ""))
			i = j
			continue
		}
		b.WriteRune(rs[i])
		i++
	}
	return strings.TrimSpace(b.String())
}

// Minutes speaks a duration in accusative: "jednu minutu", "dvě minuty", "pět minut".
func Minutes(n int) string {
	if n == 1 {
		return "jednu minutu"
	}
	return Number(n, "f") + " " + Plural(n, "minutu", "minuty", "minut")
}

// Capitalize upper-cases the first letter of a sentence.
func Capitalize(s string) string {
	for i, r := range s {
		return string(unicode.ToUpper(r)) + s[i+len(string(r)):]
	}
	return s
}
