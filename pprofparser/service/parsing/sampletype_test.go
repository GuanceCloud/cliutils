// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

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
