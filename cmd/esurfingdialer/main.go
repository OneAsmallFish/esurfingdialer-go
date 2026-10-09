package main

import (
	"log"
	"os"

	"esurfingdialer/internal/app"
	"esurfingdialer/internal/config"
)

func main() {
	cfg, err := config.ParseArgs(os.Args[1:])
	if err != nil {
		log.Printf("config error: %v", err)
		os.Exit(2)
	}

	if err := app.Run(cfg); err != nil {
		log.Printf("run error: %v", err)
		os.Exit(1)
	}
}
