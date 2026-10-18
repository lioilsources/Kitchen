#!/usr/bin/env python3
"""piperd — Piper TTS with the voice kept in memory (spouští ho predavka-server, engine: piperd).

Protocol (stdin/stdout, one line each): request = output WAV path, TAB, text; response = path or "ERR <msg>".
Loading the model takes ~1 s; a short sentence then synthesizes in ~0.1–0.4 s (vs ~1.1 s per piper call).
"""
import sys
import wave

from piper import PiperVoice


def main() -> None:
    if len(sys.argv) != 2:
        sys.exit("usage: piperd.py MODEL.onnx")
    voice = PiperVoice.load(sys.argv[1])
    print("READY", flush=True)
    for line in sys.stdin:
        path, _, text = line.rstrip("\n").partition("\t")
        try:
            with wave.open(path, "wb") as w:
                voice.synthesize_wav(text, w)
            print(path, flush=True)
        except Exception as e:  # noqa: BLE001 — server jen zaloguje a pokračuje
            print("ERR " + str(e).replace("\n", " "), flush=True)


if __name__ == "__main__":
    main()
