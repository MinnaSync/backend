package websockets

import (
	"encoding/json"
)

type Room struct {
	id         string
	clients    map[*Client]bool
	broadcast  chan []byte
	connect    chan *Client
	disconnect chan *Client
	closed     chan bool
	handlers   map[string]func(data any)
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
			r.Broadcast("user_joined", client.ID)
			r.clients[client] = true
		case client := <-r.disconnect:
			r.Broadcast("user_left", client.ID)
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
