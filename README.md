# Předávka

Klidná koordinace kuchyně a placu bez koukání do obrazovek. Kuchaři **slyší** nové objednávky z reproduktoru
na předávce, berou si je z fronty jedním stiskem, obsluha vidí na jednom displeji **za kolik minut co dorazí a na
jaký stůl**, a jídlo, které leží na předávce, si po pár minutách samo řekne o vyzvednutí.

Vše běží lokálně na LAN (jeden mini PC), bez cloudu, česky. Plán: [`docs/PLAN.md`](docs/PLAN.md).

## Stav (2026-10-18)

| Krok plánu (§8) | Stav |
|---|---|
| Repo, struktura, docs | ✓ |
| Spike 1: Piper cs → Snapcast, latence < 1 s | ✓ 136–196 ms do Snapcast FIFO (`piperd`); reálný snapserver + 2 klienti zatím neověřeno → [zpráva](docs/spikes/01-tts-snapcast.md) |
| Spike 2: ESC/POS proxy | ✓ parser + proxy + testy → [zpráva](docs/spikes/02-escpos.md) |
| Spike 3: displeje `kitchen` / `floor` | ✓ web (vestavěný) + Flutter → [zpráva](docs/spikes/03-display.md) |
| Fáze 1 jádro: event store, fronta, claim, ETA, učení prep-time, eskalace, fire chodů, WS hub, admin, simulátor | ✓ (bez pilotu) |
| Fáze 0 (pilotní restaurace), fáze 2–5 | – |

## Rychlý start (macOS, vývoj)

```bash
make run          # server na :8088, hlas přes `say` (Zuzana), ESC/POS proxy na :9100 bez tiskárny
make bon          # pošle zkušební bon (stůl 7) do proxy
make sim          # simulovaný provoz: objednávky + kuchaři + expo
```

Otevři http://localhost:8088 → plac, předávka, tablet, nastavení a jednotlivé stanice kuchyně.

## Komponenty

```
cmd/server            predavka-server (jedna binárka, SQLite, vestavěné web displeje)
internal/queue        stav objednávek z event logu, přechody, snapshoty, časovače a eskalace, fire chodů
internal/eta          prep-time (statický → naučený p50/p90 z posledních 50 vaření), ETA položek a stolů
internal/tts          hlášky v češtině (číslovky, skloňování), fronta hlášek s prioritou, Piper/say → přehrávač/Snapcast
internal/ingest/escpos  proxy mezi pokladnou a bonovací tiskárnou + parser bonů (CP852/CP1250)
internal/hub          WebSocket snapshoty pro displeje
internal/api          HTTP API (docs/protocol.md)
internal/store        SQLite: append-only události, vzorky doby přípravy, nastavení
web/                  displeje v prohlížeči: kitchen, pass, floor, tablet, admin
apps/display          Flutter aplikace (Android TV / tablet / web)
tools/sim             simulátor provozu
tools/escpos-send     zkušební bon do proxy/tiskárny
deploy/               systemd, Snapcast, piperd.py
docs/                 plán, protokol, hlášky, šablona pozorování pilotu, zprávy ze spiků, config.example.yaml
```

## Jak to funguje

- **Stav položky:** `NEW → CLAIMED → COOKING → READY → PICKED → SERVED` (+ `HOLD` pro další chod, `VOID`).
  Každá změna je událost v SQLite; po restartu se stav přehraje z logu.
- **Chody:** položky vyššího chodu čekají v `HOLD`. Auto fire je uvolní, když je předchozí chod venku a uplynula
  `course_gap` mínus doba přípravy nejdelší položky — hlavní tak dorazí zhruba po mezeře. Expo může fire dát ručně.
- **ETA:** `fronta před položkou / paralelismus stanice + p50`; po claimu `claimed_at + p50 − teď`.
  ETA stolu = max položek aktuálního chodu; na placu se mění nejvýš 1× za 30 s.
- **Učení:** každé claim → ready je vzorek; od 5 vzorků se místo odhadu šéfkuchaře použije naměřené p50/p90.
- **Eskalace:** viz tabulka v [`docs/voice-lines.md`](docs/voice-lines.md). Přetažené vaření jen displej, bez hlasu.
- **Tichý režim:** bez hlasu, displeje + push (webhook, např. ntfy → hodinky) běží dál.

## Produkce (N100, Linux)

```bash
make linux                                  # bin/predavka-server-linux-amd64 (bez CGO)
# /opt/predavka: binárka, config.yaml (z docs/config.example.yaml), venv s piper-tts, hlas cs_CZ-jirka-medium
sudo cp deploy/predavka.service /etc/systemd/system/ && sudo systemctl enable --now predavka
```

Snapcast a reproduktory: [`deploy/snapcast.md`](deploy/snapcast.md). Displeje: Android TV/tablet s Flutter
aplikací, nebo prohlížeč na `http://predavka.local:8080/floor.html`.

## Testy

```bash
make test         # go vet + go test + flutter analyze + flutter test
```
