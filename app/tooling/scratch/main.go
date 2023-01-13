package main

import (
	"fmt"
	"log"

	"github.com/dudakovict/blockchain/foundation/blockchain/genesis"
)

func main() {
	gen, err := genesis.Load()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(gen)
}
