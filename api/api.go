package api

import (
	"strings"

	"github.com/MinnaSync/minna-sync-backend/config"
	"github.com/MinnaSync/minna-sync-backend/handlers"
	"github.com/MinnaSync/minna-sync-backend/internal/ws"
	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
)

func Register(app *fiber.App) {
	app.Use("/ws", handlers.WSUpgrader)
	app.Get("/ws", websocket.New(Websocket, websocket.Config{
		Origins: strings.Split(config.Conf.AllowOrigins, ","),

		ReadBufferSize:  ws.MaxBufferSize,
		WriteBufferSize: ws.MaxBufferSize,
	}))
}
