package tts

import (
	"testing"

	"github.com/lioilsources/Kitchen/internal/config"
)

func TestNumber(t *testing.T) {
	cases := []struct {
		n      int
		gender string
		want   string
	}{
		{1, "m", "jeden"}, {1, "f", "jedna"}, {1, "n", "jedno"}, {1, "", "jedna"},
		{2, "m", "dva"}, {2, "f", "dvě"}, {2, "", "dva"}, {7, "", "sedm"}, {12, "", "dvanáct"},
		{21, "", "dvacet jedna"}, {22, "f", "dvacet dvě"}, {40, "", "čtyřicet"},
		{100, "", "sto"}, {205, "", "dvě stě pět"}, {999, "", "devět set devadesát devět"},
	}
	for _, c := range cases {
		if got := Number(c.n, c.gender); got != c.want {
			t.Errorf("Number(%d,%q) = %q, want %q", c.n, c.gender, got, c.want)
		}
	}
}

func TestTable(t *testing.T) {
	for in, want := range map[string]string{"7": "sedm", "12": "dvanáct", "T3": "T tři", "bar 2": "bar dva", "1": "jedna"} {
		if got := Table(in); got != want {
			t.Errorf("Table(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestMinutes(t *testing.T) {
	for n, want := range map[int]string{1: "jednu minutu", 2: "dvě minuty", 3: "tři minuty", 5: "pět minut", 22: "dvacet dvě minut"} {
		if got := Minutes(n); got != want {
			t.Errorf("Minutes(%d) = %q, want %q", n, got, want)
		}
	}
}

func TestLines(t *testing.T) {
	svickova := config.Say{One: "svíčková", Few: "svíčkové", Many: "svíčkových", Gender: "f"}
	polevka := config.Say{One: "polévka", Few: "polévky", Many: "polévek", Gender: "f"}
	rizek := config.Say{One: "řízek", Few: "řízky", Many: "řízků", Gender: "m"}
	got := NewOrder("7", []LineItem{{2, svickova}, {1, polevka}}, false)
	if want := "Stůl sedm: dvě svíčkové, jedna polévka."; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	got = NewOrder("12", []LineItem{{5, rizek}, {1, rizek}}, true)
	if want := "Stůl dvanáct: pět řízků, jeden řízek, s úpravou."; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if got, want := PassAging("7", 3), "Stůl sedm hotovo, na předávce tři minuty."; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if got, want := WaitClaim("7", "Gril"), "Stůl sedm, gril, čeká na kuchaře."; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if got, want := Fire("7", "hlavní"), "Stůl sedm, připravit hlavní."; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
