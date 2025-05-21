package main

import (
	"github.com/gin-gonic/gin"

	"github.com/MinnaSync/minna-sync-backend/api/ws"
)

func main() {
	wsRoutes := gin.Default()
	wsRoutes.SetTrustedProxies(nil)
	ws.Register(wsRoutes)
	wsRoutes.Run(":3001")
}
