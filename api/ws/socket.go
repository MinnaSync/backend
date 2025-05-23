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
				client.Emit("error", websockets.ErrorMessage{
					Reason: "Failed to join room. ID is not string.",
				})

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
				client.Emit("error", websockets.ErrorMessage{
					Reason: "Failed to send message. Content is not interface.",
				})

				return
			}

			message, ok := messageContent["message"].(string)
			if !ok {
				client.Emit("error", websockets.ErrorMessage{
					Reason: "Failed to send message. Message is not string.",
				})
				return
			}

			room.Broadcast("receive_message", websockets.ClientReceiveMessage{
				Username: client.User.Username,
				Message:  message,
			})
		})

		client.On("queue_media", func(data any) {
			// TODO: In the future, some of these aren't really required.
			// For example: title, series, and poster_image_url are used just for filler.
			// We only need to actually keep track of an ID, url, and duration since the server relies on that.

			media, ok := data.(map[string]interface{})
			if !ok {
				client.Emit("error", websockets.ErrorMessage{
					Reason: "Failed to queue media.",
				})

				return
			}

			id, ok := media["id"].(string)
			if !ok {
				client.Emit("error", websockets.ErrorMessage{
					Reason: "Failed to queue media. ID is not string.",
				})

				return
			}

			title, ok := media["title"].(string)
			if !ok {
				client.Emit("error", websockets.ErrorMessage{
					Reason: "Failed to queue media. Title is not string.",
				})

				return
			}

			url, ok := media["url"].(string)
			if !ok {
				client.Emit("error", websockets.ErrorMessage{
					Reason: "Failed to queue media. URL is not string.",
				})

				return
			}

			duration, err := m3u8_duration.FetchM3u8Duration(url)
			if err != nil {
				logger.Log.Error("Failed to fetch m3u8 duration.", "err", err)
				client.Emit("error", websockets.ErrorMessage{
					Reason: "Failed to queue media. Unable to fetch duration for queued m3u8 file.",
				})

				return
			}

			series, ok := media["series"].(string)
			if !ok {
				client.Emit("error", websockets.ErrorMessage{
					Reason: "Failed to queue media. Series it not string.",
				})

				return
			}

			posterImageURL, ok := media["poster_image_url"].(string)
			if !ok {
				client.Emit("error", websockets.ErrorMessage{
					Reason: "Failed to queue media. Poster image is not string.",
				})

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
				client.Emit("error", websockets.ErrorMessage{
					Reason: "Failed to update player state. State is not interface.",
				})

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
