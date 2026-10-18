.PHONY: build test run sim bon app-web linux

build:
	go build -o bin/predavka-server ./cmd/server
	go build -o bin/sim ./tools/sim
	go build -o bin/escpos-send ./tools/escpos-send

test:
	go vet ./...
	go test ./...
	cd apps/display && flutter analyze && flutter test

run: build
	./bin/predavka-server -config config.dev.yaml

sim:
	go run ./tools/sim -url http://localhost:8088 -rate 40 -cooks -speed 10

bon:
	go run ./tools/escpos-send -addr localhost:9100 -table 7

# Flutter web build servírovaný serverem na /app/ (config: app_dir: apps/display/build/web)
app-web:
	cd apps/display && flutter build web --release --base-href /app/

# N100 box (Linux amd64); čistý Go SQLite → bez CGO
linux:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o bin/predavka-server-linux-amd64 ./cmd/server
