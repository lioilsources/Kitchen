package web

import (
	"encoding/json"
	"regexp"
	"sort"
	"strings"
	"testing"
)

var placeholder = regexp.MustCompile(`\{(\w+)\}`)

func load(t *testing.T, lang string) map[string]string {
	t.Helper()
	b, err := FS.ReadFile("i18n/" + lang + ".json")
	if err != nil {
		t.Fatalf("missing catalog %s: %v", lang, err)
	}
	m := map[string]string{}
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("%s: %v", lang, err)
	}
	return m
}

func names(s string) string {
	var out []string
	for _, m := range placeholder.FindAllStringSubmatch(s, -1) {
		out = append(out, m[1])
	}
	sort.Strings(out)
	return strings.Join(out, ",")
}

// Every catalog must carry the same keys and placeholders as the Czech source.
func TestCatalogsMatchCzech(t *testing.T) {
	entries, err := FS.ReadDir("i18n")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 45 {
		t.Fatalf("expected 45 catalogs, got %d", len(entries))
	}
	base := load(t, "cs")
	for _, e := range entries {
		lang := strings.TrimSuffix(e.Name(), ".json")
		m := load(t, lang)
		for k, v := range base {
			got, ok := m[k]
			if !ok || strings.TrimSpace(got) == "" {
				t.Errorf("%s: missing %q", lang, k)
				continue
			}
			if names(got) != names(v) {
				t.Errorf("%s: %q placeholders %q, want %q", lang, k, names(got), names(v))
			}
		}
		for k := range m {
			if _, ok := base[k]; !ok {
				t.Errorf("%s: extra key %q", lang, k)
			}
		}
	}
}
