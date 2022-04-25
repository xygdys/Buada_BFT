package main

import (
	"context"
	"fmt"
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
	ctx1, cancel1 := context.WithCancel(context.Background())
	ctx2, _ := context.WithCancel(ctx1)

	go func(ctx context.Context) {
		select {
		case <-ctx.Done():
			return
		}

	}(ctx1)
	go func(ctx context.Context) {
		for {
			select {
			case <-ctx.Done():
				fmt.Println("Yes")
				return
			}
		}

	}(ctx2)

	go func() {
		cancel1()
		return

	}()

	for {

	}

}
