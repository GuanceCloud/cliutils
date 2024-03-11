package funcs

import (
	"io"
	"net/http"

	"github.com/GuanceCloud/platypus/pkg/ast"
	"github.com/GuanceCloud/platypus/pkg/engine/runtime"
	"github.com/GuanceCloud/platypus/pkg/errchain"
)

func HTTPRequestChecking(ctx *runtime.Context, funcExpr *ast.CallExpr) *errchain.PlError {
	if err := reindexFuncArgs(funcExpr, []string{
		"method", "url", "headers",
	}, 2); err != nil {
		return runtime.NewRunError(ctx, err.Error(), funcExpr.NamePos)
	}

	return nil
}

/*
params:

	method 		string (required)
	url	   		string (required)
	headers 	map[string]string
*/
func HTTPRequest(ctx *runtime.Context, funcExpr *ast.CallExpr) *errchain.PlError {
	method, methodType, err := runtime.RunStmt(ctx, funcExpr.Param[0])
	if err != nil {
		return err
	}
	if methodType != ast.String {
		return runtime.NewRunError(ctx, "param data type expect string",
			funcExpr.Param[0].StartPos())
	}
	url, urlType, err := runtime.RunStmt(ctx, funcExpr.Param[1])
	if err != nil {
		return err
	}
	if urlType != ast.String {
		return runtime.NewRunError(ctx, "param data type expect string",
			funcExpr.Param[1].StartPos())
	}
	headers, headersType, err := runtime.RunStmt(ctx, funcExpr.Param[2])
	if err != nil {
		return err
	}
	if headersType != ast.Map {
		return runtime.NewRunError(ctx, "param data type expect map",
			funcExpr.Param[2].StartPos())
	}

	client := &http.Client{}
	req, errR := http.NewRequest(method.(string), url.(string), nil)
	if errR != nil {
		ctx.Regs.ReturnAppend(nil, ast.Nil)
		return nil
	}
	for k, v := range headers.(map[string]string) {
		req.Header.Set(k, v)
	}

	resp, errR := client.Do(req)
	if errR != nil {
		ctx.Regs.ReturnAppend(nil, ast.Nil)
		return nil
	}

	defer resp.Body.Close()

	body, errR := io.ReadAll(resp.Body)
	if errR != nil {
		ctx.Regs.ReturnAppend(nil, ast.Nil)
		return nil
	}

	respData := map[string]interface{}{
		"status_code": resp.StatusCode,
		"body":        body,
	}
	ctx.Regs.ReturnAppend(respData, ast.Map)

	return nil
}
