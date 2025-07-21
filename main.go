package main

import (
	"github.com/MinnaSync/minna-sync-backend/api"
	"github.com/MinnaSync/minna-sync-backend/config"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
)

func main() {
	config.Load()

	app := fiber.New(fiber.Config{
		Immutable: true,
	})

	app.Use(cors.New(cors.Config{
		AllowOrigins: config.WSAllowOrigins,
		AllowMethods: "GET,POST,OPTIONS",
	}))

	api.Register(app)
	app.Listen(":" + config.WSPort)
}
