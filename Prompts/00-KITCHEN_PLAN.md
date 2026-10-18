# PREDAVKA_PLAN.md — handoff pro Opus

Projekt: **Předávka** (pracovní název) — klidná koordinace kuchyně a placu bez koukání do obrazovek.
Autor zadání: Ol1n. Stav: koncept → plán. Datum: 2026-10-18.

---

## 0. TL;DR

Restaurace bez stresu: kuchaři **slyší** nové objednávky z reproduktoru na předávce (volitelně zřetězené reproduktory
v kuchyni), vyzvedávají si je z fronty jedním stiskem (bump bar / NFC / pedál), systém ví, jak dlouho jídlo trvá.
Obsluha na place má **jeden klidný displej**: za kolik minut které jídlo dorazí a na jaký stůl. Hotové jídlo, které
leží na předávce, **stárne** a po pár minutách si o sebe řekne (hlas + displej + volitelně vibrace na hodinkách).

Není to nemožné. Hlas **ven** (TTS) je snadný a spolehlivý. Hlas **dovnitř** (ASR v hlučné kuchyni) je nejtěžší část —
v MVP jej **nepotřebujeme**: objednávky přicházejí z pokladny (POS) nebo z tabletu obsluhy, hlasové zadávání je fáze 3
a je nadstavba, ne nosný pilíř. Vše běží **lokálně na LAN** (mini PC / Spark ne, stačí N100), offline-first, bez cloudu.

---

## 1. Zpřesněné zadání (co bylo řečeno, co z toho plyne)

### Role a místa
| Role | Místo | Co potřebuje | Co dnes dělá |
|---|---|---|---|
| Kuchař (1–N stanic: gril, studená, dezert…) | Kuchyň, ruce špinavé/mokré | Vědět *co* dál, *pro koho*, *kolik toho čeká* — bez čtení | Čte bony z tiskárny, křičí přes pult |
| Expeditor / šéf předávky (někdy = kuchař) | Předávka (pass) | Sestavit stůl dohromady (všechny chody najednou), zavolat obsluhu | Zvonek, křik |
| Obsluha | Plac, pohyb, hluk hostů | *Kdy* a *kam* nést, co má přednost, co stárne na předávce | Chodí se ptát do kuchyně |
| Manažer | Kdekoliv | Doby přípravy, úzká hrdla, stížnosti | Nic nebo pocit |

### Zařízení (podle zadání)
1. **Chytrý reproduktor na předávce** — hlásí nové objednávky, urgence, "stůl 7 kompletní". Volitelně **zřetězené**
   reproduktory v kuchyni (jeden "mluvčí", více výstupů — stejný stream přes LAN, žádný Bluetooth).
2. **Display na place** (obsluha hlas neslyší) — tabule: *stůl → položky → ETA v minutách → stav (vaří se / hotovo na
   předávce / vyzvedni!)*. Jeden velký, čitelný z 3–5 m, žádné ovládání.
3. **Kuchyňská fronta** — objednávky se **řadí do fronty**, kuchaři si je **sami vyzvedávají** (claim). Systém zná
   odhad doby přípravy.
4. **Čekající objednávka měří čas čekání** (na claim, na vaření, na vyzvednutí z předávky) a eskaluje.

### Co z toho plyne (rozhodnutí, která si Opus může nárokovat jako dané)
- **Hlas ven = TTS** po LAN z jednoho serveru. Ne Alexa/Google (cloud, latence, čeština, soukromí). Vlastní.
- **Hlas dovnitř** (kuchař mluví do systému) je **volitelný** a vždy s *fyzickým potvrzením* (tlačítko/pedál). V hluku
  kuchyně (odsávání 70–80 dB) je spolehlivé jen: *wake-word → krátký povel → potvrzení*. Nikdy ne volný diktát jídel.
- **Vstup objednávek** v MVP: (a) adaptér na POS, (b) vlastní tablet obsluhy, (c) *nouzový* vstup: odposlech
  bonovací tiskárny (ESC/POS) — funguje s libovolnou pokladnou bez API.
- **Relax prostředí**: žádné blikání, žádné pípání na každou akci. Jeden klidný hlas, barevné "dýchání" na displeji,
  eskalace až po překročení prahu. Audio má **tichý režim** (jen displej + hodinky) pro večerní provoz.

---

## 2. Realistický pohled: kde to bolí a jak to obejít

