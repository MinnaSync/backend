package ws

import "time"

// -- Clients --

type Message struct {
	Event string `json:"event"`
	Data  any    `json:"data"`
}

type ClientJoinedRoom struct {
	Username string `json:"username"`
}

type ClientLeaveRoom struct {
	Username string `json:"username"`
}

type ClientMessage struct {
	Username string `json:"username"`
	Message  string `json:"message"`
}

type PlaybackState struct {
	Paused      bool    `json:"paused"`
	CurrentTime float64 `json:"current_time"`
}

type RoomData struct {
	NowPlaying *NowPlayingMedia `json:"now_playing"`
	Queue      []Media          `json:"queue"`
	Messages   []ChannelMessage `json:"messages"`
}

// -- Channels --

type BroadcastMessage struct {
	Client  *Client
	Message Message
}

type Media struct {
	ID             string  `json:"id"`
	Title          *string `json:"title"`
	Series         *string `json:"series"`
	URL            string  `json:"url"`
	PosterImageURL *string `json:"poster_image_url"`
	Duration       float64 `json:"-"`
}

type NowPlayingMedia struct {
	Media
	Paused      bool    `json:"paused"`
	CurrentTime float64 `json:"current_time"`

	lastResume time.Time
	ticker     *time.Ticker
	finished   chan bool
}

func (n *NowPlayingMedia) CurrentPlaybackTime() float64 {
	if n.Paused {
		return n.CurrentTime
	}

	return n.CurrentTime + float64(time.Since(n.lastResume).Seconds())
}

type PlaybackStateUpdated struct {
	Paused      *bool    `json:"paused"`
	CurrentTime *float64 `json:"current_time"`
}

type MessageType int

const (
	MessageTypeNotification MessageType = iota
	MessageTypeUserJoin
	MessageTypeUserLeave
	MessageTypeUserMessage
	MessageTypeMediaChanged
	MessageTypeMediaQueued
)

type ChannelMessage struct {
	Type     MessageType `json:"type"`
	UTCEpoch int64       `json:"utc_epoch"`
	Username string      `json:"username"`
	Content  string      `json:"content"`
}
