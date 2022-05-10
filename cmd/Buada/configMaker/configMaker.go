package main

import (
	"Buada_BFT/pkg/config"
	"log"
)

func main() {
	c, err := config.NewConfig("./config.yaml", true)
	if err != nil {
		log.Fatalln(err)
	}
	c.RemoteGen("../buildthings")
}
