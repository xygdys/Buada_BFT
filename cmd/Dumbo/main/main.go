package main

import (
	"fmt"
	"time"
)

func main() {
	// port := "8882"
	// receiveChannel := core.MakeReceiveChannel(port)
	// for {
	// 	m := <-(receiveChannel)
	// 	fmt.Println("The Message Received from channel is")
	// 	fmt.Println("id==", m.Type)
	// 	fmt.Println("sender==", m.Sender)
	// 	fmt.Println("len==", len(m.Data))
	// }
	m := [1000]int{0}
	go func() {
		for i := 0; i < 1000; i++ {
			go func(i int) {
				m[10]++
			}(i)

		}
	}()
	time.Sleep(time.Second * 1)
	fmt.Println(m[10])

}
