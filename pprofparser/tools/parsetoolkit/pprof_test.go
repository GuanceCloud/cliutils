// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package parsetoolkit

import (
	"fmt"
	"testing"
)

func TestFormatDuration(t *testing.T) {
	s := FormatDuration(100_000_000_123)
	fmt.Println(s)
}
