package tts

import (
	"strings"

	"github.com/lioilsources/Kitchen/internal/config"
)

// Hlášky — viz docs/voice-lines.md. Krátké věty, číslo stolu vždy první.

// LineItem is one order line as it should be spoken.
type LineItem struct {
	Qty int
	Say config.Say
}

// NewOrder: "Stůl sedm: dvě svíčkové, jedna polévka."
func NewOrder(table string, items []LineItem, hasMods bool) string {
	parts := make([]string, 0, len(items))
	for _, it := range items {
		parts = append(parts, Number(it.Qty, it.Say.Gender)+" "+Plural(it.Qty, it.Say.One, it.Say.Few, it.Say.Many))
	}
	s := "Stůl " + Table(table) + ": " + strings.Join(parts, ", ")
	if hasMods {
		s += ", s úpravou"
	}
	return s + "."
}

// Complete: "Stůl sedm, kompletní."
func Complete(table string) string { return "Stůl " + Table(table) + ", kompletní." }

// WaitClaim: "Stůl sedm, gril, čeká na kuchaře."
func WaitClaim(table, station string) string {
	if station == "" {
		return "Stůl " + Table(table) + " čeká na kuchaře."
	}
	return "Stůl " + Table(table) + ", " + strings.ToLower(station) + ", čeká na kuchaře."
}

// PassAging: "Stůl sedm hotovo, na předávce tři minuty."
func PassAging(table string, minutes int) string {
	return "Stůl " + Table(table) + " hotovo, na předávce " + Minutes(minutes) + "."
}

// Fire: "Stůl sedm, připravit hlavní."
func Fire(table, course string) string {
	if course == "" {
		course = "další chod"
	}
	return "Stůl " + Table(table) + ", připravit " + course + "."
}
