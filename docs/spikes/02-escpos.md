# Spike 2 — ESC/POS proxy

Datum: 2026-10-18.

## Jak to funguje
Pokladna tiskne na IP LAN boxu (port 9100) místo na tiskárnu. `internal/ingest/escpos` každé spojení:
1. přepošle bajty beze změny na skutečnou tiskárnu (`escpos.forward`) a stavové odpovědi (DLE EOT) zpět do pokladny,
2. rozdělí tiskovou úlohu na bony podle řezu papíru (`GS V`),
3. odstraní řídicí sekvence (ESC/GS/FS/DLE, obrázky, čárové kódy), dekóduje kódovou stránku
   (CP852 / CP1250 / UTF-8; `ESC t n` v datech má přednost před konfigurací),
4. regexy najde stůl, řádky `2x Svíčková  398,00` (cena se ořízne), modifikátory (`- bez brusinek`) a oddělovače chodů
   (`--- 2. chod ---`), přeskočí hlavičky/patičky (Číšník, Celkem…),
5. názvy z bonu spáruje s menu (název, SKU nebo `aliases`, bez ohledu na diakritiku). Neznámá položka jde na první
   stanici se statickým odhadem — kuchyň ji uvidí, nic se neztratí.

Když tiskárna nejde, bon se stejně zpracuje (displej) a do logu jde chyba.

## Výsledek
- Testy `internal/ingest/escpos/parse_test.go`: typický český bon (CP852, dvojitá výška, tučné, ceny, modifikátory,
  chod, řez), dva bony v jedné úloze, uzávěrka (není bon), prostý text. Vše prochází.
- End-to-end: `tools/escpos-send` → proxy (bez tiskárny) → objednávka stůl 7 (polévka NEW, svíčková + řízek HOLD
  jako 2. chod) → hláška „Stůl sedm: jedna polévka.".

## Rizika
- Každá pokladna má jiný layout bonu → regexy per restaurace (`escpos.table_re`, `item_re`, …). Vzorek bonu z pilotu
  přidat jako test.
- Pokladny, které tisknou bon jako **obrázek** (raster `GS v 0`), parser nepřečte — pak API nebo tablet.
- USB tiskárny: proxy chce síťovou tiskárnu; USB řešit přes tiskový server nebo virtuální tiskárnu v POS.

## Test bez tiskárny
```bash
go run ./cmd/server -config config.dev.yaml       # escpos.listen :9100, forward ""
go run ./tools/escpos-send -addr localhost:9100 -table 7
# nebo surová data: go run ./tools/escpos-send -dump > bon.bin && nc localhost 9100 < bon.bin
```
