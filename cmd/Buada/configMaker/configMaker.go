package main

import (
	"Buada_BFT/pkg/config"
	"fmt"
	"log"
)

func main() {
	c, err := config.NewConfig("./config.yaml", true)
	if err != nil {
		log.Fatalln(err)
	}
	fmt.Println(c.N)
	c.RemoteGen("./configs")
}