| Problém | Realita | Řešení |
|---|---|---|
| ASR v kuchyni | Hluk, čeština, názvy jídel, přízvuk; ruce zaměstnané | Fáze 3; wake-word + omezená gramatika (čísla stolů, názvy z menu jako slovník); vždy fyzický fallback |
| TTS srozumitelnost | Odsávání překřičí reproduktor | Reproduktor **na předávce** (tišší místo), 10–20 W, krátké věty, opakování na žádost (tlačítko "ještě jednou") |
| "Jak dlouho to bude trvat" | Kuchaři to vědí, systém ne | Start: statické prep-time per položka (nastaví šéfkuchař, 2 min práce). Pak **učení z historie** (p50/p90 per položka per stanice per hodina) |
| Kdo si co vzal | Bez identifikace stanice ETA nefunguje | Claim = stisk na bump baru dané stanice (nebo NFC karta kuchaře). Objednávka bez claimu > N min → hlas urguje |
| Stůl má přijít najednou | Chody z více stanic | Expo logika: "fire" časování podle nejdelší položky; display na place ukazuje ETA *stolu*, ne položky |
| Integrace s pokladnou | Každá má jiné API/žádné | Adaptér vrstva + ESC/POS sniffer jako univerzální fallback (TCP 9100 proxy mezi POS a tiskárnou) |
| Síť vypadne | Restaurace nemá IT | Vše na jednom LAN boxu, displeje jen zobrazují; stav drží i při výpadku wifi; žádná závislost na internetu |
| Hluk na place, obsluha nekouká | Displej nestačí | Volitelně: levné hodinky (Android Wear / Garmin → stačí vibrace přes push na telefon) nebo pager |

Závěr: **MVP je udělatelné za pár týdnů** vlastními silami (Go + Flutter + Piper TTS). Nemožná není žádná část,
jen ASR je "drahá" a odkládá se.

---

## 3. Architektura

```
┌───────────── LAN (restaurace) ──────────────────────────────────────────┐
│                                                                         │
│  POS (Dotykačka/Storyous/Septim/…) ──API──┐                             │
│  Bonovací tiskárna  ◄──ESC/POS proxy──────┤                             │
│  Tablet obsluhy (Flutter) ────────────────┤                             │
│  Hlas (fáze 3, whisper.cpp) ──────────────┤                             │
│                                           ▼                             │
│                     ┌──────────── predavka-server (Go, 1 binárka) ───┐  │
│                     │ ingest → order store (event log, SQLite)       │  │
│                     │ queue/claim/ETA engine                          │  │
│                     │ rules: eskalace, aging, expo "fire"             │  │
│                     │ WS/SSE hub pro displeje                         │  │
│                     │ TTS (Piper cs, lokálně) → audio stream          │  │
│                     │ HTTP API + admin UI                             │  │
│                     └───┬──────────┬───────────┬──────────┬───────────┘  │
│                         ▼          ▼           ▼          ▼              │
│                  Reproduktor   Kuchyňský    Display     Hodinky/push      │
│                  na předávce   display +    na place    (volitelné)       │
│                  (+ chain)     bump bar     (TV/tablet)                   │
└─────────────────────────────────────────────────────────────────────────┘
```

### Komponenty
- **predavka-server** (Go): jedna binárka, SQLite, embedded web (admin + displeje jako web app i Flutter web),
  WebSocket hub, Piper TTS přes CGO nebo subprocess, audio out přes ALSA/PulseAudio nebo **Snapcast server**
  (ten řeší *zřetězení* reproduktorů: Snapcast klienti na ESP32/Pi Zero = synchronní multiroom za pár stovek).
- **Displeje**: Flutter (Android TV / tablet / web). Dva režimy: `kitchen` (stanice, fronta, claim), `floor` (stůl/ETA).
  Bez interakce na place; v kuchyni jen bump bar (USB HID) nebo dotyk velkých tlačítek.
- **Ingest adaptéry**: `pos-dotykacka`, `pos-storyous`, `pos-septim` (dle API dostupnosti), `escpos-proxy`
  (parsuje bony: číslo stolu, položky, množství, poznámky — regex/LLM-assisted parser per restaurace),
  `tablet` (vlastní), `voice` (fáze 3).
- **Hlas dovnitř (fáze 3)**: whisper.cpp / faster-whisper na LAN boxu (N100 zvládne `small` cs v reálném čase pro
  krátké povely) nebo offload na Spark doma přes VPN (ne pro produkci). Wake-word: openWakeWord. Gramatika omezená.

