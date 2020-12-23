package http

import (
	"errors"
	nhttp "net/http"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestHTTPErr(t *testing.T) {
	errTest := NewNamespaceErr(errors.New("test error"), nhttp.StatusForbidden, "testing")
	errOK := NewNamespaceErr(nil, nhttp.StatusOK, "")

	DefaultNamespace = "testing2"
	errTest2 := NewErr(errors.New("test error2"), nhttp.StatusForbidden)

	router := gin.New()
	g := router.Group("")

	g.GET("/err", func(c *gin.Context) { HttpErr(c, errTest) })
	g.GET("/err2", func(c *gin.Context) { HttpErr(c, errTest2) })
	g.GET("/errf", func(c *gin.Context) { HttpErrf(c, errTest, "%s: %s", "this is a test error", "ignore me") })
	g.GET("/ok", func(c *gin.Context) { errOK.HttpBody(c, map[string]interface{}{"data1": 1, "data2": "abc"}) })

	srv := nhttp.Server{
		Addr:    ":8090",
		Handler: router,
	}

	if err := srv.ListenAndServe(); err != nil && err != nhttp.ErrServerClosed {
		panic(err)
	}
}

func TestMsgError(t *testing.T) {
	errTest := NewNamespaceErr(errors.New("test error"), nhttp.StatusForbidden, "testing")

	router := gin.New()
	g := router.Group("")

	g.GET("/errmsg",
		func(c *gin.Context) {
			err := Error(errTest, "this is a error with specific message")
			HttpErr(c, err)
		})

	g.GET("/errfmsg",
		func(c *gin.Context) {
			err := Errorf(errTest, "%s: %v", "this is a message with fmt", map[string]int{"abc": 123})
			HttpErr(c, err)
		})
	srv := nhttp.Server{
		Addr:    ":8090",
		Handler: router,
	}

	if err := srv.ListenAndServe(); err != nil && err != nhttp.ErrServerClosed {
		panic(err)
	}
}
