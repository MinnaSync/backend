package api

import (
	"time"

	"github.com/MinnaSync/minna-sync-backend/internal/logger"
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
				logger.Log.Debug("Client failed to join. Join info is not a structure.")
				return
			}

			channelId, ok := joinInfo["channel_id"].(string)
			if !ok {
				logger.Log.Debug("Client provided an invalid channel_id.")
				return
			}

			if username, ok := joinInfo["guest_username"].(string); ok {
				if len := len(username); len >= 3 && len <= 16 {
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
				Messages:   channel.Messages,
			})
		})

		client.On("send_message", func(msg any) {
			messageContent, ok := msg.(map[string]any)
			if !ok {
				logger.Log.Debug("Message failed to send. Content is not a structure.")
				return
			}

			message, ok := messageContent["message"].(string)
			if !ok {
				logger.Log.Debug("Message failed to send. Message content is not a string.")
				return
			}

			channel.SendMessage(ws.ChannelMessage{
				Type:     ws.MessageTypeUserMessage,
				UTCEpoch: time.Now().Unix(),
				Username: client.User.Username,
				Content:  message,
			})
		})

		client.On("queue_media", func(msg any) {
			var mediaData ws.Media

			media, ok := msg.(map[string]any)
			if !ok {
				logger.Log.Debug("Media failed to queue. Media is not a structure.")
				return
			}

			id, ok := media["id"].(string)
			if !ok {
				logger.Log.Debug("Media failed to queue. Media ID is not a string.")
				return
			}
			mediaData.ID = id

			if title, ok := media["title"].(string); ok {
				mediaData.Title = &title
			}

			url, ok := media["url"].(string)
			if !ok {
				logger.Log.Debug("Media failed to queue. Media URL is not a string.")
				return
			}
			mediaData.URL = url

			duration, err := m3u8_duration.FetchM3u8Duration(url)
			if err != nil {
				logger.Log.Debug("Media failed to queue. Failed to fetch duration.")
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