### Datový model
```
Order      {id, source, table, guests?, created_at, note, items[], status}
Item       {id, order_id, sku, name, qty, mods[], station, course, status, claimed_by, claimed_at, ready_at, picked_at}
Station    {id, name, display_id, prep_profile}
PrepTime   {sku, station, p50, p90, n, updated_at}     # učí se
Event      {ts, type, order_id?, item_id?, actor, payload}   # append-only, zdroj pravdy
```
Stavový automat položky: `NEW → CLAIMED → COOKING → READY → PICKED → SERVED` (+ `VOID`, `HOLD`).
Stav objednávky/stolu je odvozený (min/max ze stavů položek, "kompletní" = všechny položky chodu READY).

### Timery a eskalace (konfigurovatelné)
| Timer | Měří | Výchozí práh | Akce |
|---|---|---|---|
| wait-to-claim | NEW → CLAIMED | 2 min | hlas: "Stůl 7 čeká na kuchaře" (1×), displej kuchyň oranžová |
| cook overrun | CLAIMED → READY vs. p90 | p90 × 1.2 | displej kuchyň červená, bez hlasu (nechceme stresovat) |
| pass aging | READY → PICKED | 3 min | hlas: "Stůl 7 hotovo, na předávce 3 minuty"; plac: blikne řádek; hodinky vibrují |
| course gap | mezi chody | 8–12 min | expo: "Stůl 7, připravit hlavní" (fire) |

### ETA engine
1. Start: `eta_item = prep_time.p50(sku, station) + queue_ahead(station)`
2. `queue_ahead` = součet p50 nevyzvednutých položek před ní ve frontě stanice / paralelismus stanice.
3. `eta_table = max(eta_item pro aktuální chod)`.
4. Po claimu: `eta = claimed_at + p50 − now`, korekce z reálných časů ukončení (online průměr, exponenciální).
5. Display zobrazuje **zaokrouhleno na minuty** a mění se max 1× za 30 s (klid, ne tikání).

### Hlasové hlášky (TTS, čeština)
- Nová objednávka: "Stůl sedm: dvě svíčkové, jedna polévka." (jen předávka + stanice, kterých se týká)
- Claim potvrzení: krátký tón, ne řeč.
- Kompletní stůl: "Stůl sedm, kompletní." (plac dostane řádek zeleně)
- Urgence: viz tabulka timerů. Max 1 hláška / 20 s, frontování hlášek, priorita urgence > nová objednávka.
- Tichý režim: hlas vypnut, jen displeje + hodinky.

---

## 4. Hardware (BOM pro 1 restauraci, MVP)

| Kus | Co | Cca cena |
|---|---|---|
| 1 | LAN box: mini PC N100 16 GB (server + TTS + případně ASR) | 4–5 tis. Kč |
| 1 | Reproduktor předávka: aktivní 2.0/soundbar, voděodolnější, nebo Pi Zero 2 + Snapclient + zesilovač + repro | 1–3 tis. Kč |
| 0–2 | Zřetězené repro v kuchyni: ESP32-A1S/ Pi Zero 2 + Snapclient | 500–1 000 Kč/ks |
| 1–3 | Kuchyňský display: Android tablet 10–11" v IP krytu + držák, nebo levná TV + Android TV box | 3–6 tis. Kč/ks |
| 1–3 | Bump bar USB (nebo 4 velká tlačítka na ESP32 → HID) | 1–2 tis. Kč/ks |
| 1 | Display na place: 32–43" TV + Android TV box, nebo tablet | 4–8 tis. Kč |
| 1 | Switch/AP (pokud restaurace nemá slušnou wifi) | 1–2 tis. Kč |

Celkem MVP: ~20–30 tis. Kč na HW. Profi KDS (Partner Tech, Posiflex, PAX) jsou IP54 22" panely za řádově vyšší ceny —
nabídnout jako upgrade, ne nutnost.

---

## 5. Fáze a milníky

### Fáze 0 — Validace (1 týden, ne kód)
- Najít **1 pilotní restauraci** (ideálně známý; 20–60 míst, 2–4 kuchaři, 2–3 obsluha). Jedna směna pozorování:
  zapsat *skutečné* průtoky — kolik objednávek/h, kde se křičí, co stárne na předávce, jakou mají pokladnu.
