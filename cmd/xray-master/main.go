package main

import (
	"os"

	"github.com/thethoughtcriminal/xray-master/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
