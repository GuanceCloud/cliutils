package http

import (
	"strings"

	"github.com/gin-gonic/gin"
)

var hdrs = []string{
	`X-Real-IP`,
	`X-Forwarded-For`,
}

func GetCliIP(c *gin.Context) (string, bool) {

	for _, h := range hdrs {
		addr := c.Request.Header.Get(h)
		if len(addr) > 0 {
			realIP := strings.Split(addr, `,`)[0] // 此处只取第一个 IP
			return realIP, true
		}
	}

	return c.ClientIP(), false
}
