// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package network

import (
	"log"
	"testing"
)

func TestParseListen(t *testing.T) {
	ip, port, err := ParseListen(":48080")
	log.Println(ip, port, err)
}
