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
	ErrUnexpectedInternalServerError = NewErr(errors.New(`unexpected internal server error`), nhttp.StatusInternalServerError, ``)
)

type HttpError struct {
	ErrCode  string      `json:"errorCode,omitempty"`
	Err      error       `json:"-"`
	HttpCode int         `json:"code,omitempty"`
	Message  string      `json:"message,omitempty"`
	Content  interface{} `json:"content,omitempty"`
}

func NewErr(err error, httpCode int, namespace string) *HttpError {
	return &HttpError{
		ErrCode:  titleErr(namespace, err),
		HttpCode: httpCode,
		Err:      err,
	}
}

func (he *HttpError) Error() string {
	if he.Err == nil {
		return ""
	} else {
		return fmt.Sprintf("%s", he.Err.Error())
	}
}

func (he *HttpError) Json(args ...interface{}) ([]byte, error) {

	obj := map[string]interface{}{
		"code":      he.HttpCode,
		"errorCode": he.ErrCode,
	}

	if args == nil {
		obj[`message`] = he.Error()
	} else {
		obj[`message`] = fmt.Sprint(he.Error(), args)
	}

	j, err := json.Marshal(&obj)
	if err != nil {
		return nil, err
	}

	return j, nil
}

func (he *HttpError) HttpBody(c *gin.Context, body interface{}) {
	obj := map[string]interface{}{
		"code":      he.HttpCode,
		"errorCode": he.ErrCode,
		"content":   body,
	}

	j, err := json.Marshal(&obj)
	if err != nil {
		ErrUnexpectedInternalServerError.HttpResp(c, err)
		return
	}

	c.Data(he.HttpCode, `application/json`, j)
}

func (he *HttpError) WSResp(w nhttp.ResponseWriter, args ...interface{}) {

	obj := map[string]interface{}{
		"code":      he.HttpCode,
		"errorCode": he.ErrCode,
	}

	if args == nil {
		obj[`message`] = he.Error()
	} else {
		obj[`message`] = fmt.Sprint(he.Error(), args)
	}

	j, err := json.Marshal(&obj)
	if err != nil {
		ErrUnexpectedInternalServerError.WSResp(w, err)
		return
	}

	w.WriteHeader(he.HttpCode)
	_, _ = w.Write(j)
}

func (he *HttpError) HttpResp(c *gin.Context, args ...interface{}) {
	obj := map[string]interface{}{
		"code":      he.HttpCode,
		"errorCode": he.ErrCode,
	}

	if args == nil {
		obj[`message`] = he.Error()
	} else {
		obj[`message`] = fmt.Sprint(he.Error(), args)
	}

	j, err := json.Marshal(&obj)
	if err != nil {
		ErrUnexpectedInternalServerError.HttpResp(c, err)
		return
	}

	c.Data(he.HttpCode, `application/json`, j)
}

func HttpErr(c *gin.Context, err error) {
	he, ok := err.(*HttpError)
	if ok {
		he.HttpResp(c)
	} else {
		he = NewErr(err, nhttp.StatusInternalServerError, ``)
		he.ErrCode = ""
		he.HttpResp(c)
	}
}

func WSErr(w nhttp.ResponseWriter, err error) {

	he, ok := err.(*HttpError)
	if ok {
		he.WSResp(w)
	} else {
		he = NewErr(err, nhttp.StatusInternalServerError, ``)
		he.ErrCode = ""
		he.WSResp(w)
	}
}

func titleErr(namespace string, err error) string {
	if err == nil {
		return ""
	}

	str := err.Error()
	elem := strings.Split(str, ` `)

	out := ``
	if namespace != `` {
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
