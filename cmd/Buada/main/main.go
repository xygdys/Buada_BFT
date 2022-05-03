package main

import (
	"fmt"

	"github.com/vivint/infectious"
)

func main() {
	const (
		required = 8
		total    = 14
	)

	// Create a *FEC, which will require required pieces for reconstruction at
	// minimum, and generate total total pieces.
	f, err := infectious.NewFEC(required, total)
	if err != nil {
		panic(err)
	}

	// Prepare to receive the shares of encoded data.
	shares := make([]infectious.Share, total)
	output := func(s infectious.Share) {
		// the memory in s gets reused, so we need to make a deep copy
		shares[s.Number] = s.DeepCopy()
	}

	// the data to encode must be padded to a multiple of required, hence the
	// underscores.
	text := "hello, world! __"
	err = f.Encode([]byte(text), output)
	if err != nil {
		panic(err)
	}

	fmt.Println("----------------")
	fmt.Println("Generated shares:")
	// we now have total shares.
	for _, share := range shares {
		fmt.Printf("%d: %#v\n", share.Number, string(share.Data))
	}

	// Let's reconstitute with shares 0-5 missing and 1 piece corrupted.
	shares = shares[4:]
	shares[4].Data[0] = '!' // mutate some data

	fmt.Println("----------------")
	fmt.Println("Fucked shares:")
	for _, share := range shares {
		fmt.Printf("%d: %#v\n", share.Number, string(share.Data))
	}

	err = f.Correct(shares)
	if err != nil {
		panic(err)
	}

	result, err := f.Decode(nil, shares)
	if err != nil {
		panic(err)
	}

	// we have the original data!
	fmt.Println("----------------")
	fmt.Println("Fixed shares:")
	fmt.Printf("original text:  %#v\n", string(text))
	fmt.Printf("recovered text: %#v\n", string(result))
}
