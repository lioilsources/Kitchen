// Package config loads the YAML configuration (stations, menu, thresholds, audio, ingest).
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lioilsources/Kitchen/internal/model"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Listen   string          `yaml:"listen"`
	DataFile string          `yaml:"data_file"`
	AppDir   string          `yaml:"app_dir"` // Flutter web build pro /app/ (prázdné = jen vestavěné web displeje)
	ArtDir   string          `yaml:"art_dir"` // obrázky jídel servírované na /art/ (MenuItem.Image)
	Stations []model.Station `yaml:"stations"`
	Menu     []MenuItem      `yaml:"menu"`
	// Catalog: shared dish pool (YAML with `dishes: [MenuItem]`), path relative to this config file.
	// Menu items are completed from it by SKU; an empty menu takes the whole catalog.
	Catalog  string   `yaml:"catalog"`
	Settings Settings `yaml:"settings"`
	TTS      TTS      `yaml:"tts"`
	EscPOS   EscPOS   `yaml:"escpos"`
	Notify   Notify   `yaml:"notify"`
	// CourseNames: jak se chod jmenuje v hlášce "Stůl sedm, připravit hlavní."
	CourseNames map[int]string `yaml:"course_names"`
}

// MenuItem maps a POS SKU to a station and a static prep-time estimate (until history takes over).
type MenuItem struct {
	SKU     string   `yaml:"sku" json:"sku"`
	Name    string   `yaml:"name" json:"name"`
	Station string   `yaml:"station" json:"station"`
	Prep    Duration `yaml:"prep" json:"prep"`             // static p50
	PrepP90 Duration `yaml:"prep_p90" json:"prep_p90"`     // static p90 (default prep × 1.5)
	Course  int      `yaml:"course" json:"course"`         // default course (1 = předkrm/polévka/hlavní dle podniku)
	Aliases []string `yaml:"aliases" json:"aliases"`       // názvy na bonu, pod kterými položka chodí z POS
	Say     Say      `yaml:"say" json:"say"`               // tvary pro TTS
	Emoji   string   `yaml:"emoji" json:"emoji,omitempty"` // na displejích, když chybí obrázek
	Image   string   `yaml:"image" json:"image,omitempty"` // soubor v art_dir, např. pizza.png
}

// Say holds Czech grammatical forms: "jedna svíčková", "dvě svíčkové", "pět svíčkových".
type Say struct {
	One    string `yaml:"one" json:"one"`
	Few    string `yaml:"few" json:"few"`
	Many   string `yaml:"many" json:"many"`
	Gender string `yaml:"gender" json:"gender"` // m | f | n
}

// Settings are the runtime-adjustable thresholds (admin UI can change them).
type Settings struct {
	WaitToClaim       Duration `yaml:"wait_to_claim" json:"wait_to_claim"`
	CookOverrunFactor float64  `yaml:"cook_overrun_factor" json:"cook_overrun_factor"`
	PassAging         Duration `yaml:"pass_aging" json:"pass_aging"`
	CourseGap         Duration `yaml:"course_gap" json:"course_gap"`
	AutoFire          bool     `yaml:"auto_fire" json:"auto_fire"`
	AnnounceMinGap    Duration `yaml:"announce_min_gap" json:"announce_min_gap"`
	ETARefresh        Duration `yaml:"eta_refresh" json:"eta_refresh"`
	Quiet             bool     `yaml:"quiet" json:"quiet"`
	ReannounceAging   Duration `yaml:"reannounce_aging" json:"reannounce_aging"` // opakovat hlášku stárnutí (0 = jen jednou)
}

