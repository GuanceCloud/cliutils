// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package parsing

import (
	"fmt"
	"io"
	"os"
	"testing"

	"github.com/GuanceCloud/cliutils/testutil"
	"github.com/google/pprof/profile"
)

func TestNewDecompressor(t *testing.T) {
	f, err := os.Open("testdata/auto.pprof.lz4")

	testutil.Ok(t, err)

	rc := NewDecompressor(f)

	prof, err := profile.Parse(rc)

	testutil.Ok(t, err)

	fmt.Println(len(prof.Sample))

	buf := make([]byte, 1)
	_, err = f.Read(buf)
	testutil.Equals(t, io.EOF, err)

	err = rc.Close()
	testutil.Ok(t, err)

	_, err = f.Read(buf)
	fmt.Println(err)
	testutil.NotOk(t, err, "expect err, got nil")
}
