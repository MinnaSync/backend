package api

import (
	"github.com/MinnaSync/minna-sync-backend/internal/m3u8_duration"
	"github.com/MinnaSync/minna-sync-backend/internal/ws"
	"github.com/gofiber/contrib/websocket"
)

func Websocket(c *websocket.Conn) {
	client := ws.Serve(c)

	client.On("connection", func(msg any) {
		client.Emit("connected", any(nil))

		var channel *ws.Channel
		client.On("join_channel", func(msg any) {
			joinInfo, ok := msg.(map[string]any)
			if !ok {
				println("not ok 1")
				return
			}

			channelId, ok := joinInfo["channel_id"].(string)
			if !ok {
				println("not ok 2")
				return
			}

			if username, ok := joinInfo["guest_username"].(string); ok {
				if len := len(username); len > 3 && len < 16 {
					client.User.Username = username
				}
			}

			channel = client.ChannelConnect(channelId)

			var nowPlaying *ws.NowPlayingMedia
			if channel.Playing != nil {
				nowPlaying = &ws.NowPlayingMedia{
					Media:       channel.Playing.Media,
					Paused:      channel.Playing.Paused,
					CurrentTime: channel.Playing.CurrentPlaybackTime(),
				}
			}

			client.Emit("room_data", ws.RoomData{
				NowPlaying: nowPlaying,
				Queue:      channel.Queued,
			})
		})

		client.On("send_message", func(msg any) {
			messageContent, ok := msg.(map[string]any)
			if !ok {
				return
			}

			message, ok := messageContent["message"].(string)
			if !ok {
				return
			}

			channel.Emit("receive_message", ws.ClientMessage{
				Username: client.User.Username,
				Message:  message,
			})
		})

		client.On("queue_media", func(msg any) {
			var mediaData ws.Media

			media, ok := msg.(map[string]any)
			if !ok {
				return
			}

			id, ok := media["id"].(string)
			if !ok {
				return
			}
			mediaData.ID = id

			if title, ok := media["title"].(string); ok {
				mediaData.Title = &title
			}

			url, ok := media["url"].(string)
			if !ok {
				return
			}
			mediaData.URL = url

			duration, err := m3u8_duration.FetchM3u8Duration(url)
			if err != nil {
				return
			}
			mediaData.Duration = duration

			if series, ok := media["series"].(string); ok {
				mediaData.Series = &series
			}

			if posterImageURL, ok := media["poster_image_url"].(string); ok {
				mediaData.PosterImageURL = &posterImageURL
			}

			channel.QueueInsert(mediaData)
		})

		client.On("player_state", func(msg any) {
			state, ok := msg.(map[string]interface{})
			if !ok {
				return
			}

			updatedState := ws.PlaybackStateUpdated{}

			if paused, ok := state["paused"].(bool); ok {
				updatedState.Paused = &paused
			}

			if currentTime, ok := state["current_time"].(float64); ok {
				updatedState.CurrentTime = &currentTime
			}

			channel.PlayerState(client, updatedState)
		})
	})

	<-client.Disconnected
}
