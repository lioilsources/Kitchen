# PLAN-MOBILE.md — handoff pro Opus: Předávka bez PC

Projekt: **Předávka**. Navazuje na [`PLAN.md`](PLAN.md) (fáze 1 hotová, bez pilotu). Stav: koncept → plán.
Datum: 2026-10-22.

---

## 0. TL;DR

Celý systém má běžet **jen na mobilních zařízeních**: žádný N100, žádný Mac, žádný Snapcast/Pi. Jeden Android
tablet na předávce je **hub** (server + displej předávky + reproduktor), ostatní telefony a tablety jsou klienti
přesně jako dnes (Flutter app nebo jen prohlížeč). Server se **nepřepisuje**: Go kód se zabalí přes `gomobile bind`
do Flutter aplikace. Hlas se přesune ze serveru na klienty: hub posílá hotový český text ve WS zprávě `announce`
(to už existuje), zařízení s volbou „mluvit“ jej přečte platformním TTS (`flutter_tts`, Web Speech API).

Odhad: ~1 týden. Jádro jsou kroky 1–3 v §7, zbytek jsou drobnosti. První ostrý případ: **dětská restaurace**
(`config.kids.yaml`) bez Macu.

---

## 1. Proč to jde (co už je hotové)

| Vlastnost dnešního kódu | Důsledek pro mobil |
|---|---|
| Jedna Go binárka, `modernc.org/sqlite` (čisté Go, `CGO_ENABLED=0` v `make linux`) | `gomobile bind` bez nativních závislostí |
| Web displeje vestavěné přes `web/embed.go` | Klienti nepotřebují aplikaci, stačí prohlížeč |
| Hub posílá `announce` + `tone` po WS (`internal/tts/announcer.go`: `onSpeak` se volá **před** syntézou, i v tichém režimu) | Hlas může dělat klient, server s `tts: {engine: none}` nic neztratí |
| České hlášky (`czech.go`, `lines.go`) jsou čistý text | Skloňování zůstává v Go, klient jen čte |
| CORS `*` v `internal/api/api.go` | Flutter web i stránky z jiného původu fungují |
| ESC/POS proxy = TCP listener na `:9100` | Na Androidu běží beze změny |
| Stav = append-only event log v SQLite | Převzetí hubu jiným zařízením = přehrání logu |

Jediné vázané na PC: `internal/tts/speaker.go` (`os/exec`: piper, `say`, `aplay`/`afplay`, Snapcast FIFO) a
`os.ReadFile` cesty v configu (`config.Load`, `art_dir`, `app_dir`, `tone_file`). Zátěž (desítky objednávek/h) je
pro jakýkoli telefon zanedbatelná.

---

## 2. Rozhodnutí (dané, Opus nerozporuje)

1. **Hub = Android tablet** v zásuvce, kiosk. iOS zabije TCP server do ~30 s po přechodu do pozadí → iPhone je
   vždy jen klient. (iOS hub by vyžadoval appku trvale v popředí přes Guided Access; neřešit teď, nechat jako
   poznámku v docs.)
2. **Server zůstává v Go**, balí se přes `gomobile bind` (AAR). Přepis do Dartu (3,5 k řádků + testy) se nedělá.
3. **Hlas na klientech**, ne na hubu přes Snapcast. Hub sám také mluví (je to displej předávky s reproduktorem přes
   Bluetooth/3,5 mm). Zřetězené reproduktory = další zařízení se zapnutým „mluvit“. Není vzorkově synchronní
   (rozestup stovky ms); pro kuchyni s jedním hlavním reproduktorem přijatelné, dokumentovat.
4. **Žádný cloud, žádný relay**. Vše na jedné Wi‑Fi, hub má pevnou IP (DHCP rezervace) a ukazuje QR kód.
5. **Žádné P2P/CRDT**. Timery, eskalace a ETA potřebují jednu autoritu; failover = ruční převzetí z exportu logu.
6. **Protokol se nemění** (docs/protocol.md), jen se přidá volitelná position: klient se smí rozhodnout, co číst
   nahlas, podle `announce.kind` a vlastního nastavení (stanice kuchyně čte jen hlášky, které se jí týkají – viz §5).

---

## 3. Architektura

