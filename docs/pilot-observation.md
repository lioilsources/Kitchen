# Pozorování pilotu — šablona

Podnik: ______  Datum: ______  Směna (od–do): ______  Pozoroval: ______

## Provoz
| | Hodnota |
|---|---|
| Míst k sezení | |
| Kuchařů / stanic (jaké) | |
| Obsluha na place | |
| Expeditor? (kdo stojí na předávce) | |
| Pokladna (značka, verze, má API?) | |
| Bonovací tiskárna (model, síť/USB, IP) | |
| Kódová stránka bonu (zkusit `nc -l 9100 > bon.bin`) | |
| Wi-Fi v kuchyni (signál, kdo ji spravuje) | |

## Průtok (čárkovat po 15 min)
| Čas | Objednávek | Položek | Kolikrát obsluha přišla do kuchyně se ptát | Kolikrát se křičelo přes pult |
|---|---|---|---|---|
| | | | | |

## Předávka
- Jak dlouho leží hotové jídlo, než si ho obsluha vezme? (stopky na 10 talířů): ______
- Co se stane, když leží dlouho? (ohřívá se, vrací, nic): ______
- Jak se dnes volá obsluha? (zvonek, křik, nic): ______
- Hluk na předávce / v kuchyni (dB, aplikace v mobilu): ______ / ______

## Chody
- Servíruje se po chodech? Kdo rozhoduje, kdy začít hlavní? ______
- Typická mezera předkrm → hlavní: ______

## Menu a doby přípravy (od šéfkuchaře)
| Položka (jak je na bonu) | Stanice | Typicky (min) | Ve špičce (min) | Poznámka |
|---|---|---|---|---|
| | | | | |

→ přepsat do `config.yaml` (`menu:`), `prep` = typicky, `prep_p90` = ve špičce.

## Vzorek bonu
Přiložit foto bonu + surová data (`go run ./tools/escpos-send -dump` je vzor; reálný bon zachytit proxy bez forward
nebo `nc -l 9100 > bon.bin`). Ověřit `go test ./internal/ingest/escpos` s tímto bonem.

## Rozhodnutí
- [ ] Ingest: API pokladny / ESC/POS proxy / tablet
- [ ] Kde bude reproduktor (mimo páru), kolik zřetězených
- [ ] Kde bude displej na place (vzdálenost čtení)
- [ ] Bump bar nebo dotyk