type TTS struct {
	// Engine: "piperd" (doporučeno) | "piper" (jednorázové volání binárky, ~1 s navíc) | "say" (macOS, pro vývoj) | "none"
	Engine string `yaml:"engine"`
	// Piper: cesta k binárce a modelu (cs_CZ-jirka-medium.onnx).
	PiperBin   string `yaml:"piper_bin"`
	PiperModel string `yaml:"piper_model"`
	// piperd: perzistentní proces (deploy/piperd.py) — model zůstává v paměti, ~0,1–0,4 s na větu.
	PiperPython  string `yaml:"piper_python"`
	PiperdScript string `yaml:"piperd_script"`
	SayVoice     string `yaml:"say_voice"`
	// Output: "play" (přehraje WAV příkazem PlayCmd) | "snapfifo" (raw PCM do Snapcast FIFO) | "none"
	Output   string   `yaml:"output"`
	PlayCmd  []string `yaml:"play_cmd"` // např. ["aplay","-q"] nebo ["afplay"]; cesta k WAV se připojí
	SnapFIFO string   `yaml:"snap_fifo"`
	ToneFile string   `yaml:"tone_file"` // WAV pro potvrzovací tón (prázdné = bez tónu)
}

type EscPOS struct {
	Enabled  bool   `yaml:"enabled"`
	Listen   string `yaml:"listen"`   // ":9100"
	Forward  string `yaml:"forward"`  // skutečná tiskárna "<printer-ip>:9100" (prázdné = jen sniff)
	Charset  string `yaml:"charset"`  // "cp852" | "cp1250" | "utf8"
	TableRe  string `yaml:"table_re"` // regex se skupinou (?P<table>)
	ItemRe   string `yaml:"item_re"`  // regex se skupinami (?P<qty>) (?P<name>)
	ModRe    string `yaml:"mod_re"`   // řádek modifikátoru, skupina (?P<mod>)
	CourseRe string `yaml:"course_re"`
}

type Notify struct {
	// Webhook dostane POST JSON {"kind","text","table"} — např. ntfy.sh na LAN → push na telefon/hodinky.
	WebhookURL string   `yaml:"webhook_url"`
	Kinds      []string `yaml:"kinds"` // které hlášky posílat (výchozí: pass_aging, complete)
}

func Default() *Config {
	return &Config{
		Listen:   ":8080",
		DataFile: "predavka.db",
		Settings: Settings{
			WaitToClaim:       Duration(2 * time.Minute),
			CookOverrunFactor: 1.2,
			PassAging:         Duration(3 * time.Minute),
			CourseGap:         Duration(10 * time.Minute),
			AutoFire:          true,
			AnnounceMinGap:    Duration(20 * time.Second),
			ETARefresh:        Duration(30 * time.Second),
			ReannounceAging:   Duration(3 * time.Minute),
		},
		TTS:         TTS{Engine: "none", Output: "none"},
		EscPOS:      EscPOS{Listen: ":9100", Charset: "cp852"},
		CourseNames: map[int]string{1: "předkrm", 2: "hlavní", 3: "dezert"},
	}
}

func Load(path string) (*Config, error) {
	c := Default()
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if err := yaml.Unmarshal(b, c); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if c.Catalog != "" {
		cat := c.Catalog
		if !filepath.IsAbs(cat) {
			cat = filepath.Join(filepath.Dir(path), cat)
		}
		dishes, err := LoadCatalog(cat)
		if err != nil {
			return nil, err
		}
		if c.Menu, err = MergeCatalog(c.Menu, dishes); err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
	}
	return c, c.Validate()
}

// LoadCatalog reads a shared dish pool: a YAML file with `dishes: [MenuItem]`.
func LoadCatalog(path string) ([]MenuItem, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var f struct {
		Dishes []MenuItem `yaml:"dishes"`
	}
	if err := yaml.Unmarshal(b, &f); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return f.Dishes, nil
}

