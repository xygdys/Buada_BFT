package benchmark

import (
	"Buada_BFT/pkg/config"
	"Buada_BFT/pkg/utils/logger"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// benchmark in go test seems can't handle multi-routine program

var l *logger.Logger = logger.NewLogger("Benchmark")
var BenchmarkTable = map[string]*internal{}
var fileLocation string
var mu = sync.Mutex{}

func InitBenchmark(c config.Config) {
	fileLocation = c.Statistic
	if fileLocation == "" {
		l.Fatalf("fileLocation is empty")
	}
}

type internal struct {
	l      []int
	begin  map[int]int64
	end    map[int]int64
	txnums int
}

func Create(id string) {
	mu.Lock()
	BenchmarkTable[id] = &internal{
		l:      []int{},
		begin:  map[int]int64{},
		end:    map[int]int64{},
		txnums: 0,
	}
	mu.Unlock()
}

func Begin(id string, who int) {
	mu.Lock()
	BenchmarkTable[id].l = append(BenchmarkTable[id].l, who)
	BenchmarkTable[id].begin[who] = time.Now().UnixNano()
	mu.Unlock()
}

func End(id string, who int) {
	mu.Lock()
	BenchmarkTable[id].l = append(BenchmarkTable[id].l, who)
	BenchmarkTable[id].end[who] = time.Now().UnixNano()
	mu.Unlock()
}

func Nums(id string, who int, nums int) {
	mu.Lock()
	BenchmarkTable[id].l = append(BenchmarkTable[id].l, who)
	BenchmarkTable[id].txnums = nums
	mu.Unlock()
}

// return a map["consensus"]time(second)
func BenchmarkOuput() map[string]float64 {
	m := map[string]float64{}
	output := ""
	for key, value := range BenchmarkTable {
		value.l = removeRepByMap(value.l)
		tlist := []int64{}
		for _, index := range value.l {
			t := value.end[index] - value.begin[index]
			tlist = append(tlist, t)
		}
		m[key] = average(tlist)
	}

	format := "%-20s %-20s %-20s %-20s\n"
	output += fmt.Sprintf(format, "Name", "Latency(s)", "Throughput(tx/s)", "Nums")
	for name, t := range m {
		second := t / float64(time.Second)
		throughput := float64(BenchmarkTable[name].txnums) / second
		output += fmt.Sprintf(format, name,
			strconv.FormatFloat(second, byte('f'), 4, 64),
			strconv.FormatFloat(throughput, byte('f'), 4, 64),
			strconv.Itoa(BenchmarkTable[name].txnums))
	}

	f, err := os.OpenFile(fileLocation, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, os.ModePerm)
	if err != nil {
		l.Errorf("wirte log fail: %s", err.Error())
		return nil
	}
	defer f.Close()

	_, err = f.Write([]byte(output))
	if err != nil {
		l.Errorf("wirte log fail: %s", err.Error())
		return nil
	}

	// display
	// wait 0.5 second
	time.Sleep(500 * time.Millisecond)
	fmt.Println(strings.Repeat("=", 35) + "Test Result" + strings.Repeat("=", 35))
	fmt.Println()
	fmt.Println(output)

	return m
}

func average(s []int64) float64 {
	var length int = len(s)
	var sum int64 = 0
	for _, b := range s {
		sum += b
	}
	return float64(sum) / float64(length)
}

func removeRepByMap(slc []int) []int {
	result := []int{}
	tempMap := map[int]byte{} // 存放不重复主键
	for _, e := range slc {
		l := len(tempMap)
		tempMap[e] = 0
		if len(tempMap) != l { // 加入map后，map长度变化，则元素不重复
			result = append(result, e)
		}
	}
	return result
}
