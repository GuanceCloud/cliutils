package http

import (
	"log"
	"strings"

	"github.com/gin-gonic/gin"
)

var hdrs = []string{
	`X-Real-IP`,
	`X-Forwarded-For`,
}

func GetCliIP(c *gin.Context) (string, bool) {

	log.Printf("[debug] %+#v", c.Request.Header)

	for _, h := range hdrs {
		addr := c.Request.Header.Get(h)
		if len(addr) > 0 {

			realIP := strings.Split(addr, `,`)[0] // 此处只取第一个 IP
			if err != nil {
				log.Printf("[error] invalid addr: %s", addr)
				// ignore
				continue
			}

			log.Printf("[debug] ip:port: %s:%s", realIP, port)

			return realIP, true
		}
	}

	return c.ClientIP(), false
}
