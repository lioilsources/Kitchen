# Předávka (Kitchen)

<!-- Max 80 řádků. Co se nevejde, jde do docs/ a sem jeden odkaz. Veřejné repo: žádné LAN IP, domovské cesty, ID. -->

## Účel
Lokální koordinace kuchyně a placu: hlasová hlášení objednávek, fronta s claimem, ETA na displejích.
Jeden Go server se SQLite na LAN, bez cloudu, česky.

## Build a test
```
make build        # bin/predavka-server, bin/sim, bin/escpos-send
make test         # gate: go vet + go test + flutter analyze + flutter test (apps/display)
make run          # server na :8088 (CONFIG=config.dev.yaml, nebo config.kids.yaml)
make sim          # simulovaný provoz
make app-web      # Flutter web build servírovaný na /app/
make linux        # binárka pro N100 box (bez CGO)
```

## Mapa
- `cmd/server` – predavka-server (jedna binárka, vestavěné web displeje)
- `internal/` – queue, eta, tts, ingest/escpos, hub (WebSocket), api, store (SQLite), config
- `web/` – displeje v prohlížeči (kitchen, pass, floor, tablet, admin)
- `apps/display` – Flutter aplikace (Android TV / tablet / web / macOS)
- `tools/` – simulátor a zkušební bon
- `deploy/` – systemd, Snapcast, piperd.py
- `docs/` – plán, protokol, hlášky, spiky, `config.example.yaml`

## Invarianty
- Stav objednávek je append-only event log v SQLite; po restartu se přehrává (viz README, „Jak to funguje“).
- Produkční `config.yaml` se necommituje (je v `.gitignore`); vzor je `docs/config.example.yaml`.

## Kam dál
README.md, `docs/PLAN.md`, `docs/protocol.md`, `Prompts/`
