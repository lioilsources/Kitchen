# Hlasové hlášky (čeština)

Zdroj pravdy je kód: `internal/tts/lines.go` (věty) a `internal/tts/czech.go` (číslovky, skloňování).
Testy: `internal/tts/czech_test.go`.

## Zásady
- Krátké věty, **číslo stolu vždy první**. Žádná čísla jako cifry — vše se vyslovuje slovy ("stůl dvanáct").
- Max 1 hláška za `announce_min_gap` (výchozí 20 s). Priorita: *ještě jednou* > urgence > kompletní / fire > nová objednávka.
- Stejná urgence ve frontě se nahradí novější (žádné dvojí "stůl 7 na předávce").
- Nová objednávka starší než 2 min se nevysloví (displej ji už ukazuje).
- Tichý režim: nic se nevysloví, ale titulek na displejích a push (webhook) běží dál.
- Potvrzení claimu: krátký tón (`tts.tone_file`), ne řeč.

## Hlášky

| Situace | Kdy | Text | Typ (`kind`) |
|---|---|---|---|
| Nová objednávka | přijde bon / tablet; jen položky aktuálního chodu | „Stůl sedm: dvě svíčkové, jedna polévka." | `new_order` |
| …s úpravou | některá položka má modifikátor | „Stůl sedm: dvě svíčkové, s úpravou." | `new_order` |
| Čeká na kuchaře | NEW déle než `wait_to_claim` (2 min), 1× na stůl+stanici | „Stůl sedm, gril, čeká na kuchaře." | `wait_claim` |
| Kompletní | poslední položka chodu READY | „Stůl sedm, kompletní." | `complete` |
| Stárne na předávce | nejstarší READY chodu déle než `pass_aging` (3 min), opakuje se po `reannounce_aging` | „Stůl sedm hotovo, na předávce tři minuty." | `pass_aging` |
| Fire dalšího chodu | auto (`course_gap`) nebo expo tlačítkem | „Stůl sedm, připravit hlavní." | `fire` |
| Ještě jednou | tlačítko / klávesa R / `POST /api/repeat` | poslední hláška znovu | `repeat` |
| Přetažené vaření | vaření > p90 × 1,2 | **bez hlasu** — jen displej červeně | – |

## Tvary jídel
Každá položka menu má v konfiguraci `say: {one, few, many, gender}`:

```yaml
say: {one: svíčková, few: svíčkové, many: svíčkových, gender: f}   # jedna / dvě / pět
say: {one: řízek, few: řízky, many: řízků, gender: m}               # jeden / dva / pět
say: {one: tiramisu, few: tiramisu, many: tiramisu, gender: n}      # jedno / dvě / pět
say: {one: porce hranolek, few: porce hranolek, many: porcí hranolek, gender: f}  # pomnožná jména přes „porce"
```

Číslo stolu: čísla jako při počítání („stůl jedna", „stůl dva"), alfanumerické označení se čte po částech
(„T3" → „T tři", „bar 2" → „bar dva").

## K ověření v pilotu
- Srozumitelnost v hluku odsávání (dB na předávce), hlasitost, zda je lepší hlas `jirka` nebo jiný.
- Jestli kuchaři chtějí při nové objednávce slyšet i úpravy („bez cibule"), nebo stačí „s úpravou".
- Délka `announce_min_gap` ve špičce — 20 s může při 50 obj./h vytvářet frontu (staré hlášky se zahazují).
