// escpos-send prints a sample kitchen bon (CP852 ESC/POS) to the proxy — or to a real printer.
//
//	go run ./tools/escpos-send -addr localhost:9100 -table 7
//	go run ./tools/escpos-send -dump > bon.bin   # surová data (např. pro `nc localhost 9100 < bon.bin`)
package main

import (
	"bytes"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"time"

	"golang.org/x/text/encoding/charmap"
)

func main() {
	addr := flag.String("addr", "localhost:9100", "printer/proxy address")
	table := flag.String("table", "7", "table number")
	lines := flag.String("items", "1x Polévka dne|2x Svíčková na smetaně|- bez brusinek|1x Smažený řízek", "lines separated by |")
	dump := flag.Bool("dump", false, "write raw bytes to stdout instead of sending")
	flag.Parse()

	b := bon(*table, strings.Split(*lines, "|"))
	if *dump {
		os.Stdout.Write(b)
		return
	}
	c, err := net.DialTimeout("tcp", *addr, 3*time.Second)
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()
	if _, err := c.Write(b); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("odesláno %d B na %s (stůl %s)\n", len(b), *addr, *table)
}

func bon(table string, lines []string) []byte {
	enc := func(s string) []byte {
		out, err := charmap.CodePage852.NewEncoder().Bytes([]byte(s))
		if err != nil {
			log.Fatalf("cp852: %v", err)
		}
		return out
	}
	var b bytes.Buffer
	b.Write([]byte{0x1B, '@', 0x1B, 't', 18, 0x1B, 'a', 1, 0x1B, '!', 0x30})
	b.Write(enc("KUCHYŇ\nStůl: " + table + "\n"))
	b.Write([]byte{0x1B, '!', 0, 0x1B, 'a', 0})
	b.Write(enc("Číšník: Test   " + time.Now().Format("02.01.2006 15:04") + "\n"))
	b.Write(enc(strings.Repeat("-", 32) + "\n"))
	for _, l := range lines {
		if strings.HasPrefix(strings.TrimSpace(l), "-") {
			b.Write(enc("  " + strings.TrimSpace(l) + "\n"))
		} else {
			b.Write(enc(l + "\n"))
		}
	}
	b.Write([]byte("\n\n\n\x1DVB\x00"))
	return b.Bytes()
}