```
┌──────────── Wi‑Fi restaurace (nebo domácí) ─────────────────────────────────────────┐
│                                                                                     │
│   Android tablet na předávce = HUB (Flutter app, režim „Hub“)                       │
│   ┌───────────────────────────────────────────────────────────────────────┐         │
│   │ Kotlin ForegroundService (wakelock, notifikace „Předávka běží“)       │         │
│   │   └─ Go (gomobile AAR): config z YAML stringu, SQLite v app dir,      │         │
│   │      fronta/ETA/eskalace, WS hub, HTTP :8088, ESC/POS proxy :9100,    │         │
│   │      vestavěné web displeje, art/ z app dir                           │         │
│   │ Flutter UI: displej pass + banner s IP a QR + flutter_tts (cs‑CZ)     │         │
│   └──────────────────────┬────────────────────────────────────────────────┘         │
│                          │ ws://IP:8088/ws + HTTP                                   │
│       ┌──────────────────┼──────────────────┬──────────────────┐                    │
│       ▼                  ▼                  ▼                  ▼                    │
│  telefon číšníka    tablet kuchaře      TV / telefon plac   pokladna → :9100        │
│  tablet.html        Flutter kitchen     floor.html          (ESC/POS proxy → tisk.) │
│  (prohlížeč)        + „mluvit“ (TTS)    (bez ovládání)                              │
└─────────────────────────────────────────────────────────────────────────────────────┘
```

Server i klienti jsou **stejná Flutter aplikace**; režim Hub je čtvrtý vedle kuchyň/předávka/plac. Hub se při
startu připojí sám na sebe (`http://127.0.0.1:8088`) a chová se jako displej předávky.

### Co se z dnešní architektury vypouští
Snapcast, Pi Zero, Piper, `piperd.py`, systemd — pro mobilní nasazení. Pro N100 nasazení vše zůstává, nic se nemaže.

---

## 4. Změny v Go serveru

### 4.1 Balíček `internal/app` – spuštění serveru jako knihovny
Vytáhnout tělo `run()` z `cmd/server/main.go` do:

```go
package app

type Options struct {
    Config  *config.Config
    DataDir string      // SQLite (data_file relativní k DataDir), art/, app/
    Logger  *slog.Logger
}

type Server struct { /* engine, hub, announcer, http.Server, escpos proxy, cancel */ }

func Start(ctx context.Context, o Options) (*Server, error)  // vrátí po úspěšném Listen
func (s *Server) Addr() string                                // skutečný port (i při listen ":0")
func (s *Server) Stop(ctx context.Context) error
```
`cmd/server/main.go` se zkrátí na: načti config, `signal.NotifyContext`, `app.Start`, čekej, `Stop`.

### 4.2 Config ze stringu a relativní cesty
- `config.Parse([]byte) (*Config, error)` vedle `Load(path)` (Load jen čte soubor a volá Parse).
- `DataFile`, `ArtDir`, `AppDir`, `TTS.ToneFile` relativní cesty se resolvují vůči `Options.DataDir`
  (na PC = cwd, chování se nemění).
- `art_dir` na mobilu: obrázky z `art/kids/` se při buildu zkopírují do Flutter assets a při prvním startu
  hubu vybalí do `<DataDir>/art/`. Jednoduché, bez změn v API.

### 4.3 Balíček `mobile/` pro gomobile
Go modul zůstává jeden; balíček na nejvyšší úrovni `mobile` (gomobile potřebuje jednoduché typy: string, int, error):

```go
package mobile

// Start spustí server. configYAML je obsah configu, dataDir adresář aplikace.
// Vrací adresu "host:port".
func Start(configYAML string, dataDir string) (string, error)
func Stop() error
func IsRunning() bool
func LogTail(n int) string     // pro obrazovku diagnostiky
func ExportDB(dst string) error // kopie SQLite pro share sheet / failover
```
Build: `gomobile bind -target android -androidapi 24 -o apps/display/android/app/libs/predavka.aar ./mobile`.
Cíl do `Makefile`: `make aar`. Výsledek se **necommituje** (gitignore `*.aar`), CI jej staví.

### 4.4 TTS engine na hubu
`tts: {engine: none, output: none}` je už default. Nic dalšího. `logSpeaker.Speak` vrací hned, takže
`announce_min_gap` a prioritní fronta hlášek fungují i bez syntézy (klienti dostanou hlášky v rozestupech,
které server řídí, a nepřekřikují se).

### 4.5 Co se netýká
`internal/tts/speaker.go` nechat; `os/exec` se na Androidu nezavolá, protože engine je `none`. Testy Go beze změn.

---

## 5. Změny ve Flutter aplikaci (`apps/display`)

### 5.1 Režim Hub (`DisplayMode.hub`)
- `settings.dart`: nový režim; při něm `serverUrl = http://127.0.0.1:8088` (needitovatelné) a výběr configu:
  **Dětská restaurace** / **Výchozí** / vlastní YAML (textové pole, uloží se do app dir).
- Nový `screens/hub.dart` = `pass.dart` + horní lišta: IP adresa(y) zařízení, QR kód s `http://IP:8088/`, počet
  připojených klientů (`/api/stats`), stav serveru, tlačítka „Export DB“, „Nová hra“ (smaže DB a restartuje).
