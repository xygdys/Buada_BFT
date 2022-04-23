package main

import (
	"context"
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
	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		//ctx1, cancel1 := context.WithCancel(ctx)

		go func() {
			for {
				select {
				case <-ctx.Done():
					fmt.Println("gorutine 11 stop")
					return
				}
			}
		}()

		for {
			select {
			case <-ctx.Done():
				fmt.Println("gorutine 1 stop")
				return
			}

		}
	}()

	go func() {

		go func() {
			for {
				select {
				case <-ctx.Done():
					fmt.Println("gorutine 22 stop")
					return
				}
			}
		}()

		for {
			select {
			case <-ctx.Done():
				fmt.Println("gorutine 2 stop")
				return

			}
		}

	}()

	time.Sleep(time.Second * 1)
	cancel()
	time.Sleep(time.Second * 1)

}
