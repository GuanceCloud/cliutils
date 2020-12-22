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
	DefaultNamespace                 = ""
	ErrUnexpectedInternalServerError = NewErr(errors.New(`unexpected internal server error`), nhttp.StatusInternalServerError)
	ErrBadAuthHeader                 = NewErr(errors.New("invalid http Authorization header"), nhttp.StatusForbidden)
	ErrAuthFailed                    = NewErr(errors.New("http Authorization failed"), nhttp.StatusForbidden)
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

func NewNamespaceErr(err error, httpCode int, namespace string) *HttpError {
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

func NewErr(err error, httpCode int) *HttpError {
	return NewNamespaceErr(err, httpCode, DefaultNamespace)
}

func internalServerErr(err error) *HttpError {
	return NewErr(err, nhttp.StatusInternalServerError)
}

func (he *HttpError) Error() string {
	if he.Err == nil {
		return ""
	} else {
		return he.Err.Error()
	}
}

func (he *HttpError) HttpBody(c *gin.Context, body interface{}) {
	resp := &bodyResp{
		HttpError: he,
		Content:   body,
	}

	j, err := json.Marshal(resp)
	if err != nil {
		internalServerErr(err).httpResp(c, "%s: %+#v", "json.Marshal() failed", resp)
		return
	}

	c.Data(he.HttpCode, `application/json`, j)
}

func HttpErr(c *gin.Context, err error) {
	he, ok := err.(*HttpError)
	if ok {
		he.httpResp(c, "")
	} else { // undefined error code
		internalServerErr(err).httpResp(c, "")
	}
}

func HttpErrf(c *gin.Context, err error, format string, args ...interface{}) {
	he, ok := err.(*HttpError)
	if ok {
		he.httpResp(c, format, args...)
	} else {

		internalServerErr(err).httpResp(c, "")
		//he = NewErr(err, nhttp.StatusInternalServerError)
		//he.ErrCode = ""
		//he.httpResp(c, format, args...)
	}
}

func (he *HttpError) httpResp(c *gin.Context, format string, args ...interface{}) {
	resp := &bodyResp{
		HttpError: he,
	}

	if args != nil {
		resp.Message = fmt.Sprintf(format, args...)
	}

	j, err := json.Marshal(&resp)
	if err != nil {
		internalServerErr(err).httpResp(c, "%s: %+#v", "json.Marshal() failed", resp)
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
