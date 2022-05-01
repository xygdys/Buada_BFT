package main

import (
	"bytes"
	"fmt"
)

func main() {
	var buf bytes.Buffer
	buf.WriteByte(1)
	buf.WriteByte(1)
	b1 := buf.Bytes()
	buf.Truncate(1)
	buf.WriteByte(2)
	b2 := buf.Bytes()
	fmt.Println(b1, b2)
}
