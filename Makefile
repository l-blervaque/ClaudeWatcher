BINARY  := cw
BUILD   := ./cmd/cw
INSTALL := /usr/local/bin/$(BINARY)

.PHONY: build install run test clean

build:
	go build -o $(BINARY) $(BUILD)

install: build
	cp $(BINARY) $(INSTALL)
	@echo "Installed → $(INSTALL)"

run:
	go run $(BUILD)

test:
	go test ./...

clean:
	rm -f cw claudewatcher
