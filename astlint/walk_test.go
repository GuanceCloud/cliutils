// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package astlint

import (
	"go/token"
	T "testing"

	"github.com/stretchr/testify/assert"
)

func TestWalk(t *T.T) {
	t.Run(`basic`, func(t *T.T) {
		fs := &token.FileSet{}
		files, err := findFiles([]string{}, fs)
		assert.NoError(t, err)

		iss := lintFile(fs, WithFiles(files...))
		t.Logf("get %d issues", len(iss))
		for _, i := range iss {
			t.Logf("%s: %s", i.pos, i.text)
		}
	})
}
