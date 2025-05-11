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

	client.Once("connection", func(data any) {
		client.Emit("connected", data)

		client.Once("join_room", func(data any) {
			roomId, ok := data.(string)
			if !ok {
				return
			}

			room := client.Join(roomId)
			client.On("send_message", func(data any) {
				room.Emit("receive_message", data)
			})
		})
	})
}
