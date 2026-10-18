# Protokol displejů (server ↔ klienti)

Server: `predavka-server`, výchozí port `8080`. Vše na LAN, bez autentizace (MVP).

## WebSocket `GET /ws`

Server po připojení pošle hned `snapshot` a pak nový `snapshot` po každé změně stavu
(nejvýš 4× za sekundu) a minimálně každých 5 s (heartbeat; klient podle něj pozná výpadek).
Klient nic neposílá (akce jdou přes HTTP).

Zprávy (JSON, jedna zpráva = jeden WS text frame):

```json
{"type": "snapshot", "data": Snapshot}
{"type": "announce", "data": {"text": "Stůl sedm, kompletní.", "kind": "complete", "ts": "2026-10-18T12:00:00Z"}}
{"type": "tone",     "data": {"kind": "claim"}}
```

`announce.kind`: `new_order | complete | wait_claim | pass_aging | fire | repeat | info`.
Displeje mohou hlášku krátce ukázat jako titulek (klidně, bez animace).

### Snapshot

```json
{
  "now": "2026-10-18T12:00:00Z",
  "quiet": false,
  "stations": [{"id": "gril", "name": "Gril"}, {"id": "studena", "name": "Studená"}],
  "items": [ItemView],
  "tables": [TableView]
}
```

`items` obsahuje jen **aktivní** položky (stav `HOLD`, `NEW`, `CLAIMED`, `COOKING`, `READY`),
seřazené podle pořadí ve frontě (nejstarší první).

### ItemView

```json
{
  "id": "it_01J...",
  "order_id": "or_01J...",
  "table": "7",
  "sku": "svickova",
  "name": "Svíčková",
  "qty": 2,
  "mods": ["bez brusinek"],
  "note": "",
  "station": "gril",
  "course": 1,
  "status": "NEW",
  "claimed_by": "",
  "created_at": "2026-10-18T11:58:00Z",
  "claimed_at": null,
  "ready_at": null,
  "wait_s": 120,
  "eta_min": 9,
  "start_in_min": 0,
  "alert": "none"
}
```

- `status`: `HOLD | NEW | CLAIMED | COOKING | READY` (ve snapshotu; `PICKED/SERVED/VOID` jsou neaktivní).
- `wait_s`: sekundy v aktuálním stavu (NEW → čeká na claim, CLAIMED/COOKING → vaří se, READY → leží na předávce).
- `eta_min`: odhad minut do READY (zaokrouhleno nahoru, 0 = hotovo/za chvíli). U READY vždy 0.
- `start_in_min`: expo doporučení, kdy začít, aby chod dorazil najednou (0 = hned).
- `alert`: `none | warn | late`
  - NEW: `warn` po překročení `wait_to_claim`
  - CLAIMED/COOKING: `late` po překročení `p90 × cook_overrun_factor`
  - READY: `warn` po `pass_aging`, `late` po 2× `pass_aging`

### TableView (pro displej na place)

```json
{
  "table": "7",
  "status": "cooking",
  "eta_min": 6,
  "course": 1,
  "items_total": 3,
  "items_ready": 1,
  "ready_wait_s": 0,
  "alert": "none",
  "items": [{"name": "Svíčková", "qty": 2, "status": "COOKING"}]
}
```

- `status`: `waiting` (nic nezačato) | `cooking` | `partial` (část chodu na předávce) | `ready` (chod kompletní na předávce, **vyzvedni**) | `hold` (čeká na další chod)
- `eta_min` je ETA **stolu** = max ETA položek aktuálního chodu; mění se nejvýš 1× za `eta_refresh` (30 s).
- `ready_wait_s`: jak dlouho nejstarší READY položka chodu leží na předávce.
- `alert`: `none | warn | late` (pass aging).
- Řazení: `ready` nahoře (nejdéle čekající první), pak podle `eta_min` vzestupně.

## HTTP API

| Metoda | Cesta | Tělo | Účinek |
|---|---|---|---|
| GET | `/api/state` | – | aktuální Snapshot |
| POST | `/api/orders` | `{"table":"7","source":"tablet","note":"","items":[{"sku":"svickova","qty":2,"mods":[],"course":1}]}` | nová objednávka |
| POST | `/api/items/{id}/bump` | `{"by":"gril"}` | posun o stav dál: NEW→CLAIMED, CLAIMED/COOKING→READY, READY→PICKED |
| POST | `/api/items/{id}/claim` | `{"by":"Petr"}` | NEW→CLAIMED |
| POST | `/api/items/{id}/start` | – | CLAIMED→COOKING |
| POST | `/api/items/{id}/ready` | – | →READY |
| POST | `/api/items/{id}/pick` | – | READY→PICKED |
| POST | `/api/items/{id}/serve` | – | PICKED→SERVED |
| POST | `/api/items/{id}/void` | – | →VOID |
| POST | `/api/items/{id}/hold` | – | NEW→HOLD |
| POST | `/api/stations/{id}/bump` | `{"by":"..."}` | bump nejstarší položky stanice (pro bump bar bez výběru) |
| POST | `/api/tables/{table}/pick` | – | všechny READY položky stolu → PICKED (expo) |
| POST | `/api/tables/{table}/fire` | – | HOLD položky dalšího chodu → NEW |
| POST | `/api/repeat` | – | zopakuj poslední hlášku |
| POST | `/api/quiet` | `{"on":true}` | tichý režim |
| GET | `/api/menu` | – | menu z konfigurace |
| GET/PUT | `/api/settings` | Settings | prahy (minuty/sekundy) |
| GET | `/api/stats` | – | prep-time statistiky |

Chyby: HTTP 4xx s `{"error":"..."}`.

## Bump bar (klávesnice)

Bump bary jsou USB HID klávesnice. Kuchyňský displej mapuje:

| Klávesa | Akce |
|---|---|
| `1`–`9` | vybrat n-tý lístek |
| `Enter` / `Space` | bump vybraného (bez výběru nejstaršího) |
| `←` `→` | posun výběru |
| `R` | zopakuj poslední hlášku |
| `Esc` | zrušit výběr |
