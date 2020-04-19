package http

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestMiddlewares(t *testing.T) {
	r := gin.New()

	r.Use(CORSMiddleware)
	r.Use(TraceIDMiddleware)
	r.Use(RequestLoggerMiddleware)

	v1 := r.Group("/v1")
	v1.GET("/get", func(c *gin.Context) { c.Data(400, "application/json", []byte(`{"error": "get-error"}`)) })
	v1.GET("/get500", func(c *gin.Context) { c.Data(500, "application/json", []byte(`{"error": "get-error"}`)) })
	v1.POST("/post", func(c *gin.Context) { c.Data(400, "application/json", []byte(`{"error": "post-error"}`)) })
	v1.GET("/getok", func(c *gin.Context) { c.Data(200, "application/json", []byte(`{"get": "ok"}`)) })
	v1.POST("/postok", func(c *gin.Context) { c.Data(200, "application/json", []byte(`{"post": "ok"}`)) })

	srv := &http.Server{
		Addr:    `localhost:1234`,
		Handler: r,
	}

	if err := srv.ListenAndServe(); err != nil {
		panic(err)
	}
}
