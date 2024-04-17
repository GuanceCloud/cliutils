// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package funcs

import (
	"fmt"

	"github.com/GuanceCloud/cliutils/pipeline/ptinput"
	"github.com/GuanceCloud/platypus/pkg/ast"
	"github.com/GuanceCloud/platypus/pkg/engine/runtime"
	"github.com/GuanceCloud/platypus/pkg/errchain"
	"github.com/tidwall/gjson"
)

func GJSONChecking(ctx *runtime.Context, funcExpr *ast.CallExpr) *errchain.PlError {
	if err := reindexFuncArgs(funcExpr, []string{
		"input", "json_path", "key_name",
	}, 2); err != nil {
		return runtime.NewRunError(ctx, err.Error(), funcExpr.NamePos)
	}

	if _, err := getKeyName(funcExpr.Param[0]); err != nil {
		return runtime.NewRunError(ctx, err.Error(), funcExpr.Param[0].StartPos())
	}

	switch funcExpr.Param[1].NodeType { //nolint:exhaustive
	case ast.TypeAttrExpr, ast.TypeIdentifier, ast.TypeIndexExpr:
		var err error
		if err != nil {
			return runtime.NewRunError(ctx, err.Error(), funcExpr.Param[1].StartPos())
		}
	default:
		return runtime.NewRunError(ctx, fmt.Sprintf("expect AttrExpr, IndexExpr or Identifier, got %s",
			funcExpr.Param[1].NodeType), funcExpr.Param[1].StartPos())
	}

	if funcExpr.Param[2] != nil {
		switch funcExpr.Param[2].NodeType { //nolint:exhaustive
		case ast.TypeAttrExpr, ast.TypeIdentifier, ast.TypeStringLiteral:
		default:
			return runtime.NewRunError(ctx, fmt.Sprintf("expect AttrExpr or Identifier, got %s",
				funcExpr.Param[2].NodeType), funcExpr.Param[2].StartPos())
		}
	}

	return nil
}

func GJSON(ctx *runtime.Context, funcExpr *ast.CallExpr) *errchain.PlError {
	var jpath *ast.Node

	srcKey, err := getKeyName(funcExpr.Param[0])
	if err != nil {
		return runtime.NewRunError(ctx, err.Error(), funcExpr.Param[0].StartPos())
	}

	switch funcExpr.Param[1].NodeType {
	case ast.TypeAttrExpr, ast.TypeIdentifier, ast.TypeIndexExpr:
		jpath = funcExpr.Param[1]
	default:
		return runtime.NewRunError(ctx, fmt.Sprintf("expect AttrExpr or Identifier, got %s",
			funcExpr.Param[1].NodeType), funcExpr.Param[1].StartPos())
	}

	targetKey, _ := getKeyName(jpath)

	if funcExpr.Param[2] != nil {
		switch funcExpr.Param[2].NodeType { //nolint:exhaustive
		case ast.TypeAttrExpr, ast.TypeIdentifier, ast.TypeStringLiteral:
			targetKey, _ = getKeyName(funcExpr.Param[2])
		default:
			return runtime.NewRunError(ctx, fmt.Sprintf("expect AttrExpr or Identifier, got %s",
				funcExpr.Param[2].NodeType), funcExpr.Param[2].StartPos())
		}
	}

	cont, err := ctx.GetKeyConv2Str(srcKey)
	if err != nil {
		l.Debug(err)
		return nil
	}

	fmt.Println(jpath)
	res := gjson.Get(cont, jpath.String())
	rType := res.Type

	fmt.Println(res)

	var dtype ast.DType
	var v any
	switch rType {
	case 2:
		v = res.Float()
		dtype = ast.Float
	case 1, 4:
		v = res.Bool()
		dtype = ast.Bool
	case 3, 5:
		v = res.String()
		dtype = ast.String
	default:
		return nil
	}

	if err := addKey2PtWithVal(ctx.InData(), targetKey, v, dtype, ptinput.KindPtDefault); err != nil {
		l.Debug(err)
		return nil
	}

	return nil
}
