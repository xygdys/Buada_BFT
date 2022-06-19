all: build

.PHONY: build
build:
	go build -o ./build/start ./cmd/Buada/main

.PHONY: config
config:
	go run ./cmd/Buada/configMaker

.PHONY: clean
clean:
	rm build/*
