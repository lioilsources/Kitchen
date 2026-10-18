package escpos

import (
	"bytes"

	"golang.org/x/text/encoding/charmap"
)

// sampleBon builds a typical kitchen bon as a Czech POS sends it: CP852, double-height header,
// item lines with prices, modifier lines, course separator, paper cut.
func sampleBon(table string) []byte {
	enc := func(s string) []byte {
		b, err := charmap.CodePage852.NewEncoder().Bytes([]byte(s))
		if err != nil {
			panic(err)
		}
		return b
	}
	var b bytes.Buffer
	b.Write([]byte{esc, '@'})       // init
	b.Write([]byte{esc, 't', 18})   // CP852
	b.Write([]byte{esc, 'a', 1})    // center
	b.Write([]byte{esc, '!', 0x30}) // double size
	b.Write(enc("KUCHYŇ\n"))
	b.Write(enc("Stůl: " + table + "\n"))
	b.Write([]byte{esc, '!', 0})
	b.Write([]byte{esc, 'a', 0})
	b.Write(enc("Číšník: Jana   18.10.2026 12:31\n"))
	b.Write(enc("--------------------------------\n"))
	b.Write(enc("1x Polévka dne             45,00\n"))
	b.Write(enc("1 Tatarák                 189,00\n"))
	b.Write(enc("--- 2. chod ---\n"))
	b.Write([]byte{esc, 'E', 1})
	b.Write(enc("2x Svíčková na smetaně    398,00\n"))
	b.Write([]byte{esc, 'E', 0})
	b.Write(enc("  - bez brusinek\n"))
	b.Write(enc("1x Smažený řízek          229,00\n"))
	b.Write(enc("  - bramborový salát\n"))
	b.Write(enc("--------------------------------\n"))
	b.Write(enc("Celkem                    861,00\n"))
	b.Write([]byte("\n\n\n"))
	b.Write([]byte{gs, 'V', 66, 0}) // partial cut with feed
	return b.Bytes()
}
