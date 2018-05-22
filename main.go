package main

import (
	"flag"
	"log"
)

var _config *conf

func main() {
	var configfile string
	flag.StringVar(&configfile, "c", "config.yaml", "config file")
	flag.Parse()

	cfg, err := loadConfig(configfile)
	if err != nil {
		log.Fatalf("load config file error: %s", err)
	}
	_config = cfg

	initHandler()
	select {}
}
