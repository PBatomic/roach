package main

import (
	"os"
	"roach/internal"
)

func main() {
	os.Setenv("ROACH_TOKEN", "x")
	server := internal.New(":8081")
	server.Start()
}
