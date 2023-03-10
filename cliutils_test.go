// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package cliutils

import (
	"encoding/base64"
	"log"
	"testing"
)

func TestEncrypt(t *testing.T) {
	phrase := []byte("Pa55W0rd")
	data := []byte("data to be encrypted")

	endata, err := Encrypt(data, phrase)

	log.Printf("[debug] base64(endata): %s, err: %v", base64.StdEncoding.EncodeToString(endata), err)

	deData, err := Decrypt(endata, phrase)
	log.Printf("[debug] dedata: %s, err: %v", string(deData), err)
}
