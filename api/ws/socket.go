package ws

import (
	websockets "github.com/MinnaSync/minna-sync-backend/internal/websockets"
	"github.com/gin-gonic/gin"
)

func Socket(c *gin.Context) {
	client := websockets.Serve(c.Writer, c.Request)

	if client == nil {
		return
	}

	client.On("connection", func(data any) {
		// Tell the client that they are connected
		// This is used to start other handlers on connection.
		client.Emit("connected", data)

		var room *websockets.Room
		client.On("join_room", func(data any) {
			roomId, ok := data.(string)
			if !ok {
				return
			}

			room = client.Join(roomId)
		})

		client.On("send_message", func(data any) {
			messageContent, ok := data.(map[string]interface{})
			if !ok {
				return
			}

			message, ok := messageContent["message"].(string)
			if !ok {
				return
			}

			room.Broadcast("receive_message", ClientReceiveMessage{
				Username: client.User.Username,
				Message:  message,
			})
		})
	})
}
