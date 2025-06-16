package main

import (
	"github.com/MinnaSync/minna-sync-backend/api"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
)

func main() {
	app := fiber.New(fiber.Config{
		Immutable: true,
	})

	app.Use(cors.New(cors.Config{
		AllowOrigins: "https://minna.gura.sa.com",
		AllowMethods: "GET,POST,OPTIONS",
	}))

	api.Register(app)
	app.Listen(":3001")
}
