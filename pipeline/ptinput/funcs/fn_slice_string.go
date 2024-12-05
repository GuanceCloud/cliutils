package funcs

import (
	_ "embed"

	"math"

	"github.com/GuanceCloud/platypus/pkg/ast"
	"github.com/GuanceCloud/platypus/pkg/engine/runtime"
	"github.com/GuanceCloud/platypus/pkg/errchain"
)

// embed docs.
var (
	//go:embed md/slice_string.md
	docSliceString string

	//go:embed md/slice_string.en.md
	docSliceStringEN string

	// todo: parse function definition
	_ = "fn slice_string(name: str, start: int, end: int) -> str"

	FnSliceString = NewFunc(
		"slice_string",
		[]*Param{
			{
				Name: "name",
				Type: []ast.DType{ast.String},
			},
			{
				Name: "start",
				Type: []ast.DType{ast.Int},
			},
			{
				Name:     "end",
				Type:     []ast.DType{ast.Int},
				Optional: true,
				DefaultVal: func() (any, ast.DType) {
					return int64(math.MaxInt64), ast.Int
				},
			},
			{
				Name:     "step",
				Type:     []ast.DType{ast.Int},
				Optional: true,
				DefaultVal: func() (any, ast.DType) {
					return int64(1), ast.Int
				},
			},
		},
		[]ast.DType{ast.String},
		[2]*PLDoc{
			{
				Language: langTagZhCN, Doc: docSliceString,
				FnCategory: map[string][]string{
					langTagZhCN: {cStringOp}},
			},
			{
				Language: langTagEnUS, Doc: docSliceStringEN,
				FnCategory: map[string][]string{
					langTagEnUS: {eStringOp}},
			},
		},
		sliceString,
	)
)

func sliceString(ctx *runtime.Task, funcExpr *ast.CallExpr, vals ...any) *errchain.PlError {
	if len(vals) < 2 || len(vals) > 4 {
		ctx.Regs.ReturnAppend("", ast.String)
		return nil
	}
	name := vals[0].(string)
	length := int64(len(name))
	start, ok := vals[1].(int64)
	if !ok {
		ctx.Regs.ReturnAppend("", ast.String)
		return nil
	}
	end, ok := vals[2].(int64)
	if !ok {
		ctx.Regs.ReturnAppend("", ast.String)
		return nil
	}
	step, ok := vals[3].(int64)
	if !ok || step == 0 {
		ctx.Regs.ReturnAppend("", ast.String)
		return nil
	}

	if start < 0 {
		start = int64(len(name)) + start
	}
	if end < 0 {
		end = int64(len(name)) + end
	}

	substring := ""
	if step > 0 {
		if start < 0 {
			start = 0
		}
		for i := start; i < length && i < end; i += step {
			substring += string(name[i])
		}
		ctx.Regs.ReturnAppend(substring, ast.String)
		return nil
	} else {
		if start > length-1 {
			start = length - 1
		}
		for i := start; i > end && i >= 0; i += step {
			substring += string(name[i])
		}
		ctx.Regs.ReturnAppend(substring, ast.String)
		return nil
	}
}
