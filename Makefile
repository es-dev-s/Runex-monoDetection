.PHONY: test run tidy

test:
	go test -race -count=1 ./...

tidy:
	go mod tidy

run:
	go run ./cmd/server
