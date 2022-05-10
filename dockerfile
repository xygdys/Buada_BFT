FROM golang:1.18-buster
WORKDIR ./Buada_BFT
COPY . .
RUN go mod download
