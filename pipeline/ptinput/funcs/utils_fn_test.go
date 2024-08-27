// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package funcs

import (
	"fmt"
	"testing"
	"time"

	"github.com/GuanceCloud/cliutils/pipeline/ptinput"
	"github.com/GuanceCloud/cliutils/point"
	tu "github.com/GuanceCloud/cliutils/testutil"
	"github.com/GuanceCloud/platypus/pkg/ast"
	"github.com/GuanceCloud/platypus/pkg/engine"
	"github.com/GuanceCloud/platypus/pkg/engine/runtime"
	"github.com/GuanceCloud/platypus/pkg/errchain"
	"github.com/GuanceCloud/platypus/pkg/token"
	"github.com/stretchr/testify/assert"
)

func TestNewFn(t *testing.T) {
	t.Run("new_check_p", func(t *testing.T) {
		err := panicWrap(func() {
			NewFunc("check_p", nil, nil, [2]*PLDoc{{}, {}}, nil)
		})
		assert.NoError(t, err)
	})

	t.Run("new_check_n", func(t *testing.T) {
		err := panicWrap(func() {
			NewFunc("check_n", []*Param{
				{Name: "name", Type: []ast.DType{ast.String}},
				{Name: "age", Type: []ast.DType{ast.Int},
					Optional: true, DefaultVal: func() (any, ast.DType) { return int64(0), ast.Int }},
				{Name: "tags", Type: []ast.DType{ast.String}, VariableP: true,
					DefaultVal: VarPDefaultVal},
			}, nil, [2]*PLDoc{nil, nil}, nil)
		})
		assert.NoError(t, err)
	})

	t.Run("new_check_r", func(t *testing.T) {
		err := panicWrap(func() {
			NewFunc("check_r", []*Param{
				{Name: "name", Type: []ast.DType{ast.String}},
				{Name: "age", Type: []ast.DType{ast.Int},
					Optional: true, DefaultVal: func() (any, ast.DType) { return int64(0), ast.Int }},
				{Name: "fields", Type: []ast.DType{ast.String, ast.Int, ast.Bool, ast.Float, ast.List, ast.Map, ast.Nil}, VariableP: true,
					DefaultVal: func() (any, ast.DType) {
						return []any{
							float64(1), int64(1), true, "abc", []any{"a"}, map[string]any{"a": 1}, nil,
						}, ast.List
					}},
			}, nil, [2]*PLDoc{nil, nil}, nil)
		})
		assert.NoError(t, err)
	})

	t.Run("new_check_err_r", func(t *testing.T) {
		err := panicWrap(func() {
			NewFunc("check_err_r", []*Param{
				{Name: "name", Type: []ast.DType{ast.String}},
				{Name: "age", Type: []ast.DType{ast.Int},
					Optional: true, DefaultVal: func() (any, ast.DType) { return int64(0), ast.Int }},
				{Name: "fields", Type: []ast.DType{ast.String, ast.Int, ast.Bool, ast.Float, ast.List, ast.Map}, VariableP: true,
					DefaultVal: func() (any, ast.DType) {
						return []any{
							float64(1), int64(1), true, "abc", []any{"a"}, map[string]any{"a": 1},
							[]byte{},
						}, ast.List
					}},
			}, nil, [2]*PLDoc{nil, nil}, nil)
		})
		assert.Error(t, err)
		if err != nil {
			assert.Equal(t, "parameter fields: default value data type not match", err.Error())
		}
	})

	t.Run("new_check_err_0", func(t *testing.T) {
		err := panicWrap(func() {
			NewFunc("check_n", []*Param{
				{Name: "name", Type: []ast.DType{ast.String},
					Optional: true},
				{Name: "age", Type: []ast.DType{ast.Int},
					Optional: true, DefaultVal: func() (any, ast.DType) { return int64(0), ast.Int }},
				{Name: "tags", Type: []ast.DType{ast.String}, VariableP: true,
					DefaultVal: VarPDefaultVal},
			}, nil, [2]*PLDoc{nil, nil}, nil)
		})
		assert.Error(t, err)
		if err != nil {
			assert.Equal(t, "parameter name: optional parameter should have default value", err.Error())
		}
	})

	t.Run("new_check_err_1", func(t *testing.T) {
		err := panicWrap(func() {
			NewFunc("check_n", []*Param{
				{Name: "name", Type: []ast.DType{ast.String}},
				{Name: "age", Type: []ast.DType{ast.Int},
					Optional: true, DefaultVal: func() (any, ast.DType) { return int64(0), ast.Int }},
				{Name: "tags", Type: []ast.DType{ast.String}, VariableP: true},
				{Name: "tags", Type: []ast.DType{ast.String}, VariableP: true},
			}, nil, [2]*PLDoc{nil, nil}, nil)
		})
		assert.Error(t, err)
		if err != nil {
			assert.Equal(t, "parameter tags: variable parameter should be the last one", err.Error())
		}
	})

	t.Run("new_check_err_1", func(t *testing.T) {
		err := panicWrap(func() {
			NewFunc("check_n", []*Param{
				{Name: "name", Type: []ast.DType{ast.String}},
				{Name: "age", Type: []ast.DType{ast.Int},
					Optional: true, DefaultVal: func() (any, ast.DType) { return int64(0), ast.Int }},
				{Name: "age", Type: []ast.DType{ast.String}},

				{Name: "tags", Type: []ast.DType{ast.String}, VariableP: true},
			}, nil, [2]*PLDoc{nil, nil}, nil)
		})
		assert.Error(t, err)
		if err != nil {
			assert.Equal(t, "duplicate parameter name: age", err.Error())
		}
	})

	t.Run("new_check_err_2", func(t *testing.T) {
		err := panicWrap(func() {
			NewFunc("check_n", []*Param{
				{Name: "name", Type: []ast.DType{ast.String}},
				{Name: "age", Type: []ast.DType{ast.Int},
					Optional: true, DefaultVal: func() (any, ast.DType) { return int64(0), ast.Int }},
				{Name: "opt", Type: []ast.DType{ast.String}},

				{Name: "tags", Type: []ast.DType{ast.String}, VariableP: true},
			}, nil, [2]*PLDoc{nil, nil}, nil)
		})
		assert.Error(t, err)
		if err != nil {
			assert.Equal(t, "parameter opt: required parameter should not follow optional parameter", err.Error())
		}
	})

	t.Run("new_check_err_3", func(t *testing.T) {
		err := panicWrap(func() {
			NewFunc("check_n", []*Param{
				{Name: "name", Type: []ast.DType{ast.String}},
				{Name: "age", Type: []ast.DType{ast.Int},
					Optional: true, DefaultVal: func() (any, ast.DType) { return 0, ast.Int }},
				{Name: "tags", Type: []ast.DType{ast.String}, VariableP: true},
			}, nil, [2]*PLDoc{nil, nil}, nil)
		})
		assert.Error(t, err)
		if err != nil {
			assert.Equal(t, "parameter age: value type not match", err.Error())
		}
	})

	t.Run("new_check_err_4", func(t *testing.T) {
		err := panicWrap(func() {
			NewFunc("check_n", []*Param{
				{Name: "name", Type: []ast.DType{ast.String}},
				{Name: "age", Type: []ast.DType{ast.Int},
					Optional: true, DefaultVal: func() (any, ast.DType) { return float64(1.1), ast.Float }},
				{Name: "tags", Type: []ast.DType{ast.String}, VariableP: true},
			}, nil, [2]*PLDoc{nil, nil}, nil)
		})
		assert.Error(t, err)
		if err != nil {
			assert.Equal(t, "parameter age: default value data type not match", err.Error())
		}
	})
}

