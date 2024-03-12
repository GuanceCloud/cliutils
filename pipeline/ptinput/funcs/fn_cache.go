package funcs

import (
	"fmt"

	"github.com/GuanceCloud/platypus/pkg/ast"
	"github.com/GuanceCloud/platypus/pkg/engine/runtime"
	"github.com/GuanceCloud/platypus/pkg/errchain"
)

func CacheCreateChecking(ctx *runtime.Context, funcExpr *ast.CallExpr) *errchain.PlError {
	if len(funcExpr.Param) != 0 {
		return runtime.NewRunError(ctx, fmt.Sprintf(
			"func %s expects 0 arg", funcExpr.Name), funcExpr.NamePos)
	}
	return nil
}
func CacheCreate(ctx *runtime.Context, funcExpr *ast.CallExpr) *errchain.PlError {
	pt, err := getPoint(ctx.InData())
	if err != nil {
		return nil
	}

	pt.CreateCache()

	return nil
}
func CacheGetChecking(ctx *runtime.Context, funcExpr *ast.CallExpr) *errchain.PlError {
	if len(funcExpr.Param) != 1 {
		return runtime.NewRunError(ctx, fmt.Sprintf(
			"func %s expects 1 arg", funcExpr.Name), funcExpr.NamePos)
	}
	return nil
}

func CacheGet(ctx *runtime.Context, funcExpr *ast.CallExpr) *errchain.PlError {
	val, dtype, err := runtime.RunStmt(ctx, funcExpr.Param[0])
	if err != nil {
		return err
	}

	if dtype != ast.String {
		return runtime.NewRunError(ctx, "param data type expect string",
			funcExpr.Param[0].StartPos())
	}

}
