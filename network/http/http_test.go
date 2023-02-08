// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package http

import (

	// nolint:gosec
	"crypto/md5"
	"fmt"
	"log"
	"net/http"
	"testing"
	"time"
)

func TestSign(t *testing.T) {
	o := &SignOption{
		AuthorizationType: "ABC",
		SignHeaders:       []string{`Date`, `Content-MD5`, `Content-Type`},
		SK:                "sk-cba",
		AK:                "ak-123",
		Sign:              "",
	}

	r, err := http.NewRequest(`POST`, `http://1.2.3.4:1234/v1/api`, http.NoBody)
	if err != nil {
		t.Fatal(err)
	}

	r.Header.Set("Date", time.Now().UTC().Format(http.TimeFormat))
	r.Header.Set(`Content-MD5`, fmt.Sprintf("%x", md5.Sum([]byte(`{}`)))) //nolint:gosec
	r.Header.Set(`Content-Type`, `application/json`)

	o.Sign, err = o.SignReq(r)
	if err != nil {
		t.Fatal(err)
	}

	r.Header.Set(`Authorization`, fmt.Sprintf("%s %s:%s", o.AuthorizationType, o.AK, o.Sign))
	log.Printf("[debug] %+#v", o)

	o2 := &SignOption{
		AuthorizationType: "ABC",
		SignHeaders:       []string{`Date`, `Content-MD5`, `Content-Type`},
		SK:                "sk-cba",
		AK:                "ak-123",
	}

	if err := o2.ParseAuth(r); err != nil {
		t.Fatal(err)
	}

	if sign, err := o2.SignReq(r); err != nil || o2.Sign != sign {
		t.Fatalf("sign failed: %v, %s <> %s", err, o2.Sign, sign)
	}
	log.Printf("[debug] %+#v", o)
}