func TestRunFunc(t *testing.T) {
	fnLi1 := []*Function{
		NewFunc("trigger", []*Param{
			{Name: "msg", Type: []ast.DType{ast.String}},
			{Name: "args", Type: []ast.DType{ast.Bool, ast.Int, ast.Float, ast.String,
				ast.List, ast.Map, ast.Nil}, VariableP: true, DefaultVal: VarPDefaultVal},
		}, nil, [2]*PLDoc{nil, nil}, func(ctx *runtime.Task, funcExpr *ast.CallExpr, vals ...any) *errchain.PlError {
			var msg string
			switch vals[0].(type) {
			case string:
				msg = vals[0].(string)
			default:
				var pos token.LnColPos
				if funcExpr.Param[0] != nil {
					pos = funcExpr.Param[0].StartPos()
				} else {
					pos = funcExpr.NamePos
				}
				return runtime.NewRunError(ctx, "unexpected type", pos)
			}
			var varP []any
			switch v := vals[1].(type) {
			case []any:
				varP = v
			case nil:
			default:
				return runtime.NewRunError(ctx, "unexpected type", funcExpr.Param[1].StartPos())
			}

			s := fmt.Sprintf(msg, varP...)
			_ = addKey2PtWithVal(ctx.InData(), "msg", s, ast.String, ptinput.KindPtDefault)

			return nil
		}),
	}

	fnLi2 := []*Function{
		NewFunc("trigger2", []*Param{
			{Name: "msg", Type: []ast.DType{ast.String}, Optional: true, DefaultVal: func() (any, ast.DType) {
				return "test", ast.String
			}},
			{Name: "args", Type: []ast.DType{ast.Bool, ast.Int, ast.Float, ast.String,
				ast.List, ast.Map, ast.Nil}, VariableP: true, DefaultVal: VarPDefaultVal},
		}, nil, [2]*PLDoc{nil, nil}, func(ctx *runtime.Task, funcExpr *ast.CallExpr, vals ...any) *errchain.PlError {
			var msg string
			switch vals[0].(type) {
			case string:
				msg = vals[0].(string)
			default:
				var pos token.LnColPos
				if funcExpr.Param[0] != nil {
					pos = funcExpr.Param[0].StartPos()
				} else {
					pos = funcExpr.NamePos
				}
				return runtime.NewRunError(ctx, "unexpected type", pos)
			}
			var varP []any
			switch v := vals[1].(type) {
			case []any:
				varP = v
			case nil:
			default:
				return runtime.NewRunError(ctx, "unexpected type", funcExpr.Param[1].StartPos())
			}

			s := fmt.Sprintf(msg, varP...)
			_ = addKey2PtWithVal(ctx.InData(), "msg", s, ast.String, ptinput.KindPtDefault)

			return nil
		}),
	}

	cases := []struct {
		name     string
		pl, in   string
		outKey   string
		expected string
		funcs    []*Function
		fail     bool
	}{
		{
			name: "pos_varp",
			pl: `
a = 1
b = "aaa"
c = 1.1
x = trigger("%d %s %.3f", a, b, c)
`,
			outKey:   "msg",
			funcs:    fnLi1,
			expected: "1 aaa 1.100",
			fail:     false,
		},
		{
			name: "err",
			pl: `
a = 1
b = "aaa"
c = 1.1
x = trigger()
`,
			outKey:   "msg",
			funcs:    fnLi1,
			expected: "1 aaa 1.100",
			fail:     true,
		},
		{
			name: "pos_varp_1",
			pl: `
x = trigger("%d %v %v %v %.1f", 1, true, [], {}, 1.1)
`,
			outKey:   "msg",
			funcs:    fnLi1,
			expected: "1 true [] map[] 1.1",
			fail:     false,
		},
		{
			name: "named",
			pl: `
a = 1
b = "aaa"
c = 1.1
x = trigger("%d %s %.3f", args = [a, b, c])
`,
			outKey:   "msg",
			funcs:    fnLi1,
			expected: "1 aaa 1.100",
			fail:     false,
		},
		{
			name: "named1",
			pl: `
a = 1
b = "aaa"
c = 1.1
x = trigger(args = [a, b, c], msg="%d %s %.3f")
`,
			outKey:   "msg",
			funcs:    fnLi1,
			expected: "1 aaa 1.100",
			fail:     false,
		},
		{
			name: "named2",
			pl: `
x = trigger("abc")
`,
			outKey:   "msg",
			funcs:    fnLi1,
			expected: "abc",
			fail:     false,
		},

		{
			name: "p3",
			pl: `
a = 1
b = "aaa"
c = 1.1
x = trigger("%d %s %.3f %v", args = [a, b, c, nil])
`,
			outKey:   "msg",
			funcs:    fnLi1,
			expected: "1 aaa 1.100 <nil>",
			fail:     false,
		},
		{
			name: "fn2",
			pl: `
x = trigger2()
`,
			outKey:   "msg",
			funcs:    fnLi2,
			expected: "test",
			fail:     false,
		},
		{
			name: "fn2-1",
			pl: `
x = trigger2("%d", 1)
`,
			outKey:   "msg",
			funcs:    fnLi2,
			expected: "1",
			fail:     false,
		},
		{
			name: "fn2-1-1",
			pl: `
x = trigger2("%d %s", 1, "a")
`,
			outKey:   "msg",
			funcs:    fnLi2,
			expected: "1 a",
			fail:     false,
		},
		{
			name: "fn2-2",
			pl: `
x = trigger2(args=[1, "a"], msg="%d %s")
`,
			outKey:   "msg",
			funcs:    fnLi2,
			expected: "1 a",
			fail:     false,
		},
		{
			name: "fn2-2-1",
			pl: `
x = trigger2( msg="%d %s", args=[1, "a"])
`,
			outKey:   "msg",
			funcs:    fnLi2,
			expected: "1 a",
			fail:     false,
		},
		{
			name: "fn2-2-2",
			pl: `
x = trigger2( msg="aaa")
`,
			outKey:   "msg",
			funcs:    fnLi2,
			expected: "aaa",
			fail:     false,
		},
		{
			name: "fn-run-failed",
			pl: `
x = trigger2( msg="aaa", args = "")
`,
			outKey:   "msg",
			funcs:    fnLi2,
			expected: "aaa",
			fail:     true,
		},
	}

	for idx, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			script, err := parseScipt(tc.pl, tc.funcs)
			if err != nil {
				if tc.fail {
					t.Logf("[%d]expect error: %s", idx, err)
				} else {
					t.Errorf("[%d] failed: %s", idx, err)
				}
				return
			}

			pt := ptinput.NewPlPoint(
				point.Logging, "test", nil, map[string]any{"message": tc.in}, time.Now())

			errR := script.Run(pt, nil)
			if errR != nil {
				t.Fatal(errR.Error())
			}

			if v, _, err := pt.Get(tc.outKey); err != nil {
				if !tc.fail {
					t.Errorf("[%d]expect error: %s", idx, err)
				}
			} else {
				tu.Equals(t, tc.expected, v)
				t.Logf("[%d] PASS", idx)
			}
		})
	}
}

func panicWrap(f func()) (err error) {
	defer func() {
		if e := recover(); e != nil {
			err = fmt.Errorf("%v", e)
		}
	}()
	f()
	return
}

func parseScipt(s string, funcs []*Function) (*runtime.Script, error) {
	fn := map[string]runtime.FuncCall{}
	fnCheck := map[string]runtime.FuncCheck{}

	for _, f := range funcs {
		fn[f.Name] = f.Call
		fnCheck[f.Name] = f.Check
	}

	ret1, ret2 := engine.ParseScript(map[string]string{
		"default.p": s,
	},
		fn, fnCheck,
	)

	if len(ret1) > 0 {
		return ret1["default.p"], nil
	}

	if len(ret2) > 0 {
		return nil, ret2["default.p"]
	}

	return nil, fmt.Errorf("parser func error")
}
