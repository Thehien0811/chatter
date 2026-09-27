package main

import (
	"log"

	"github.com/thehien0811/chatter/internal/gateway"
)

func main() {
	app := gateway.NewApp("")
	log.Println("API gateway listening on :8080")
	if err := app.Run(":8080"); err != nil {
		log.Fatal(err)
	}
}
