package funcs

import (
	"fmt"

	"github.com/GuanceCloud/platypus/pkg/ast"
	"github.com/GuanceCloud/platypus/pkg/engine/runtime"
	"github.com/GuanceCloud/platypus/pkg/errchain"
)

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

	pt, errP := getPoint(ctx.InData())
	if errP != nil {
		ctx.Regs.ReturnAppend(nil, ast.Nil)
		return nil
	}

	c := pt.GetCache()
	v, exist, errG := c.Get(val.(string))
	if !exist || errG != nil {
		ctx.Regs.ReturnAppend(nil, ast.Nil)
		return nil
	}

	ctx.Regs.ReturnAppend(v, ast.String)

	return nil
}
