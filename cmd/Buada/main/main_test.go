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
	"sync"
	"testing"
)

func TestMain(m *testing.M) {

	N := 4
	wg1 := sync.WaitGroup{}
	wg2 := sync.WaitGroup{}
	wg1.Add(N)
	wg2.Add(N)
	for i := 0; i < N; i++ {
		go func(i int) {
			path := "../../../configs/config_" + fmt.Sprint(i) + ".yaml"
			c, _ := config.NewConfig(path, true)

			logg := logger.NewLoggerWithID("config", c.PID)
			tssConfig := config.TSSconfig{}
			err := tssConfig.UnMarshal(c.TSSconfig)
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
			wg1.Done()

			wg1.Wait()
			p.InitSendChannel()
			txlength := 250
			value := make([]byte, txlength*c.Txnum/c.N)
			rand.Read(value)

			benchmarkName := "Buada"
			if i == 0 {
				benchmark.InitBenchmark(c)
				benchmark.Create(benchmarkName)
				benchmark.Begin(benchmarkName, c.PID)
			}

			_, resultLen := aab.Buada(p, 0, value)

			log.Printf("results tx nums: %d", resultLen/txlength)
			if i == 0 {
				benchmark.End(benchmarkName, c.PID)
				benchmark.Nums(benchmarkName, c.PID, resultLen/txlength)
				_ = benchmark.BenchmarkOuput()
			}

			wg2.Done()
		}(i)
	}
	wg2.Wait()
}
