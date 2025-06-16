package ws

import (
	"sync"
	"time"
)

var (
	channels = make(map[string]*Channel)
)

type Channel struct {
	mu sync.Mutex

	id          string
	controller  *Client
	connections map[*Client]bool

	Playing *NowPlayingMedia
	Queued  []Media

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

		Playing: nil,
		Queued:  make([]Media, 0),

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
				return
			}

			c.connections[client] = true
			c.Emit("user_joined", ClientJoinedRoom{
				Username: client.User.Username,
			})
		case client := <-c.leave:
			if _, ok := c.connections[client]; ok {
				delete(c.connections, client)
				close(client.send)
			}

			c.Emit("user_left", ClientLeaveRoom{
				Username: client.User.Username,
			})

			if len(c.connections) == 0 {
				close(c.join)
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
		return
	}

	c.Playing = &NowPlayingMedia{
		Media:       m,
		Paused:      false,
		CurrentTime: 0,

		lastResume: time.Now(),
		ticker:     time.NewTicker(1 * time.Second),
		finished:   make(chan bool),
	}

	c.Emit("media_changed", c.Playing)
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

		lastResume: time.Now(),
		ticker:     time.NewTicker(1 * time.Second),
		finished:   make(chan bool),
	}

	c.Emit("media_changed", c.Playing)
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

	// Handles pause/play state changes.
	if state.Paused != nil && c.Playing.Paused != *state.Paused {
		c.Playing.Paused = *state.Paused

		if *state.Paused == false {
			c.Playing.lastResume = time.Now()
		} else {
			c.Playing.CurrentTime = c.Playing.CurrentPlaybackTime()
		}
	}

	// Handles current playback time changes.
	if state.CurrentTime != nil && c.Playing.CurrentTime != *state.CurrentTime {
		c.Playing.CurrentTime = *state.CurrentTime
	}

	c.Broadcast("state_updated", PlaybackState{
		Paused:      c.Playing.Paused,
		CurrentTime: c.Playing.CurrentTime,
	}, sender)
}
