# Spike 3 — displeje `kitchen` / `pass` / `floor`

Datum: 2026-10-18.

Dvě implementace nad stejným protokolem (`docs/protocol.md`):

1. **Vestavěné web displeje** (`web/`, v binárce serveru) — fungují hned na libovolném prohlížeči / Android TV
   prohlížeči: `/kitchen.html?station=gril`, `/pass.html`, `/floor.html`, `/tablet.html`, `/admin.html`.
2. **Flutter aplikace** (`apps/display`) — Android TV / tablet / web, režimy kitchen, pass, floor, nastavení
   (URL serveru, režim, stanice, kuchař), bump bar, keep-screen-on, auto-reconnect. Web build lze servírovat
   ze serveru na `/app/` (`app_dir` v konfiguraci, build s `--base-href /app/`).

## Ověřeno
- Flutter: `flutter analyze` bez nálezů, `flutter test` (modely, obrazovky 1920×1080 i 400×800 bez přetečení,
  API proti mock serveru, **parsování skutečného snapshotu ze serveru** `test/fixtures/snapshot_server.json`),
  `flutter build web --release` i `flutter build apk --debug` prošly.
- Server: WebSocket klient dostal za 30 s 49 snapshotů + hlášky, simulátor (`tools/sim -cooks`) jel bez chyb.
- Web stránky: vrací 200, JS se parsuje (`node --check`).

## Neověřeno
- Vizuální kontrola na skutečné TV na 3–5 m a čitelnost v kuchyni.
- Reálný USB bump bar (mapování kláves je standardní, ale modely se liší).
