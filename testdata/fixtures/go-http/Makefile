.PHONY: run build

run:
	go run ./cmd/server

build:
	CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o out ./cmd/server
