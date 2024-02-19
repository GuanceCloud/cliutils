// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

// Package main used for GC escape:
//   $ go build -gcflags '-m -m' ./x.go
package main

import (
	"time"

	"github.com/GuanceCloud/cliutils/point"
)

func main() {
	pp := point.NewPointPoolLevel3()

	kvs := [][]any{
		//[]any{"f1", 123},
		//[]any{"f2", 1.23},
		[]any{"f3", "str"},
		//[]any{"f4", false},
		//[]any{"f5", []byte("some-binary-data")},
		//[]any{"tag1", "val-1", true},
	}

	pt := point.NewPointV3("abc", kvs, point.WithTime(time.Unix(0, 123)), point.WithPointPool(pp))
	_ = pt

	mainStr := "123"
	strFunc(mainStr)
}

func strFunc(s string) {
	x := s
	_ = x
}
