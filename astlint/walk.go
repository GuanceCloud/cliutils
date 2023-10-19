// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package astlint

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

func findFiles(paths []string, fileSet *token.FileSet, opts ...Option) (files []*ast.File, err error) {

	o := &opt{}
	for _, x := range opts {
		x(o)
	}

	for _, path := range paths {
		if _, err = os.Stat(path); os.IsNotExist(err) {
			return
		}

		for f := range walkDir(path, o) {
			if x, err := parser.ParseFile(fileSet, f, nil, parser.AllErrors); err != nil {
				return nil, fmt.Errorf("parser.ParseFile(%q): %s", f, err)
			} else {
				files = append(files, x)
			}
		}
	}

	return
}

func walkDir(root string, o *opt) chan string {
	out := make(chan string)

	go func() {
		defer close(out)
		err := filepath.Walk(root,
			func(p string, info os.FileInfo, err error) error {
				sep := string(filepath.Separator)

				if !o.withVendor && strings.Contains(p, "vendor"+sep) {
					return nil
				}

				if !o.withTest && strings.Contains(info.Name(), "_test.go") {
					return nil
				}

				if info.IsDir() {
					return nil
				}

				if !strings.HasSuffix(p, ".go") {
					return nil
				}

				out <- p
				return nil
			})

		if err != nil {
			return
		}
	}()
	return out
}
