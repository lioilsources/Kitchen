# Snapcast — zřetězené reproduktory (předávka + kuchyň)

Server (N100, stejný box jako predavka-server):

```ini
# /etc/snapserver.conf
[stream]
source = pipe:///tmp/snapfifo?name=predavka&sampleformat=22050:16:1&mode=create
buffer = 400          # ms; menší = rychlejší hláška, větší = odolnost proti výpadkům wifi

[http]
enabled = true        # web UI pro hlasitost jednotlivých reproduktorů: http://box:1780
```

`sampleformat` musí odpovídat hlasu Piper (`cs_CZ-jirka-medium` = 22050 Hz, 16 bit, mono).
predavka-server: `tts.output: snapfifo`, `tts.snap_fifo: /tmp/snapfifo`.

Klienti (Pi Zero 2 W + I²S zesilovač / USB DAC, nebo ESP32 se snapclient firmware):

```bash
sudo apt install snapclient
# /etc/default/snapclient
SNAPCLIENT_OPTS="--host predavka.local --latency 0"
```

Kontrola synchronizace: dva klienti vedle sebe → hláška nesmí znít ozvěnou. Hlasitost per reproduktor přes Snapweb.
