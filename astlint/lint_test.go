package astlint

import (
	"go/parser"
	"go/token"
	T "testing"

	"github.com/stretchr/testify/assert"
)

func TestLint(t *T.T) {
	fs := token.NewFileSet()

	f := "./testdata/logger.go"
	//f := "/Users/tanbiao/go/src/gitlab.jiagouyun.com/cloudcare-tools/datakit/cmd/datakit/main.go"

	file, err := parser.ParseFile(fs, f, nil, parser.AllErrors)
	assert.NoError(t, err)

	iss := lintFile(fs, WithFiles(file))
	t.Logf("get %d issues", len(iss))
	for _, i := range iss {
		t.Logf("%s: %s", i.pos, i.text)
	}
}
