package main

import (
	"log"

	"github.com/thehien0811/chatter/internal/grpcserver"
)

func main() {
	if err := grpcserver.Serve("9004", nil); err != nil {
		log.Fatal(err)
	}
}
