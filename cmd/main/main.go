package main

import (
	"time"

	"roach/internal"
)

func main() {
	server := internal.New(":8081")
	server.Start()

	time.Sleep(time.Second * 20)
}
