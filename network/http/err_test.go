package http

import (
	"errors"
	nhttp "net/http"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestHTTPErr(t *testing.T) {
	errTest := NewErr(errors.New("test error"), nhttp.StatusForbidden, "testing")
	errOK := NewErr(nil, nhttp.StatusOK, "")

	router := gin.New()
	g := router.Group("")

	g.GET("/err", func(c *gin.Context) { HttpErr(c, errTest, "this is a test error") })
	g.GET("/ok", func(c *gin.Context) { errOK.HttpBody(c, map[string]interface{}{"data1": 1, "data2": "abc"}) })

	srv := nhttp.Server{
		Addr:    ":8090",
		Handler: router,
	}

	if err := srv.ListenAndServe(); err != nil && err != nhttp.ErrServerClosed {
		panic(err)
	}
}
