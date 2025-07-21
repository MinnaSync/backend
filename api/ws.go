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

	// TODO: This flow needs to be refactored.
	//
	// Right now it waits on the join_channel event before "connecting" to the actual channel.
	// We send a join notification the second the client connects.
	// This is done because we want to allow the user to set a guest username before firing the notification for joining.
	// Instead, it should immediately connect to the channel, but not send a join notification util we get the join_channel event.

	client.On("connection", func(msg any) {
		client.Emit("connected", map[string]any{})
	})

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

		channel := client.ChannelConnect(channelId)

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

			if episode, ok := media["episode"].(float64); ok {
				i := int(episode)
				mediaData.Episode = &i
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

		client.On("run_command", func(msg any) {
			command, ok := msg.(map[string]any)
			if !ok {
				logger.Log.Debug("Command failed to run. Command is not a structure.")
				return
			}

			commandType, ok := command["type"].(float64)
			if !ok {
				logger.Log.Debug("Command failed to run. Command type is not an integer.", "type", command["type"])
				return
			}

			switch ws.CommandType(commandType) {
			case ws.CommandTypeTakeRemote:
				channel.GrantControl(client)
				break
			case ws.CommandTypePurgeMessages:
				channel.PurgeMessages(client)
				break
			case ws.CommandTypeSkip:
				channel.QueueChange()
				break
			}
		})

		client.On("queue_remove", func(msg any) {
			mediaId, ok := msg.(map[string]any)
			if !ok {
				logger.Log.Debug("Media ID failed to remove. Media ID is not a structure.")
				return
			}

			id, ok := mediaId["id"].(string)
			if !ok {
				logger.Log.Debug("Media ID failed to remove. Media ID is not a string.")
				return
			}

			channel.QueueRemove(id)
		})
	})

	<-client.Disconnected
}
