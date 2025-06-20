package ws

import (
	"time"

	"github.com/gofiber/contrib/websocket"
	"github.com/google/uuid"
)

var (
	// The max size a message sent to the Websocket can be.
	MaxBufferSize = 512 * 2
	// How long a client has to read the next message.
	ResponseWait = 60 * time.Second
	// How long a client has to write a message
	ReplyWait = 60 * time.Second
	// How often the client should be pinged by the server.
	PingInterval = 30 * time.Second

	clients = make(map[string]*Client, 0)
)

type UserInfo struct {
	Username string
}

type Client struct {
	id   string
	conn *websocket.Conn

	send chan Message
	recv chan Message

	handlers map[string][]func(msg any)

	User         UserInfo
	Disconnected chan bool
	Channel      *Channel
}

func NewClient(conn *websocket.Conn) *Client {
	id := uuid.NewString()

	client := &Client{
		id:   id,
		conn: conn,

		send: make(chan Message, MaxBufferSize),
		recv: make(chan Message, MaxBufferSize),

		handlers: make(map[string][]func(msg any)),

		User: UserInfo{
			Username: "Guest_" + id,
		},
		Disconnected: make(chan bool, 1),
	}

	clients[id] = client

	return client
}

func (c *Client) writePump() {
	ticker := time.NewTicker(PingInterval)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case msg, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(ReplyWait))

			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			err := c.conn.WriteJSON(msg)
			if err != nil {
				return
			}
		case msg := <-c.recv:
			c.handle(msg)
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(ReplyWait))

			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		case <-c.Disconnected:
			if c.Channel != nil {
				c.Channel.leave <- c
			}

			delete(clients, c.id)

			return
		}
	}
}

func (c *Client) readPump() {
	defer func() {
		close(c.Disconnected)
	}()

	c.conn.SetReadLimit(int64(MaxBufferSize))
	c.conn.SetReadDeadline(time.Now().Add(ResponseWait))
	c.conn.SetPongHandler(func(_ string) error {
		c.conn.SetReadDeadline(time.Now().Add(ResponseWait))
		return nil
	})

	for {
		msg := new(Message)
		err := c.conn.ReadJSON(msg)
		if err != nil {
			return
		}

		c.recv <- *msg
	}
}

func (c *Client) handle(msg Message) {
	for _, handler := range c.handlers[msg.Event] {
		handler(msg.Data)
	}
}

func (c *Client) ChannelConnect(channelId string) *Channel {
	c.Channel = JoinChannel(channelId, c)
	return c.Channel
}

func (c *Client) On(event string, handler func(msg any)) {
	c.handlers[event] = append(c.handlers[event], handler)
}

func (c *Client) Emit(event string, msg any) {
	c.send <- Message{
		Event: event,
		Data:  msg,
	}
}
