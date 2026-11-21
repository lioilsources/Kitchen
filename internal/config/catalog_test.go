package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadWithCatalog(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("dishes.yaml", `dishes:
  - {sku: gulas, name: Guláš, station: teple, prep: 5m, course: 2, aliases: [guláš], say: {one: guláš, few: guláše, many: gulášů, gender: m}}
  - {sku: pivo, name: Pivo, station: teple, prep: 1m}
`)
	write("r.yaml", `catalog: dishes.yaml
stations: [{id: teple, name: Teplá}, {id: gril, name: Gril}]
menu:
  - {sku: gulas, station: gril, aliases: ["Hovězí guláš z karabáčku"]}
  - {sku: vlastni, name: Vlastní jídlo, station: teple}
`)
	c, err := Load(filepath.Join(dir, "r.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Menu) != 2 {
		t.Fatalf("menu len %d, want 2", len(c.Menu))
	}
	g := c.Menu[0]
	if g.Name != "Guláš" || g.Station != "gril" || g.Prep.D() != 5*time.Minute || g.Course != 2 || g.Say.Few != "guláše" {
		t.Fatalf("merged item wrong: %+v", g)
	}
	if len(g.Aliases) != 2 || g.Aliases[0] != "Hovězí guláš z karabáčku" {
		t.Fatalf("aliases %v", g.Aliases)
	}

	write("all.yaml", "catalog: dishes.yaml\nstations: [{id: teple}]\n")
	c, err = Load(filepath.Join(dir, "all.yaml"))
	if err != nil || len(c.Menu) != 2 {
		t.Fatalf("whole catalog: %v %d", err, len(c.Menu))
	}

	write("bad.yaml", "catalog: dishes.yaml\nstations: [{id: teple}]\nmenu: [{sku: nic}]\n")
	if _, err := Load(filepath.Join(dir, "bad.yaml")); err == nil {
		t.Fatal("unknown sku without name should fail")
	}
}
