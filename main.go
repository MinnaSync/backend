package main

import (
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
	}))

	ws.Register(wsRoutes)

	wsRoutes.Run(":3001")
}
