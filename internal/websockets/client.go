package websockets

import (
	"crypto/rand"
	"encoding/json"
	"net/http"

	"github.com/gorilla/websocket"
)

type Client struct {
	ID           string
	conn         *websocket.Conn
	disconnected chan bool
	send         chan []byte
	recieve      chan []byte
	handlers     map[string]func(data any)
}

var (
	Clients = make(map[string]*Client)
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		return origin == "http://localhost:5173"
	},
}

func generateID() string {
	var chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890-"

	length := 10

	ll := len(chars)
	b := make([]byte, length)
	rand.Read(b)

	for i := range length {
		b[i] = chars[int(b[i])%ll]
	}

	return string(b)
}

func NewClient(id string, conn *websocket.Conn) *Client {
	return &Client{
		ID:           id,
		conn:         conn,
		send:         make(chan []byte, 256),
		recieve:      make(chan []byte, 256),
		disconnected: make(chan bool),
		handlers:     make(map[string]func(data any)),
	}
}

func (c *Client) Handle(incoming *Message) {
	if c.handlers == nil {
		return
	}

	event := c.handlers[incoming.Event]
	if event == nil {
		return
	}

	event(incoming.Data)
}

func (c *Client) Emit(event string, data any) {
	message, err := json.Marshal(Message{
		Event: event,
		Data:  data,
	})
	if err != nil {
		return
	}

	c.send <- message
}

func (c *Client) Off(event string) {
	delete(c.handlers, event)
}

func (c *Client) On(event string, handler func(data any)) {
	c.handlers[event] = handler
}

func (c *Client) Once(event string, handler func(data any)) {
	c.handlers[event] = func(data any) {
		c.Off(event)
		handler(data)
	}
}

func (c *Client) Close() {
}

func (c *Client) Join(roomId string) *Room {
	return JoinRoom(roomId, c)
}

func (c *Client) Leave(roomId string) {
	println(roomId)
	LeaveRoom(roomId, c)
}

func (c *Client) writePump() {
	for {
		select {
		case message := <-c.send:
			err := c.conn.WriteMessage(websocket.TextMessage, message)
			if err != nil {
				return
			}
		case message := <-c.recieve:
			var incoming Message

			err := json.Unmarshal(message, &incoming)
			if err != nil {
				continue
			}

			c.Handle(&incoming)
		case <-c.disconnected:
			c.Close()
			return
		}
	}
}

func (c *Client) readPump() {
	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			return
		}

		c.recieve <- message
	}
}

func Serve(w http.ResponseWriter, r *http.Request) *Client {
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return nil
	}

	client := NewClient(generateID(), c)

	go client.writePump()
	go client.readPump()

	return client
}
