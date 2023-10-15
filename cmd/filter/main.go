package main

import (
	"flag"
	"log"
	"os"

	"github.com/GuanceCloud/cliutils/filter"
)

var (
	conditionFile string
)

func init() {
	flag.StringVar(&conditionFile, "condition-path", "", "condition file")
}

func main() {
	flag.Parse()

	condData, err := os.ReadFile(conditionFile)
	if err != nil {
		log.Printf("ReadFile: %s", err)
		os.Exit(-1)
	}

	where, err := filter.GetConds(string(condData))
	if err != nil {
		log.Printf("GetConds: %s", err)
		os.Exit(-1)
	}

	log.Printf("conditions:\n%s", where.String())
}
