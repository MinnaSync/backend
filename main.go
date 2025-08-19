package main

import (
	"github.com/MinnaSync/minna-sync-backend/api"
	"github.com/MinnaSync/minna-sync-backend/config"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/sirupsen/logrus"
)

func init() {
	level, err := logrus.ParseLevel(config.Conf.LogLevel)

	if err != nil {
		panic(err)
	}

	logrus.SetLevel(level)
}

func main() {
	app := fiber.New(fiber.Config{
		Immutable: true,
	})

	app.Use(cors.New(cors.Config{
		AllowOrigins: config.Conf.AllowOrigins,
		AllowMethods: "GET,POST,OPTIONS",
	}))

	api.Register(app)
	app.Listen(":" + config.Conf.Port)
}