- Výstup: `docs/pilot-observation.md` + seznam položek menu s odhady prep-time od šéfkuchaře.
- Rozhodnout ingest: API pokladny vs. ESC/POS proxy vs. vlastní tablet.

### Fáze 1 — MVP "slyšet a vidět" (3–4 týdny)
- `predavka-server`: ingest (tablet + ESC/POS proxy), event store, fronta, claim, timery, WS hub, TTS Piper cs,
  audio out, admin UI (menu, stanice, prep-time, prahy).
- Flutter display app: režim `kitchen` (fronta per stanice, claim velkým tlačítkem/bump bar, barvy) a `floor`
  (stůl, položky, ETA, stav; bez ovládání).
- Snapcast pro zřetězení repro.
- Simulátor provozu (generátor objednávek z reálného pozorování) pro ladění bez restaurace.
- Nasazení v pilotu, 2 týdny provozu, denní feedback.
- **Definition of done**: obsluha nechodí do kuchyně ptát se; kuchaři nečtou bony; manažer vidí časy.

### Fáze 2 — Integrace POS (2–3 týdny, paralelně s pilotem)
- Adaptéry podle pokladny pilotu; pak další podle poptávky (Dotykačka, Storyous/Teya, Septim).
- Webhooky/polling, mapování SKU → stanice.

### Fáze 3 — Hlas dovnitř (3–4 týdny, až po fázi 1 ověřené)
- openWakeWord + whisper.cpp na LAN boxu; gramatika: `"[stanice] beru stůl N"`, `"stůl N hotovo"`, `"ještě jednou"`,
  `"stůl N na později"`; vždy fyzické/akustické potvrzení; metriky WER v reálném hluku před zapnutím v pilotu.
- Volitelně: diktát objednávky obsluhou na place (tišší, mobil u pusy) — to je vlastně realističtější use-case než kuchyň.

### Fáze 4 — Učení a analytika (2 týdny)
- Online učení prep-time, ETA kalibrace (MAE v minutách), heatmapa úzkých hrdel, report po směně (e-mail/Discord).
- Export CSV, jednoduchý manažerský dashboard (web).

### Fáze 5 — Produkt (průběžně)
- Instalační "krabice" (image pro N100, QR párování displejů), onboarding do 1 hodiny.
- Ceník: HW za nákup + 1–2 tis. Kč/měsíc za SW a podporu (pod cenou KDS od Toast/Square, které jsou 20–35 USD/zařízení/měsíc, a bez vazby na konkrétní pokladnu).
- Zařadit na olin.now/business jako segment **gastro** (on-prem, bez cloudu, česky).

---

## 6. Konkurence (stav 10/2026, z veřejných zdrojů)

| Systém | Trh | Co umí | Co chybí vzhledem k Předávce |
|---|---|---|---|
| **Toast KDS** (US) | Součást Toast POS, ~35 USD/měs./zařízení + vlastní odolný HW | Expo + prep stanice, barvy dle času, notifikace serverovi na handheld (Toast Go) nebo SMS po dokončení, vícejazyčné lístky, "order ready" zprávy hostům | Jen s Toast POS (v CZ není), hlas žádný, "klid" ne — je to obrazovka plná lístků; ETA pro plac neukazuje |
| **Square KDS** (US/EU) | 20 USD/zařízení/měs. k Square for Restaurants | Levný, Android tablety, kitchen/expo obrazovky | Podle komunitních vláken **neumí notifikovat obsluhu, že je jídlo hotové**, slabý zvuk nové objednávky, bez stavu "in progress"; vázán na Square |
| **Fresh KDS** (US) | 15–89 USD/měs., nezávislý na POS | Až 10 obrazovek, hold/priorita, SMS hostům, analytika | Uživatelé hlásí odpojování; žádný hlas, žádný ETA display pro plac, iOS historicky chyběl |
| **Lightspeed KDS** (2026) | K Lightspeed POS | Real-time úpravy objednávek, jméno hosta, workflow priorit dine-in vs. delivery | Jen Lightspeed; žádný hlas; CZ okrajově |
| **QSR Automations ConnectSmart Kitchen** | Enterprise (Cheesecake Factory…); v ČR **exkluzivně přes Septim** | Nejbohatší: throttling podle vytížení, delayed routing, recepty, pacing, DineTime | Drahé, konzultační implementace, cílí na řetězce a Michelin; malá hospoda si to nekoupí |
| **Septim** (CZ) | Česká pokladna pro vyšší gastro | KDS (= QSR), skladu, receptury | Cena, složitost, cloudová závislost; bez hlasu |
| **Storyous / Teya** (CZ, ~4 000 provozů CZ/PL/ES) | Cloud POS, kiosk, terminály | Objednávky z kiosku rovnou do kuchyně/baru (tisk/bonování) | Veřejně žádný plnohodnotný KDS s ETA/frontou; cloud |
| **Dotykačka** (CZ, 75+ funkcí) | Cloud POS pro malé provozy | Mobilní číšník, mapa stolů, zákaznický displej, bonovací tiskárny, integrace (Wolt, Bolt…) | V dokumentaci jen *zákaznický* displej, kuchyň = tiskárna; žádná fronta/ETA/hlas |
| **Hlasové AI (Xina AI, hi.auto, voice.ai…)** | Drive-thru / objednávky od hostů | ASR objednávek od zákazníka → POS/KDS | Řeší *host → pokladna*, ne *kuchyň ↔ plac*; cloud, angličtina |
| **Pagery / zvonky / vysílačky** | Low-tech všude | Spolehlivé "přijď" | Bez obsahu, bez ETA, bez dat |

