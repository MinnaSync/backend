package ws

import (
	"fmt"
	"sync"
	"time"

	"github.com/MinnaSync/minna-sync-backend/internal/logger"
)

var (
	channels = make(map[string]*Channel)
)

type Channel struct {
	mu sync.Mutex

	id          string
	controller  *Client
	connections map[*Client]bool

	Playing  *NowPlayingMedia
	Queued   []Media
	Messages []ChannelMessage

	join  chan *Client
	leave chan *Client

	closed chan bool
}

func JoinChannel(channelId string, client *Client) *Channel {
	if c, exists := channels[channelId]; exists {
		c.connections[client] = true
		c.join <- client
		return c
	}

	channel := &Channel{
		mu: sync.Mutex{},

		id:          channelId,
		controller:  client,
		connections: make(map[*Client]bool),

		Playing:  nil,
		Queued:   make([]Media, 0),
		Messages: make([]ChannelMessage, 0, 100),

		join:  make(chan *Client),
		leave: make(chan *Client),

		closed: make(chan bool, 1),
	}

	go channel.open()
	channels[channelId] = channel
	channel.join <- client

	return channel
}

func (c *Channel) open() {
	for {
		select {
		case client, open := <-c.join:
			if !open {
				logger.Log.Warn("Client attempted to join closed channel.")
				return
			}

			c.connections[client] = true
			c.SendMessage(ChannelMessage{
				Type:     MessageTypeUserJoin,
				UTCEpoch: time.Now().Unix(),
				Username: "System",
				Content: fmt.Sprintf(
					"%s has joined the room.",
					client.User.Username,
				),
			})
		case client := <-c.leave:
			c.SendMessage(ChannelMessage{
				Type:     MessageTypeUserLeave,
				UTCEpoch: time.Now().Unix(),
				Username: "System",
				Content: fmt.Sprintf(
					"%s has left the room.",
					client.User.Username,
				),
			})

			if _, ok := c.connections[client]; ok {
				delete(c.connections, client)
				close(client.send)
			}

			if len(c.connections) == 0 {
				close(c.closed)
			} else {
				if c.controller != client {
					continue
				}

				var controller *Client
				for c := range c.connections {
					controller = c
					break
				}

				if controller != nil {
					c.controller = controller
					// logger.Log.Debug(fmt.Sprintf("User %s is now the controller for room %s.", controller.User.Username, c.id))
				}
			}
		case <-c.closed:
			c.mu.Lock()
			defer c.mu.Unlock()

			delete(channels, c.id)
			return
		}
	}
}

func (c *Channel) Broadcast(event string, data any, sender *Client) {
	for client := range c.connections {
		if client == sender {
			continue
		}

		client.Emit(event, data)
	}
}

func (c *Channel) Emit(event string, data any) {
	for client := range c.connections {
		client.Emit(event, data)
	}
}

func (c *Channel) playback() {
	for {
		select {
		case <-c.Playing.ticker.C:
			currentPlaybackTime := c.Playing.CurrentPlaybackTime()

			// Do not push any sync events when paused.
			if c.Playing.Paused {
				continue
			}

			if (int64(currentPlaybackTime) % 10) == 0 {
				c.Emit("state_sync", PlaybackState{
					Paused:      c.Playing.Paused,
					CurrentTime: currentPlaybackTime,
				})
			}

			if currentPlaybackTime > c.Playing.Duration {
				if len(c.Queued) != 0 {
					c.QueueChange()
					continue
				}

				close(c.Playing.finished)
			}
		case <-c.Playing.finished:
			c.Playing.ticker.Stop()
			c.Playing = nil
			return
		}
	}
}

func (c *Channel) QueueInsert(m Media) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.Playing != nil {
		c.Queued = append(c.Queued, m)
		c.Emit("queue_updated", m)

		c.SendMessage(ChannelMessage{
			Type:     MessageTypeMediaQueued,
			UTCEpoch: time.Now().Unix(),
			Username: "System",
			Content:  fmt.Sprintf("%s - %s has been added to the queue.", *m.Title, *m.Series),
		})

		return
	}

	c.Playing = &NowPlayingMedia{
		Media:       m,
		Paused:      false,
		CurrentTime: 0,

		lastChange: time.Now(),
		ticker:     time.NewTicker(1 * time.Second),
		finished:   make(chan bool),
	}

	c.Emit("media_changed", c.Playing)
	c.SendMessage(ChannelMessage{
		Type:     MessageTypeMediaChanged,
		UTCEpoch: time.Now().Unix(),
		Username: "System",
		Content:  fmt.Sprintf("%s - %s is now playing.", *m.Title, *m.Series),
	})

	go c.playback()
}

func (c *Channel) QueueChange() {
	c.mu.Lock()
	defer c.mu.Unlock()

	next := c.Queued[0]
	c.Playing = &NowPlayingMedia{
		Media:       next,
		Paused:      false,
		CurrentTime: 0,

		lastChange: time.Now(),
		ticker:     time.NewTicker(1 * time.Second),
		finished:   make(chan bool),
	}

	c.Emit("media_changed", c.Playing)
	c.SendMessage(ChannelMessage{
		Type:     MessageTypeMediaChanged,
		UTCEpoch: time.Now().Unix(),
		Username: "System",
		Content:  fmt.Sprintf("%s - %s is now playing.", *next.Title, *next.Series),
	})

	c.Queued = c.Queued[1:]
}

func (c *Channel) PlayerState(sender *Client, state PlaybackStateUpdated) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.Playing == nil {
		return
	}

	// Tells the sending client to sync back since they are not the controller.
	if c.controller != sender {
		currentPlaybackTime := c.Playing.CurrentPlaybackTime()

		sender.Emit("state_sync", PlaybackState{
			Paused:      c.Playing.Paused,
			CurrentTime: currentPlaybackTime,
		})

		return
	}

	c.Playing.ticker.Stop() // Stop the ticker to prevent sending any updates.

	// Handles pause/play state changes.
	if state.Paused != nil && c.Playing.Paused != *state.Paused {
		if *state.Paused == false {
			c.Playing.lastChange = time.Now()
		} else {
			c.Playing.CurrentTime = c.Playing.CurrentPlaybackTime()
		}

		c.Playing.Paused = *state.Paused
	}

	// Handles current playback time changes.
	if state.CurrentTime != nil && c.Playing.CurrentTime != *state.CurrentTime {
		c.Playing.lastChange = time.Now()
		c.Playing.CurrentTime = *state.CurrentTime
	}

	c.Broadcast("state_updated", PlaybackState{
		Paused:      c.Playing.Paused,
		CurrentTime: c.Playing.CurrentTime,
	}, sender)

	c.Playing.ticker.Reset(1 * time.Second) // Restarts the ticker to start sending updates again
}

func (c *Channel) SendMessage(message ChannelMessage) {
	c.Messages = append(c.Messages, message)
	c.Emit("channel_message", message)
}
