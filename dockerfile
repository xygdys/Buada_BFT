FROM golang:1.18-buster
ENV GOPROXY https://goproxy.cn
WORKDIR ./Buada_BFT
COPY . .
RUN go mod download