### Mezera, kterou zaplníme
1. **Hlas ven + displej na place s ETA** — nikdo z výše uvedených to nedělá; KDS je obrazovka pro kuchaře,
   obsluha je ignorovaná (viz Square vlákna).
2. **Nezávislost na pokladně** (ESC/POS proxy) — Fresh KDS je nezávislý, ale bez hlasu/ETA; ostatní vážou na svůj POS.
3. **On-prem, česky, bez cloudu, levný HW** — vše výše je cloud a anglicky nebo enterprise.
4. **Učení prep-time → ETA** — enterprise (QSR) to má, SMB nic.
5. **"Relax" design** — záměrně málo informací, žádné pípání; opak současných KDS.

Zdroje: fitsmallbusiness.com/best-kitchen-display-systems, merchantmaverick.com (KDS srovnání), community.squareup.com
(vlákna "How do staff know an order is ready", "Notification when prep is complete", "Custom Android KDS Notification Sound"),
support.toasttab.com (Order-ready texts, KDS notifications), digitaltransactions.net (Lightspeed KDS 2026),
septim.cz/en/products/kitchen-displays, lightspeedhq.com/integrations/qsr-automations-inc, storyous.com,
dotykacka.cz/aplikace-dotykacka, manual.dotykacka.cz (zákaznický displej), play.google.com (Fresh KDS).

---

## 7. Rizika a otevřené otázky

- **Pilot**: bez reálné restaurace je to hračka. Kdo? (Priorita 1 před kódem.)
- **POS API**: Dotykačka a Storyous mají partnerská API — prověřit podmínky/ceny; ESC/POS proxy jako plán B je
  ale univerzální a dá se postavit hned.
- **Hygiena/IP**: tablety v kuchyni potřebují kryt; reproduktor na předávce mimo páru.
- **Hluk vs. hlas**: změřit dB v pilotu, než se slíbí "slyší to celá kuchyň".
- **Právní**: žádná osobní data hostů (stoly, ne jména) → GDPR minimální. Hlasové nahrávky (fáze 3) nikam neukládat.
- **Název**: "Předávka" je česky trefné; pro export pracovně "Pass". Doménu zatím neřešit.

---

## 8. Co má Opus udělat jako první (konkrétní kroky)

1. Založit repo `lioilsources/predavka` (Go monorepo: `cmd/server`, `internal/{ingest,queue,eta,tts,hub,store}`,
   `apps/display` Flutter, `tools/sim`).
2. `docs/` : tento plán, `pilot-observation.md` (šablona), `voice-lines.md` (všechny hlášky cs), `config.example.yaml`.
3. Spike 1 (1 den): Piper cs TTS → Snapcast → 2 klienti synchronně. Změřit latenci od eventu k hlasu (< 1 s).
4. Spike 2 (1 den): ESC/POS proxy — zachytit bon z libovolné tiskárny (otestovat na virtuální: `nc -l 9100`),
   naparsovat stůl + položky.
5. Spike 3 (2 dny): Flutter display `floor` + `kitchen` nad fake WS feedem ze simulátoru.
6. Pak fáze 1 podle sekce 5. Reportovat do Director session po každém spiku.
