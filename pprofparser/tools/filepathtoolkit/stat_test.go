// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package filepathtoolkit

import "testing"

func TestFileExists(t *testing.T) {
	t.Log(FileExists("/sbin/shutdown"))
	t.Log(FileExists("/etc/tmpfile"))
	t.Log(FileExists("/root"))
}
