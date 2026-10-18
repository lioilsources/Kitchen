# Spike 1 — Piper cs TTS → Snapcast, latence událost → hlas

Datum: 2026-10-18. Měřeno na vývojovém Macu (Apple Silicon), ne na N100.

## Výsledek
| Varianta | Latence (událost → začátek audia) |
|---|---|
| `piper` CLI spouštěné pro každou větu (Python piper-tts 1.x) | **~1,1–1,3 s** (načtení modelu při každém volání) ✗ |
| `piperd` — perzistentní proces s modelem v paměti (`deploy/piperd.py`) | **136–196 ms** přes celý server až do Snapcast FIFO ✓ |
| čistá syntéza v procesu (krátká věta) | 70–184 ms |
| macOS `say -v Zuzana` (jen vývoj) | ~1,2 s |

Model: `cs_CZ-jirka-medium` (63 MB, 22050 Hz, 16 bit, mono). Věta „Stůl sedm: dvě svíčkové, jedna polévka." = ~4 s audia.

**Závěr:** cíl < 1 s je splněn s `engine: piperd`. Na N100 očekávám 3–5× pomalejší syntézu (~0,3–0,8 s) — ověřit
na cílovém HW (`GET /api/stats` → `voice_latency_ms`). Jednorázové volání `piper` nechat jen jako fallback.

## Co bylo ověřeno
- Server → `piperd` → WAV → raw PCM → FIFO (`output: snapfifo`): data dorazila (7,8 s audia za 2 hlášky, formát sedí).
- Odstup mezi hláškami drží i u FIFO (server čeká délku audia, zápis do roury se jinak vrátí hned).

## Co ověřeno NEBYLO
- Skutečný `snapserver` + 2 `snapclient` synchronně — na vývojovém stroji Snapcast není nainstalován.
  Postup je v `deploy/snapcast.md`; latenci Snapcastu přidává jeho buffer (výchozí 1000 ms → pro hlášky
  nastavit `buffer = 300–400`).
- Srozumitelnost v hluku kuchyně.

## Jak zopakovat
```bash
python3 -m venv /opt/predavka/venv && /opt/predavka/venv/bin/pip install piper-tts
curl -LO https://huggingface.co/rhasspy/piper-voices/resolve/main/cs/cs_CZ/jirka/medium/cs_CZ-jirka-medium.onnx
curl -LO https://huggingface.co/rhasspy/piper-voices/resolve/main/cs/cs_CZ/jirka/medium/cs_CZ-jirka-medium.onnx.json
# config.yaml: tts: {engine: piperd, piper_python: /opt/predavka/venv/bin/python, piper_model: …onnx, output: snapfifo, snap_fifo: /tmp/snapfifo}
curl -XPOST localhost:8080/api/orders -d '{"table":"7","items":[{"sku":"svickova","qty":2}]}'
curl localhost:8080/api/stats   # voice_latency_ms
```