- Platform channel `predavka/hub`: `start(configYaml, dataDir)`, `stop()`, `status()`, `logTail()`.
- Android: `HubService : Service` ve `android/.../kotlin/com/ol1n/predavka_display/`:
  foreground notification (`FOREGROUND_SERVICE`, `FOREGROUND_SERVICE_DATA_SYNC` nebo `SPECIAL_USE` dle targetSdk),
  `PARTIAL_WAKE_LOCK`, `WifiManager.WifiLock` (HIGH_PERF), volání Go `Start/Stop`. Při zavření Activity služba
  běží dál; při killu systémem se restartuje (`START_STICKY`) a server se spustí znovu – stav je v SQLite.
- Manifest: `FOREGROUND_SERVICE`, `WAKE_LOCK`, `ACCESS_WIFI_STATE`, `POST_NOTIFICATIONS`,
  `REQUEST_IGNORE_BATTERY_OPTIMIZATIONS` (jednorázová výzva v UI hubu). `usesCleartextTraffic` už je.
- Obrazovka nezhasíná (už řešeno pro displeje), ale hub navíc musí přežít zhasnutí: ověřit, že služba a Go goroutiny
  běží se zhasnutou obrazovkou ≥ 1 h (Doze). Pokud ne → `setExactAndAllowWhileIdle` není cesta, řešit
  přes whitelist baterie (výzva výše) a dokumentovat.

### 5.2 Hlas na klientech (`flutter_tts`)
- Nastavení per zařízení: **Mluvit** (vyp/zap), hlasitost, jazyk pevně `cs-CZ`. Na Androidu zkontrolovat
  dostupnost hlasu (`getLanguages`), jinak nabídnout instalaci Google TTS cs; na iOS je Zuzana.
- Na WS `announce`: pokud **Mluvit** zapnuto a hláška se zařízení týká:
  - režim pass/hub/floor: všechno;
  - režim kitchen: `new_order` jen když obsahuje položky této stanice (snapshot má `station` u položek; `announce`
    nemá → **přidat do `announce.data` pole `stations: []string`** v `tts.Announcement`, server jej už zná v místě
    vzniku hlášky v `internal/queue`). Jediná změna protokolu, zpětně kompatibilní.
- Na WS `tone`: krátký tón z assetu (`audioplayers` nebo `flutter_tts` neumí tón → použít `just_audio`/`audioplayers`).
- `repeat`: tlačítko / klávesa `R` už volá `POST /api/repeat`; server rozešle `announce` s `kind: repeat`, klienti
  přečtou. Beze změny.
- Fronta na klientovi: `flutter_tts` hraje sekvenčně; pokud přijde hláška během mluvení, zařadit (max 3, starší
  `new_order` zahodit po 30 s stejně jako server – `staleAfter`).

### 5.3 Objevení hubu
- Hub: QR kód (`qr_flutter`) + velká IP. Výchozí adresa klienta `http://predavka.local:8080` zůstane pro N100.
- Klient (Flutter): tlačítko „Najít hub“ → mDNS/NSD dotaz `_predavka._tcp` (`nsd` plugin), hub inzeruje přes
  Android `NsdManager` ze služby. Prohlížeče `.local` na Androidu neřeší → pro web klienty je QR/IP jediná cesta.
- Skener QR v klientské appce není nutný (QR otevře systémová kamera v prohlížeči; Flutter klient má pole adresy).

### 5.4 Web klienti (`web/app.js`)
- `P.speak` přes `window.speechSynthesis` s hlasem `cs-CZ`; zapíná se tlačítkem „🔊 Hlas“ v hlavičce
  (uloží se do `localStorage`). iOS Safari vyžaduje gesto před prvním přehráním → tlačítko je zároveň odemknutí.
- `kitchen.html`: filtrovat podle `stations` jako ve Flutteru. `pass.html`/`floor.html`: vše.
- Žádné další změny; stránky už přijímají `announce`.

---

## 6. Provoz, failover, limity (do docs)

| Téma | Řešení |
|---|---|
| Pevná adresa hubu | DHCP rezervace v routeru; hub ukazuje IP; při změně IP klienti přepíší adresu (QR) |
| Hub spadne / vybije se | Stav v SQLite, služba `START_STICKY`. Ruční převzetí: Export DB (share → druhý tablet → Import) a start hubu tam; klienti změní adresu. Automatický failover **mimo rozsah** |
| Hub v pozadí | Foreground service; UI může být zavřené. Doporučení: kiosk (Android „Připnout obrazovku“) |
| Dvě hubs na jedné síti | mDNS ukáže obě; dokumentovat „jeden hub na síť“, server nic nehlídá |
| Pokladna → ESC/POS | Pokladna tiskne na IP tabletu `:9100`, hub forwarduje na tiskárnu. Beze změny kódu |
| Hlas není synchronní | Doporučit jeden hlavní reproduktor (hub) + volitelně stanice; ostatní zařízení s „Mluvit“ vypnuto |
| iOS jako hub | Ne. iPhone/iPad jsou klienti (Flutter nebo Safari s Web Speech) |
| Čas | Všechna zařízení synchronizují přes NTP, ETA počítá server z vlastních hodin; klienti ukazují `eta_min`, ne absolutní časy – OK |
| Záloha dat | Export DB tlačítkem; `kids.db` = „Nová hra“ |

