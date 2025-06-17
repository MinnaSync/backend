package ws

import (
	"github.com/gofiber/contrib/websocket"
)

func Serve(c *websocket.Conn) *Client {
	client := NewClient(c)

	go client.writePump()
	go client.readPump()

	return client
}
