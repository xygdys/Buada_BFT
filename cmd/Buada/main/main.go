package main

import (
	"Buada_BFT/internal/aab"
	"Buada_BFT/internal/party"
	"Buada_BFT/pkg/config"
	"Buada_BFT/pkg/utils/benchmark"
	"Buada_BFT/pkg/utils/logger"
	"crypto/rand"
	"fmt"
	"log"
	"time"
)

func main() {
	c, err := config.NewConfig("./config.yaml", true)
	if err != nil {
		fmt.Println(err)
	}
	logg := logger.NewLoggerWithID("config", c.PID)
	tssConfig := config.TSSconfig{}
	err = tssConfig.UnMarshal(c.TSSconfig)
	if err != nil {
		logg.Fatalf("fail to unmarshal tssConfig: %s", err.Error())
	}
	tseConfig := config.TSEconfig{}
	err = tseConfig.UnMarshal(c.TSEconfig)
	if err != nil {
		logg.Fatalf("fail to unmarshal tseConfig: %s", err.Error())
	}

	p := party.NewHonestParty(uint32(c.N), uint32(c.F), uint32(c.PID), c.IPList, c.PortList, tssConfig.Pk, tssConfig.Sk, tseConfig.Tpke)
	p.InitReceiveChannel()

	//time.Sleep(time.Duration(c.PrepareTime))
	time.Sleep(time.Second * time.Duration(c.PrepareTime))

	p.InitSendChannel()

	txlength := 250
	value := make([]byte, txlength*c.Txnum/c.N)
	rand.Read(value)

	benchmarkName := "Buada"
	benchmark.InitBenchmark(c)
	benchmark.Create(benchmarkName)
	benchmark.Begin(benchmarkName, c.PID)

	_, resultLen := aab.Buada(p, 0, value)

	log.Printf("results tx nums: %d", resultLen/txlength)
	benchmark.End(benchmarkName, c.PID)
	benchmark.Nums(benchmarkName, c.PID, resultLen/txlength)
	_ = benchmark.BenchmarkOuput()

	time.Sleep(time.Second * time.Duration(c.WaitTime))
}