// MergeCatalog completes menu items from the catalog by SKU: fields set in the menu win,
// aliases are the union (menu first). An empty menu returns the whole catalog.
func MergeCatalog(menu, dishes []MenuItem) ([]MenuItem, error) {
	if len(menu) == 0 {
		return append([]MenuItem(nil), dishes...), nil
	}
	bySKU := make(map[string]MenuItem, len(dishes))
	for _, d := range dishes {
		bySKU[d.SKU] = d
	}
	out := make([]MenuItem, 0, len(menu))
	for _, m := range menu {
		d, ok := bySKU[m.SKU]
		if !ok {
			if m.Name == "" {
				return nil, fmt.Errorf("menu %q: not in catalog", m.SKU)
			}
			out = append(out, m)
			continue
		}
		if m.Name == "" {
			m.Name = d.Name
		}
		if m.Station == "" {
			m.Station = d.Station
		}
		if m.Prep == 0 {
			m.Prep = d.Prep
		}
		if m.PrepP90 == 0 {
			m.PrepP90 = d.PrepP90
		}
		if m.Course == 0 {
			m.Course = d.Course
		}
		if m.Say == (Say{}) {
			m.Say = d.Say
		}
		if m.Emoji == "" {
			m.Emoji = d.Emoji
		}
		if m.Image == "" {
			m.Image = d.Image
		}
		seen := map[string]bool{}
		var aliases []string
		for _, a := range append(append([]string(nil), m.Aliases...), d.Aliases...) {
			if !seen[a] {
				seen[a] = true
				aliases = append(aliases, a)
			}
		}
		m.Aliases = aliases
		out = append(out, m)
	}
	return out, nil
}

func (c *Config) Validate() error {
	if len(c.Stations) == 0 {
		return fmt.Errorf("config: no stations")
	}
	st := map[string]bool{}
	for i := range c.Stations {
		s := &c.Stations[i]
		if s.ID == "" {
			return fmt.Errorf("config: station without id")
		}
		if s.Parallelism <= 0 {
			s.Parallelism = 1
		}
		if s.Name == "" {
			s.Name = s.ID
		}
		st[s.ID] = true
	}
	seen := map[string]bool{}
	for i := range c.Menu {
		m := &c.Menu[i]
		if m.SKU == "" || m.Name == "" {
			return fmt.Errorf("config: menu item %d needs sku and name", i)
		}
		if seen[m.SKU] {
			return fmt.Errorf("config: duplicate sku %q", m.SKU)
		}
		seen[m.SKU] = true
		if !st[m.Station] {
			return fmt.Errorf("config: menu %q: unknown station %q", m.SKU, m.Station)
		}
		if m.Prep == 0 {
			m.Prep = Duration(8 * time.Minute)
		}
		if m.PrepP90 == 0 {
			m.PrepP90 = Duration(time.Duration(m.Prep) * 3 / 2)
		}
		if m.Course == 0 {
			m.Course = 1
		}
		if m.Say.One == "" {
			m.Say.One = strings.ToLower(m.Name)
		}
		if m.Say.Few == "" {
			m.Say.Few = m.Say.One
		}
		if m.Say.Many == "" {
			m.Say.Many = m.Say.Few
		}
		if m.Say.Gender == "" {
			m.Say.Gender = "f"
		}
	}
	return nil
}

func (c *Config) MenuBySKU() map[string]*MenuItem {
	out := make(map[string]*MenuItem, len(c.Menu))
	for i := range c.Menu {
		out[c.Menu[i].SKU] = &c.Menu[i]
	}
	return out
}

// Duration marshals as Go duration strings ("2m", "90s") in YAML and JSON.
type Duration time.Duration

func (d Duration) D() time.Duration { return time.Duration(d) }

func (d *Duration) UnmarshalYAML(n *yaml.Node) error {
	v, err := time.ParseDuration(n.Value)
	if err != nil {
		return fmt.Errorf("line %d: %w", n.Line, err)
	}
	*d = Duration(v)
	return nil
}

func (d Duration) MarshalJSON() ([]byte, error) {
	return []byte(`"` + time.Duration(d).String() + `"`), nil
}

func (d *Duration) UnmarshalJSON(b []byte) error {
	v, err := time.ParseDuration(strings.Trim(string(b), `"`))
	if err != nil {
		return err
	}
	*d = Duration(v)
	return nil
}
