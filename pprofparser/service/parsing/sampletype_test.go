package parsing

import (
	"fmt"
	"testing"
)

func TestGetFileByEvent(t *testing.T) {
	events := generateDictByEvent(pprofTypeMaps)

	for lang, m := range events {
		fmt.Println(lang)

		for e, file := range m {
			fmt.Println("\t", e, ":", file)
		}

		fmt.Println("--------------------------------")
	}
}