---

## 7. Kroky pro Opuse (pořadí, odhady)

### Spike M1 (½ dne): gomobile proof
1. `internal/app` (§4.1), `config.Parse` + `DataDir` (§4.2), `mobile/` (§4.3), `make aar`.
2. Ověřit, že `go test ./...` a `make linux` fungují beze změny; `cmd/server` chová se stejně (`make run`, `make sim`).
3. Minimální Flutter Hub: `MethodChannel` → `Start(kidsYaml, filesDir)`; z jiného telefonu otevřít
   `http://IP:8088/tablet.html` a odeslat objednávku. **Měřit:** start serveru na tabletu < 2 s, WS snapshot do 100 ms.
4. Zpráva do `docs/spikes/04-mobile-hub.md`: co fungovalo, velikost AAR/APK, zařízení, verze Androidu.

### Krok M2 (1–2 dny): Hub jako produkt
5. `HubService` (foreground, wakelock, wifi lock, sticky), výzva na baterii, `screens/hub.dart` s QR + IP + stav
   + Export DB + Nová hra. Výběr configu (kids/výchozí/vlastní). Art assets → DataDir.
6. Test přežití: hub 1 h se zhasnutým displejem, klient každých 5 min pošle objednávku, zkontrolovat log.

### Krok M3 (1 den): Hlas na klientech
7. `flutter_tts` + nastavení „Mluvit“, filtr podle `stations` (přidat do `Announcement` a WS; aktualizovat
   `docs/protocol.md`), tón, klientská fronta. Web: `speechSynthesis` + tlačítko Hlas.
8. Test: hub + tablet kuchyně + telefon plac, 10 objednávek ze simulátoru (`tools/sim` proti IP hubu), poslouchat,
   zda se hlášky nepřekrývají a kuchyň slyší jen své.

### Krok M4 (½ dne): objevení a dokumentace
9. mDNS (hub inzeruje, Flutter klient „Najít hub“).
10. README: sekce **„Bez PC“** (dětská restaurace: tablet = hub, ostatní prohlížeč), tabulka rolí/adres jako dnes.
    `apps/display/README.md`: režim Hub, Mluvit. `docs/PLAN.md` §4 BOM: varianta „jen mobily“ (0 Kč HW navíc,
    tablet 3–5 tis.). Aktualizovat tabulku stavu v README.
11. CI (`.github`): job `flutter build apk` s krokem `gomobile bind` (Go + Android SDK/NDK v runneru); AAR jako artefakt.

### Definition of done
- Dětská restaurace proběhne **bez Macu**: tablet hub + 2 telefony (číšník, plac) + 1 tablet kuchyň, hlas z hubu
  a z kuchyně, ESC/POS nepotřeba.
- `make test` zelený; `make run` na Macu a `make linux` nezměněné chování.
- Hub přežije 1 h se zhasnutou obrazovkou.
- Dokumentace podle bodu 10.

---

## 8. Rizika a otevřené otázky

- **gomobile a Go 1.26**: ověřit kompatibilitu `golang.org/x/mobile` s aktuálním Go a NDK; pokud by `bind` zlobil,
  plán B je `c-shared` (`go build -buildmode=c-shared`) + Dart FFI (`ffigen`). Rozhodnout ve spiku M1, ne dřív.
- **Doze a Wi‑Fi při zhasnuté obrazovce**: některé výrobce (Xiaomi, Huawei) zabíjejí služby i s whitelistem.
  Dokumentovat doporučené tablety (Samsung, Lenovo, Pixel) a kiosk režim.
- **Kvalita Android TTS cs**: Google TTS cs je použitelná, ne krásná. Piper na Androidu (sherpa‑onnx) je možný
  upgrade později; ne teď.
- **Velikost APK**: AAR s Go runtime ~10–15 MB; přijatelné.
- **Bezpečnost**: LAN bez autentizace zůstává (MVP). Hub na veřejné Wi‑Fi = kdokoli pošle objednávku; dokumentovat
  „vlastní Wi‑Fi / hotspot“. Hotspot z hubu (tablet jako AP) je zajímavá varianta pro dětskou hru bez routeru –
  Android umí „Local only hotspot“ jen programově s omezeními; nechat jako otázku, nezkoušet v M1–M4.
- **Dva hlasy naráz** (hub + kuchyň blízko sebe): v praxi vyzkoušet, případně doporučit jen hub.
