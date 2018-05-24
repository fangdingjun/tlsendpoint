package main

import (
	"flag"
	"fmt"
	"os"

	log "github.com/fangdingjun/go-log"
	"github.com/fangdingjun/go-log/formatters"
	"github.com/fangdingjun/go-log/writers"
)

var _config *conf

func main() {
	var configfile string
	var logfile string
	var loglevel string
	var logFileSize int64
	var logKeepCount int

	flag.StringVar(&configfile, "c", "config.yaml", "config file")
	flag.StringVar(&logfile, "log_file", "", "log file path, default to stdout")
	flag.StringVar(&loglevel, "log_level", "INFO", "log level, levels are: \nOFF, FATAL, PANIC, ERROR, WARN, INFO, DEBUG")
	flag.Int64Var(&logFileSize, "log_file_size", 10, "max log file size, MB")
	flag.IntVar(&logKeepCount, "log_count", 10, "max count of log file to keep")
	flag.Parse()

	if logfile != "" {
		log.Default.Out = &writers.FixedSizeFileWriter{
			Name:     logfile,
			MaxSize:  logFileSize * 1024 * 1024,
			MaxCount: logKeepCount,
		}
	}
	if loglevel != "" {
		lvname, err := log.ParseLevel(loglevel)
		if err != nil {
			fmt.Fprintf(os.Stderr, "invalid level %s", loglevel)
			os.Exit(1)
		}
		log.Default.Level = lvname
	}

	log.Default.Formatter = &formatters.TextFormatter{
		TimeFormat: "2006-01-02 15:04:05.000",
	}

	cfg, err := loadConfig(configfile)
	if err != nil {
		log.Fatalf("load config file error: %s", err)
	}
	_config = cfg
	log.Debugf("config: %+v", _config)
	initHandler()
	select {}
}
