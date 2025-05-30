package websockets

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/MinnaSync/minna-sync-backend/internal/logger"
	"github.com/gorilla/websocket"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = 30 * time.Second

	// Maximum message size allowed from peer.
	maxMessageSize = 512 * 2
)

type ClientUser struct {
	Username string
}

type Client struct {
	id           string
	name         string
	conn         *websocket.Conn
	room         *Room
	disconnected chan bool
	send         chan []byte
	recieve      chan []byte
	err          chan []byte
	handlers     map[string]func(data any)
	ratelimit    time.Duration
	lastRead     time.Time

	User *ClientUser
}

type ClientOptions struct {
	Id   string
	Name *string
}

var (
	Clients = make(map[string]*Client)
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// We really don't need the origin to be checked by the upgrader.
	// It's checked in a business layer further up.
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func NewClient(conn *websocket.Conn, id string) *Client {
	defaultName := fmt.Sprintf("Guest_%s", id)

	return &Client{
		id:           id,
		name:         defaultName,
		conn:         conn,
		room:         nil,
		send:         make(chan []byte, maxMessageSize),
		recieve:      make(chan []byte, maxMessageSize),
		err:          make(chan []byte, maxMessageSize),
		disconnected: make(chan bool),
		handlers:     make(map[string]func(data any)),
		ratelimit:    250 * time.Millisecond,
		lastRead:     time.Now(),

		User: &ClientUser{
			Username: fmt.Sprintf("Guest_%v", id),
		},
	}
}

func (c *Client) Handle(incoming *Message) {
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

func (c *Client) Join(roomId string) *Room {
	c.room = JoinRoom(roomId, c)
	return c.room
}

func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))

			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			err := c.conn.WriteMessage(websocket.TextMessage, message)
			if err != nil {
				logger.Log.Error("Failed to write message.", "err", err)
				return
			}
		case message := <-c.recieve:
			var incoming Message

			err := json.Unmarshal(message, &incoming)
			if err != nil {
				logger.Log.Error("Failed to unmarshal message.", "err", err)
				return
			}

			c.Handle(&incoming)
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				logger.Log.Error("Failed to write ping.", "err", err)
				return
			}
		case <-c.disconnected:
			if c.room != nil {
				c.room.disconnect <- c
			}

			delete(Clients, c.id)
		}
	}
}

func (c *Client) readPump() {
	defer func() {
		c.disconnected <- true
	}()

	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error { c.conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })

	for {
		now := time.Now()
		if now.Sub(c.lastRead) < c.ratelimit {
			continue
		}
		c.lastRead = now

		_, message, err := c.conn.ReadMessage()
		if err != nil {
			logger.Log.Debug("Websocket disconnected.", "err", err)
			return
		}

		c.recieve <- message
	}
}

func generateId() string {
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

func Serve(w http.ResponseWriter, r *http.Request) *Client {
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		logger.Log.Error("Failed to upgrade websocket.", "err", err)
		return nil
	}

	client := NewClient(c, generateId())

	go client.writePump()
	go client.readPump()

	return client
}
