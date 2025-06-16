package ws

import (
	"time"

	"github.com/gofiber/contrib/websocket"
)

var (
	// The max size a message sent to the Websocket can be.
	MaxBufferSize = 512 * 2
	// How long a client has to read the next message.
	ResponseWait = 60 * time.Second
	// How long a client has to write a message
	ReplyWait = 60 * time.Second
	// How often the client should be pinged by the server.
	PingInterval = 30 * time.Second
)

func Serve(c *websocket.Conn) *Client {
	client := NewClient(c)

	go client.writePump()
	go client.readPump()

	return client
}
