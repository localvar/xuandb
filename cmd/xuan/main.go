package main

import (
	"flag"
	"fmt"

	"github.com/localvar/xuandb/pkg/config"
	"github.com/localvar/xuandb/pkg/version"
)

func main() {
	flag.Parse()

	if config.ShowVersion() {
		fmt.Println("xuandb cli version:", version.Version())
		fmt.Println("Built with:", version.GoVersion())
		fmt.Println("Git commit:", version.Revision())
		if version.LocalModified() {
			fmt.Println("Warning: this build contains uncommitted changes.")
		}
		return
	}

	if err := config.Load(""); err != nil {
		fmt.Println("Failed to load configuration:", err)
		return
	}
}
