.PHONY: build test
build:
	go build -o bin/xray-master ./cmd/xray-master
test:
	go test ./...
