BINARY   := hom3
BUILD_DIR := ./dist
MAIN      := main.go

.PHONY: build run test lint clean tidy

build:
	go build -o $(BUILD_DIR)/$(BINARY) $(MAIN)

run:
	go run $(MAIN)

test:
	go test ./...

lint:
	golangci-lint run ./...

tidy:
	go mod tidy

clean:
	rm -rf $(BUILD_DIR)
