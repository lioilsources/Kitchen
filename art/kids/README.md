# Dětské ilustrace jídel

12 ikon pro dětskou verzi Předávky (`<sku>.png`, 512×512 RGBA, průhledné pozadí, předmět na střed s okrajem 8 %).

| | |
|---|---|
| Model | [FLUX.1-schnell](https://huggingface.co/black-forest-labs/FLUX.1-schnell) (Apache 2.0), NVIDIA NIM na DGX Sparku, 1024×1024, 4 kroky, seed 4242 |
| Ořez pozadí | [BiRefNet](https://github.com/ZhengPeng7/BiRefNet) `BiRefNet-general` (MIT) přes ComfyUI-RMBG, refine_foreground |
| Postproces | ořez na obsah, zmenšení na 512 px s okrajem 8 %; `palacinky.png` má navíc dorovnaný bílý okraj nálepky (dilatace alfy 13 px) |
| Vygenerováno | 21. 10. 2026 |

Prompt (`{subj}` podle tabulky níže):

> cute flat vector sticker illustration for a children's app, {subj}, single object centered, thick dark brown outline, bold cheerful saturated colors, simple rounded shapes, minimal shading, thick white sticker border around the object, plain light grey background, no text, no letters, no logo

| sku | subj |
|---|---|
| palacinky | two rolled crepes with red strawberry jam on a small plate |
| pizza | a whole round pepperoni pizza with melted cheese |
| hamburger | a big hamburger with sesame bun, lettuce, tomato and cheese |
| hranolky | golden french fries in a red paper cone |
| spagety | a plate of spaghetti with red tomato sauce |
| parek | a hot dog sausage in a long bread roll with a line of mustard |
| zmrzlina | an ice cream cone with two scoops, pink strawberry and white vanilla |
| dort | a slice of layered cream cake with one strawberry on top |
| limonada | a glass of yellow lemonade with a lemon slice and a striped straw |
| kakao | a cute round brown mug of hot chocolate drink with whipped cream on top |
| ovoce | a fruit plate with apple slices, banana, grapes and strawberries |
| chef | a happy smiling child chef wearing a tall white chef hat and white apron, waving, full body |
