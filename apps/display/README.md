# Předávka – displej

Flutter aplikace pro displeje v kuchyni, na předávce a na place. Data bere ze serveru
`predavka-server` (WebSocket `/ws` + HTTP API, viz `docs/protocol.md`).

## Režimy

- **Kuchyň** (`kitchen`) – fronta jedné stanice. Velké lístky: stůl, množství × jídlo, úpravy,
  čekání, ETA. HOLD je ztlumený, READY malé šedé na konci. Klepnutí na lístek = bump.
- **Předávka** (`pass`) – stoly seskupené dohromady: co je hotové a co se ještě vaří.
  Klepnutí na stůl = vyzvednuto (`/api/tables/{stůl}/pick`), tlačítko **Fire** pustí další chod.
- **Plac** (`floor`) – bez ovládání, čitelné z 3–5 m: stůl, jídla, „za 6 min“, stav.
  Hotové stoly nahoře zeleně, poslední hláška se na ~8 s ukáže dole.

Barvy se mění jen pomalým „dýcháním“ (oranžová = pozor, červená = pozdě), nic nebliká.
Tečka vpravo nahoře ukazuje spojení; když 12 s nic nepřijde, ukáže se
„Spojení se serverem ztraceno“ a zůstanou poslední data. Připojení se obnovuje samo.

## Nastavení

Při prvním spuštění, pak **dlouhým stiskem na název nahoře** (nebo klávesou F2 / Menu):

- adresa serveru – výchozí `http://predavka.local:8080`, na webu adresa, odkud se stránka načetla,
- režim – Kuchyň / Předávka / Plac,
- stanice (jen kuchyň) – načte se ze serveru, případně zadat ručně,
- jméno kuchaře (volitelné) – posílá se jako `by` při bumpu, jinak se posílá stanice.

Uloží se v zařízení.

## Bump bar (USB klávesnice, režim Kuchyň)

| Klávesa | Akce |
|---|---|
| `1`–`9` | vybrat n-tý lístek (číslo je v rohu lístku) |
| `Enter` / `Space` | bump vybraného, bez výběru nejstaršího na stanici |
| `←` `→` | posun výběru |
| `R` | zopakovat poslední hlášku |
| `Esc` | zrušit výběr |

## Spuštění

```sh
cd apps/display
flutter pub get
flutter run -d chrome          # web (vývoj)
flutter run -d macos           # macOS
flutter run -d <android-id>    # tablet / Android TV (adb devices)
```

- **Web**: `flutter build web --release`, obsah `build/web` může servírovat přímo server
  (pak se adresa serveru doplní sama). Při jiném původu musí server povolit CORS.
- **Android tablet / TV**: `flutter build apk --release`, nainstalovat `adb install`.
  Aplikace se objeví i v launcheru Android TV, displej nezhasíná, HTTP v LAN je povolené.
  Ovládat lze D-padem / klávesnicí, nastavení přes F2 nebo Menu.

## Testy

```sh
flutter analyze
flutter test
```
