package escpos

import (
	"reflect"
	"testing"
)

func TestParseSampleBon(t *testing.T) {
	p, err := NewParser("utf8", "", "", "", "") // ESC t 18 v datech musí přepnout na CP852
	if err != nil {
		t.Fatal(err)
	}
	tk, ok := p.Parse(sampleBon("7"))
	if !ok {
		t.Fatalf("not parsed; raw:\n%s", tk.Raw)
	}
	if tk.Table != "7" {
		t.Errorf("table = %q", tk.Table)
	}
	want := []Line{
		{Qty: 1, Name: "Polévka dne"},
		{Qty: 1, Name: "Tatarák"},
		{Qty: 2, Name: "Svíčková na smetaně", Mods: []string{"bez brusinek"}, Course: 2},
		{Qty: 1, Name: "Smažený řízek", Mods: []string{"bramborový salát"}, Course: 2},
	}
	if !reflect.DeepEqual(tk.Lines, want) {
		t.Errorf("lines:\n got %+v\nwant %+v\nraw:\n%s", tk.Lines, want, tk.Raw)
	}
}

func TestSplitMultipleBons(t *testing.T) {
	raw := append(sampleBon("3"), sampleBon("12")...)
	parts := Split(raw)
	if len(parts) != 2 {
		t.Fatalf("split into %d parts, want 2", len(parts))
	}
	p, _ := NewParser("cp852", "", "", "", "")
	var tables []string
	for _, b := range parts {
		tk, ok := p.Parse(b)
		if !ok {
			t.Fatalf("part not parsed: %s", tk.Raw)
		}
		tables = append(tables, tk.Table)
	}
	if !reflect.DeepEqual(tables, []string{"3", "12"}) {
		t.Errorf("tables = %v", tables)
	}
}

func TestNotABon(t *testing.T) {
	p, _ := NewParser("utf8", "", "", "", "")
	if _, ok := p.Parse([]byte("Uzávěrka dne\nCelkem 12 345,00\n")); ok {
		t.Error("daily report parsed as kitchen bon")
	}
}

func TestPlainTextBon(t *testing.T) {
	p, _ := NewParser("utf8", "", "", "", "")
	tk, ok := p.Parse([]byte("STUL 5\n3 x Burger\n* medium\n2 Hranolky\n"))
	if !ok || tk.Table != "5" || len(tk.Lines) != 2 || tk.Lines[0].Qty != 3 || tk.Lines[0].Mods[0] != "medium" {
		t.Errorf("got %+v ok=%v", tk, ok)
	}
}
