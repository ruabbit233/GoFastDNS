package main

import (
	"GoFastDNS/internal/cli"
	"os"
)

func main() {
	os.Exit(cli.Run(os.Args[1:]))
}
