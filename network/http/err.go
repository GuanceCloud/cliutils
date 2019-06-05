package http

import (
	"encoding/json"
	"errors"
	"fmt"
	nhttp "net/http"
	"strings"
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

/*
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
} */

func (he *HttpError) JsonBody(body interface{}) ([]byte, error) {

	obj := map[string]interface{}{
		"code":      he.HttpCode,
		"errorCode": he.ErrCode,
		"content":   body,
	}

	j, err := json.Marshal(&obj)
	if err != nil {
		return nil, err
	}

	return j, nil
}

func (he *HttpError) JsonErr() ([]byte, error) {

	obj := map[string]interface{}{
		"code":      he.HttpCode,
		"errorCode": he.ErrCode,
	}

	j, err := json.Marshal(&obj)
	if err != nil {
		return nil, err
	}
	return j, nil
}

// 按照默认的方式返回既定的 error 信息
func HttpErr(err error) *HttpError {
	he, ok := err.(*HttpError)
	if ok {
		return he
	} else {
		he = NewErr(err, nhttp.StatusInternalServerError, ``)
		he.ErrCode = ""
		return he
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
