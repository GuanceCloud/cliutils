// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package astlint

import (
	"fmt"
	"go/ast"
	"go/token"
	"log"
)

type Issue struct {
	text string
	pos  *token.Position
}

type visitor struct {
	fs     *token.FileSet
	issues []*Issue
}

func (v *visitor) Visit(n ast.Node) ast.Visitor {
	if n == nil {
		return v
	}

	switch t := n.(type) {
	case *ast.CallExpr:
		return v.parseCallerExpr(t)

	case *ast.File:
		//log.Printf("current package name: %s", t.Name)

	default:
	}

	return v
}

func (v *visitor) parseCallerExpr(call *ast.CallExpr) ast.Visitor {

	switch stmt := call.Fun.(type) {
	case *ast.Ident:
		//log.Printf("get Ident stmt %s", stmt.String())

	case *ast.SelectorExpr:
		//log.Printf("get SelectorExpr stmt %v", stmt)

		//if ident, ok := stmt.X.(*ast.Ident); ok && ident.Obj != nil && ident.Obj.Kind == ast.Var {
		//	log.Printf("get SelectorExpr stmt.Sel: %+#v", stmt.Sel)
		//	log.Printf("get SelectorExpr stmt.X.Obj: %+#v", ident.Obj)
		//}

		switch stmt.Sel.Name {
		case "Info":
		case "Debug":
		case "Debugf":

		case "Infof", "Warnf":

			if len(call.Args) > 1 {
				if fmtStr, ok := call.Args[0].(*ast.BasicLit); ok && fmtStr.Kind == token.STRING && len(fmtStr.Value) < 16 {
					pos := v.fs.Position(call.Pos())
					v.issues = append(v.issues, &Issue{
						text: fmt.Sprintf("too short log message: %s", fmtStr.Value),
						pos:  &pos,
					})
				}
			}

		case "DefaultSLogger":
		case "SLogger":
		}
	default: // pass
	}

	return v
}

type opt struct {
	files []*ast.File

	withVendor bool
	withTest   bool
}

type Option func(*opt) error

func WithFiles(files ...*ast.File) Option {
	return func(o *opt) error {
		o.files = append(o.files, files...)
		return nil
	}
}

func WithVendor(on bool) Option {
	return func(o *opt) error {
		o.withVendor = on
		return nil
	}
}

func WithTest(on bool) Option {
	return func(o *opt) error {
		o.withTest = on
		return nil
	}
}

func lintFile(fs *token.FileSet, opts ...Option) []*Issue {
	v := &visitor{
		fs: fs,
	}

	o := &opt{}
	for _, opt := range opts {
		if err := opt(o); err != nil {
			return nil
		}
	}

	for _, f := range o.files {
		ast.Walk(v, f)
	}

	return v.issues
}

func init() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
}
