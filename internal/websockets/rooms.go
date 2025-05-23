package websockets

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/MinnaSync/minna-sync-backend/internal/logger"
)

type QueuedMedia struct {
	ID             string `json:"id"`
	Title          string `json:"title"`
	Series         string `json:"series"`
	URL            string `json:"url"`
	PosterImageURL string `json:"poster_image_url"`

	Duration float64 `json:"-"`
}

type NowPlayingMedia struct {
	Title          string  `json:"title"`
	Series         string  `json:"series"`
	URL            string  `json:"url"`
	PosterImageURL string  `json:"poster_image_url"`
	Paused         bool    `json:"paused"`
	CurrentTime    float64 `json:"current_time"`

	Duration   float64   `json:"-"`
	LastResume time.Time `json:"-"`

	ticker *time.Ticker
	done   chan bool
}

func (n *NowPlayingMedia) CurrentPlaybackTime() float64 {
	if n.Paused {
		return n.CurrentTime
	}

	return n.CurrentTime + float64(time.Since(n.LastResume).Seconds())
}

type Room struct {
	id         string
	clients    map[*Client]bool
	controller *Client
	broadcast  chan []byte
	connect    chan *Client
	disconnect chan *Client
	closed     chan bool
	handlers   map[string]func(data any)

	Playing *NowPlayingMedia
	Queue   []QueuedMedia
}

var (
	rooms = make(map[string]*Room)
)

func JoinRoom(roomId string, client *Client) *Room {
	if r, ok := rooms[roomId]; ok {
		r.connect <- client
		return rooms[roomId]
	}

	room := &Room{
		id:         roomId,
		clients:    make(map[*Client]bool),
		controller: client,
		broadcast:  make(chan []byte, 256),
		connect:    make(chan *Client),
		disconnect: make(chan *Client),
		closed:     make(chan bool),
		handlers:   make(map[string]func(data any)),
		Queue:      make([]QueuedMedia, 0),
	}
	go room.run()
	rooms[roomId] = room

	room.connect <- client
	return room
}

func (r *Room) run() {
	for {
		select {
		case client := <-r.connect:
			r.Broadcast("user_joined", ClientJoinRoom{
				Username: client.User.Username,
			})

			r.clients[client] = true
		case client := <-r.disconnect:
			r.Broadcast("user_left", ClientLeaveRoom{
				Username: client.User.Username,
			})

			if _, ok := r.clients[client]; ok {
				delete(r.clients, client)
				close(client.send)
			}

			if len(r.clients) == 0 {
				close(r.closed)
			} else {
				if r.controller != client {
					continue
				}

				var newController *Client
				for c := range r.clients {
					newController = c
					break
				}

				if newController != nil {
					r.controller = newController
					logger.Log.Debug(fmt.Sprintf("User %s is now the controller for room %s.", newController.User.Username, r.id))
				}
			}
		case message := <-r.broadcast:
			for client := range r.clients {
				select {
				case client.send <- message:
				default:
				}
			}
		case <-r.closed:
			delete(rooms, r.id)
			return
		}
	}
}

func (r *Room) QueueInsert(data QueuedMedia) {
	if r.Playing != nil {
		r.Queue = append(r.Queue, data)
		r.Broadcast("queue_updated", data)
		return
	}

	r.Playing = &NowPlayingMedia{
		Title:          data.Title,
		Series:         data.Series,
		URL:            data.URL,
		PosterImageURL: data.PosterImageURL,
		Paused:         false,
		CurrentTime:    0,

		Duration:   data.Duration,
		LastResume: time.Now(),

		ticker: time.NewTicker(1 * time.Second),
		done:   make(chan bool),
	}

	r.Broadcast("media_changed", &r.Playing)
	go r.startTicker()
}

func (r *Room) QueueChange() {
	next := r.Queue[0]
	r.Playing = &NowPlayingMedia{
		Title:          next.Title,
		Series:         next.Series,
		URL:            next.URL,
		PosterImageURL: next.PosterImageURL,
		Paused:         false,
		CurrentTime:    0,

		Duration:   next.Duration,
		LastResume: time.Now(),

		ticker: time.NewTicker(1 * time.Second),
		done:   make(chan bool),
	}

	r.Broadcast("media_changed", &r.Playing)
	r.Queue = r.Queue[1:]
}

func (r *Room) UpdatePlayerState(data ClientStateUpdated, c *Client) {
	if r.Playing == nil {
		return
	}

	if r.controller != c {
		currentTime := r.Playing.CurrentPlaybackTime()

		// Tell the client to sync back since they are not the controller.
		r.Broadcast("state_sync", ClientTimeUpdated{
			Paused:      r.Playing.Paused,
			CurrentTime: currentTime,
		})

		return
	}

	if data.Paused != nil && r.Playing.Paused != *data.Paused {
		if *data.Paused == false {
			r.Playing.LastResume = time.Now()
		} else {
			r.Playing.CurrentTime = r.Playing.CurrentPlaybackTime()
		}

		r.Playing.Paused = *data.Paused
	}

	if data.CurrentTime != nil && r.Playing.CurrentTime != *data.CurrentTime {
		r.Playing.CurrentTime = *data.CurrentTime
	}

	r.Emit("state_updated", ClientTimeUpdated{
		Paused:      r.Playing.Paused,
		CurrentTime: r.Playing.CurrentTime,
	}, c)
}

func (r *Room) startTicker() {
	for {
		select {
		case <-r.Playing.ticker.C:
			currentTime := r.Playing.CurrentPlaybackTime()

			// Do not push any sync events.
			if r.Playing.Paused {
				continue
			}

			// Otheriwse, tell all clients to sync the state every 10 seconds of the current time.
			if (int64(currentTime) % 10) == 0 {
				r.Broadcast("state_sync", ClientTimeUpdated{
					Paused:      r.Playing.Paused,
					CurrentTime: currentTime,
				})
			}

			// When the duration is up, it should attempt to change the queue.
			// Otherwise, it closes the playing as done.
			if currentTime > r.Playing.Duration {
				if len(r.Queue) != 0 {
					r.QueueChange()
					continue
				}

				close(r.Playing.done)
			}
		case <-r.Playing.done:
			r.Playing.ticker.Stop()
			r.Playing = nil
			return
		}
	}
}

func (r *Room) Broadcast(event string, data any) {
	message, err := json.Marshal(Message{
		Event: event,
		Data:  data,
	})
	if err != nil {
		return
	}

	select {
	case r.broadcast <- message:
	default:
	}
}

func (r *Room) Emit(event string, data any, c *Client) {
	message, err := json.Marshal(Message{
		Event: event,
		Data:  data,
	})
	if err != nil {
		return
	}

	for client := range r.clients {
		if client.id == c.id {
			continue
		}

		select {
		case client.send <- message:
		default:
		}
	}
}
