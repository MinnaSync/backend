package main

import (
	"github.com/gin-gonic/gin"

	"github.com/MinnaSync/minna-sync-backend/api/rest"
	"github.com/MinnaSync/minna-sync-backend/api/ws"
)

func main() {
	restRoutes := gin.Default()
	wsRoutes := gin.Default()

	restRoutes.SetTrustedProxies(nil)
	wsRoutes.SetTrustedProxies(nil)

	rest.Register(restRoutes)
	ws.Register(wsRoutes)

	go restRoutes.Run(":8443")
	wsRoutes.Run(":3000")
}
