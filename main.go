package main

import (
	"net"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/MinnaSync/minna-sync-backend/api/ws"
	"github.com/gin-contrib/cors"
)

func main() {
	wsRoutes := gin.Default()
	wsRoutes.SetTrustedProxies(nil)

	wsRoutes.Use(cors.New(cors.Config{
		AllowOrigins: []string{"https://minna.gura.sa.com"},
		AllowMethods: []string{"GET", "POST", "OPTIONS"},
		AllowOriginWithContextFunc: func(c *gin.Context, origin string) bool {
			originUrl, _ := url.Parse(origin)

			originHost := originUrl.Host
			originHost, _, _ = net.SplitHostPort(originHost)
			reqHost := c.Request.Host
			reqHost, _, _ = net.SplitHostPort(reqHost)

			return strings.HasSuffix(originHost, reqHost)
		},
	}))

	ws.Register(wsRoutes)

	wsRoutes.Run(":3001")
}
