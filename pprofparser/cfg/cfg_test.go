// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package cfg

import (
	"fmt"
	"testing"
)

func TestInitConfig(t *testing.T) {
	err := Load("testdata/conf.yml")
	if err != nil {
		t.Error(err)
	}
	fmt.Printf("%+#v\n", Cfg)
}
