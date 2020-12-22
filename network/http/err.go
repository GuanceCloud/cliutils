//nolint:golint,stylecheck
package http

import (
	"encoding/json"
	"errors"
	"fmt"
	nhttp "net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

var (
	ErrUnexpectedInternalServerError = NewErr(errors.New(`unexpected internal server error`), nhttp.StatusInternalServerError, "")
	ErrBadAuthHeader                 = NewErr(errors.New("invalid http Authorization header"), nhttp.StatusForbidden, "")
	ErrAuthFailed                    = NewErr(errors.New("http Authorization failed"), nhttp.StatusForbidden, "")
)

type HttpError struct {
	ErrCode  string `json:"error_code,omitempty"`
	Err      error  `json:"-"`
	HttpCode int    `json:"-"`
}

type bodyResp struct {
	*HttpError
	Message string      `json:"message,omitempty"`
	Content interface{} `json:"content,omitempty"`
}

func NewErr(err error, httpCode int, namespace string) *HttpError {
	if err == nil {
		return &HttpError{
			HttpCode: httpCode,
		}
	} else {
		return &HttpError{
			ErrCode:  titleErr(namespace, err),
			HttpCode: httpCode,
			Err:      err,
		}
	}
}

func (he *HttpError) HttpBody(c *gin.Context, body interface{}) {
	resp := &bodyResp{
		HttpError: he,
		Content:   body,
	}

	j, err := json.Marshal(resp)
	if err != nil {
		ErrUnexpectedInternalServerError.httpResp(c, err)
		return
	}

	c.Data(he.HttpCode, `application/json`, j)
}

func HttpErr(c *gin.Context, err error, msg string) {
	he, ok := err.(*HttpError)
	if ok {
		he.httpResp(c, msg)
	} else {
		he = NewErr(err, nhttp.StatusInternalServerError, "")
		he.ErrCode = ""
		he.httpResp(c, msg)
	}
}

func (he *HttpError) Error() string {
	if he.Err == nil {
		return ""
	} else {
		return he.Err.Error()
	}
}

func (he *HttpError) httpResp(c *gin.Context, args ...interface{}) {
	resp := &bodyResp{
		HttpError: he,
	}

	if args != nil {
		resp.Message = fmt.Sprint(args...)
	}

	j, err := json.Marshal(&resp)
	if err != nil {
		ErrUnexpectedInternalServerError.httpResp(c, err)
		return
	}

	c.Data(he.HttpCode, `application/json`, j)
}

func titleErr(namespace string, err error) string {
	if err == nil {
		return ""
	}

	str := err.Error()
	elem := strings.Split(str, ` `)

	var out string
	if namespace != "" {
		out = namespace + `.`
	}

	for idx, e := range elem {
		if idx == 0 {
			out += e
			continue
		}
		out += strings.Title(e)
	}

	return out
}
