package websockets

import (
	"encoding/json"
	"math"
	"time"
)

type QueuedMedia struct {
	Title    string  `json:"title"`
	URL      string  `json:"url"`
	Duration float64 `json:"-"`
}

type NowPlayingMedia struct {
	Title       string  `json:"title"`
	URL         string  `json:"url"`
	Duration    float64 `json:"-"`
	Paused      bool    `json:"paused"`
	CurrentTime float64 `json:"current_time"`

	ticker *time.Ticker
	done   chan bool
}

type Room struct {
	id         string
	clients    map[*Client]bool
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
		case message := <-r.broadcast:
			for client := range r.clients {
				select {
				case client.send <- message:
				default:
				}
			}
		case <-r.closed:
		}
	}
}

func (r *Room) QueueInsert(data QueuedMedia) {
	if r.Playing != nil {
		r.Queue = append(r.Queue, data)
		r.Broadcast("queue_updated", &r.Playing)
		return
	}

	r.Playing = &NowPlayingMedia{
		Title:       data.Title,
		URL:         data.URL,
		Duration:    data.Duration,
		Paused:      false,
		CurrentTime: 0,

		ticker: time.NewTicker(1 * time.Second),
	}

	r.Broadcast("media_changed", &r.Playing)
	go r.startTicker()
}

func (r *Room) QueueChange() {
	next := r.Queue[0]
	r.Playing = &NowPlayingMedia{
		Title:       next.Title,
		URL:         next.URL,
		Duration:    next.Duration,
		Paused:      false,
		CurrentTime: 0,
	}

	r.Broadcast("media_changed", &r.Playing)
	r.Queue = r.Queue[1:]
}

func (r *Room) startTicker() {
	for {
		select {
		case <-r.Playing.ticker.C:
			r.Playing.CurrentTime += float64(1 * time.Second.Seconds())

			// If the current time is an interval of 5, tell the client to sync if it's desynced..
			if math.Mod(r.Playing.CurrentTime, 5) == 0 {
				r.Broadcast("time_updated", &r.Playing)
			}

			if r.Playing.CurrentTime >= r.Playing.Duration {
				r.QueueChange()
			}
		case <-r.Playing.done:
			break
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
