package ws

import websockets "github.com/MinnaSync/minna-sync-backend/internal/websockets"

type ClientSendMessage struct {
	// The message that was sent from the client
	Message string `json:"message"`
}

type ClientReceiveMessage struct {
	// The user that sent the message.
	Username string `json:"username"`

	// The message that was sent.
	Message string `json:"message"`
}

type ClientQueueMedia struct {
	// The title of the media that was queued.
	Title string `json:"title"`

	// The URL for the media that was queued.
	URL string `json:"url"`
}

type RoomMediaQueued struct {
	Title string `json:"title"`
	URL   string `json:"url"`
}

type RoomData struct {
	NowPlaying *websockets.NowPlayingMedia `json:"now_playing"`
	Queue      []websockets.QueuedMedia    `json:"queue"`
}
