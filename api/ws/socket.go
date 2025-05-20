package ws

import (
	"github.com/MinnaSync/minna-sync-backend/internal/logger"
	"github.com/MinnaSync/minna-sync-backend/internal/m3u8_duration"
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

			// since playing could be nil, we have to see if media is playing first.
			// has to be done this way otherwise there'll be a nil reference error.
			// reason this has to be done in the first place is because of how playback time is calculates.
			playing := room.Playing
			if playing != nil {
				playing.CurrentTime = playing.CurrentPlaybackTime()
			}

			client.Emit("room_data", websockets.RoomData{
				NowPlaying: room.Playing,
				Queue:      room.Queue,
			})
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

			room.Broadcast("receive_message", websockets.ClientReceiveMessage{
				Username: client.User.Username,
				Message:  message,
			})
		})

		client.On("queue_media", func(data any) {
			media, ok := data.(map[string]interface{})
			if !ok {
				return
			}

			id, ok := media["id"].(string)
			if !ok {
				return
			}

			title, ok := media["title"].(string)
			if !ok {
				return
			}

			url, ok := media["url"].(string)
			if !ok {
				return
			}

			duration, err := m3u8_duration.FetchM3u8Duration(url)
			if err != nil {
				logger.Log.Error("Failed to fetch m3u8 duration.", "err", err)
				return
			}

			series, ok := media["series"].(string)
			if !ok {
				return
			}

			posterImageURL, ok := media["poster_image_url"].(string)
			if !ok {
				return
			}

			room.QueueInsert(websockets.QueuedMedia{
				ID:             id,
				Title:          title,
				Series:         series,
				URL:            url,
				PosterImageURL: posterImageURL,
				Duration:       duration,
			})
		})

		client.On("player_state", func(data any) {
			state, ok := data.(map[string]interface{})
			if !ok {
				return
			}

			updatedState := websockets.ClientStateUpdated{}

			if paused, ok := state["paused"].(bool); ok {
				updatedState.Paused = &paused
			}

			if currentTime, ok := state["current_time"].(float64); ok {
				updatedState.CurrentTime = &currentTime
			}

			room.UpdatePlayerState(updatedState, client)
		})
	})
}
