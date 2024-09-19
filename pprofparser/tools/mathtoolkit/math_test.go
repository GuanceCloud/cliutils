// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package mathtoolkit

import (
	"fmt"
	"testing"
)

func TestTrunc(t *testing.T) {
	if Trunc(3.1415926) != 3 {
		t.Fatal("trunc result wrong")
	}

	fmt.Println(Trunc(3))
	fmt.Println(Trunc(5.9999999999))
	fmt.Println(Trunc(-3.00001))
	fmt.Println(Trunc(0))
	fmt.Println(Trunc(3.00000002))
}
