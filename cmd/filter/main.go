package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/GuanceCloud/cliutils/filter"
)

var (
	conditionFile string
)

// nolint: gochecknoinits
func init() {
	flag.StringVar(&conditionFile, "condition-path", "", "condition file")
}

func main() {
	flag.Parse()

	condData, err := os.ReadFile(filepath.Clean(conditionFile))
	if err != nil {
		log.Printf("ReadFile: %s", err)
		os.Exit(-1)
	}

	where, err := filter.GetConds(string(condData))
	if err != nil {
		log.Printf("GetConds: %s", err)
		os.Exit(-1)
	}

	fmt.Printf("Parse %d conditions ok\n", len(where))
}
