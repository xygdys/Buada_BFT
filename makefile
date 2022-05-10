all: build

.PHONY: build
build:
	go build -o ./build/start ./cmd/Buada/main

.PHONY: clean
clean:
	rm build/*
