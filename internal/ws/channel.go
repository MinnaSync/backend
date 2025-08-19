package ws

import (
	"fmt"
	"slices"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
)

var (
	// The amount of messages that will be stored in-memory for a channel.
	MaxStoredMessages = 100

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
		Messages: make([]ChannelMessage, 0, MaxStoredMessages),

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
				log.Warn("Client attempted to join a closed channel.")
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

			if c.controller == nil {
				c.controller = client
			}
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

			// Removes the controller and selects a new one if the channel has clients.
			// TODO: Add a timer for when the room should automatically close after inactivity.
			if c.controller == client {
				c.controller = nil

				if len(c.connections) == 0 {
					continue
				}

				for c := range c.connections {
					c.Channel.controller = c
					break
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

func (c *Channel) SendMessage(message ChannelMessage) {
	if len(c.Messages) >= MaxStoredMessages {
		c.Messages = c.Messages[1:]
	}

	c.Messages = append(c.Messages, message)
	c.Emit("channel_message", message)
}

func (c *Channel) PurgeMessages(sender *Client) {
	c.Messages = make([]ChannelMessage, 0)

	c.Emit("command", Command{
		Type: CommandTypePurgeMessages,
	})

	c.SendMessage(ChannelMessage{
		Type:     MessageTypeNotification,
		UTCEpoch: time.Now().Unix(),
		Username: "System",
		Content:  fmt.Sprintf("%s has purged channel messages.", sender.User.Username),
	})
}

func (c *Channel) GrantControl(sender *Client) {
	if c.controller == sender {
		return
	}

	c.controller = sender
	c.SendMessage(ChannelMessage{
		Type:     MessageTypeNotification,
		UTCEpoch: time.Now().Unix(),
		Username: "System",
		Content:  fmt.Sprintf("%s has taken control of the room.", sender.User.Username),
	})
}

func (c *Channel) playback() {
	for {
		select {
		case <-c.Playing.ticker.C:
			currentPlaybackTime := c.Playing.CurrentPlaybackTime()

			if currentPlaybackTime >= c.Playing.Duration-0.5 {
				if len(c.Queued) != 0 {
					c.QueueChange()
					continue
				}

				close(c.Playing.finished)
				continue
			}

			if !c.Playing.Paused && (int64(currentPlaybackTime)%10) == 0 {
				c.Emit("state_sync", PlaybackState{
					Paused:      c.Playing.Paused,
					CurrentTime: currentPlaybackTime,
				})
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

func (c *Channel) QueueRemove(id string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for i, m := range c.Queued {
		if m.ID != id {
			continue
		}

		c.Queued = slices.Delete(c.Queued, i, i+1)

		c.Emit("media_removed", MediaId{
			ID: id,
		})
		c.SendMessage(ChannelMessage{
			Type:     MessageTypeMediaRemoved,
			UTCEpoch: time.Now().Unix(),
			Username: "System",
			Content:  fmt.Sprintf("%s - %s has been removed from the queue.", *m.Title, *m.Series),
		})
	}
}

func (c *Channel) QueueChange() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.Queued) == 0 {
		return
	}

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

func (c *Channel) QueueSort(id string, i int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.Queued) == 0 {
		return
	}

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
