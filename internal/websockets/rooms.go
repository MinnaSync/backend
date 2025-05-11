package websockets

import "encoding/json"

type Room struct {
	id         string
	clients    map[*Client]bool
	broadcast  chan []byte
	connect    chan *Client
	disconnect chan *Client
	closed     chan bool
}

var (
	rooms = make(map[string]*Room)
)

func JoinRoom(roomId string, client *Client) *Room {
	if r, ok := rooms[roomId]; ok {
		r.connect <- client
		r.Emit("user_joined", client.ID)
		return rooms[roomId]
	}

	room := &Room{
		id:         roomId,
		clients:    make(map[*Client]bool),
		broadcast:  make(chan []byte),
		connect:    make(chan *Client),
		disconnect: make(chan *Client),
		closed:     make(chan bool),
	}
	rooms[roomId] = room

	go room.run()
	room.connect <- client
	room.Emit("user_joined", client.ID)

	return room
}

func LeaveRoom(roomId string, client *Client) {
	if r, ok := rooms[roomId]; ok {
		r.disconnect <- client
		r.Emit("user_left", client.ID)
	}
}

func (r *Room) Close() {
}

func (r *Room) run() {
	for {
		select {
		case client := <-r.connect:
			r.clients[client] = true
		case client := <-r.disconnect:
			if _, ok := r.clients[client]; ok {
				delete(r.clients, client)
				client.Close()
			}

			if len(r.clients) == 0 {
				r.Close()
			}
		case message := <-r.broadcast:
			for client := range r.clients {
				select {
				case client.send <- message:
				}
			}
		case <-r.closed:
			for client := range r.clients {
				client.Close()
			}
			return
		}
	}
}

func (r *Room) Emit(event string, data any) {
	message, err := json.Marshal(Message{
		Event: event,
		Data:  data,
	})
	if err != nil {
		return
	}

	r.broadcast <- message
}
