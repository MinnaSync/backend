package websockets

type Message struct {
	Event string `json:"event"`
	Data  any    `json:"data"`
}

type ClientJoinRoom struct {
	Username string `json:"username"`
}

type ClientLeaveRoom struct {
	Username string `json:"username"`
}

type ClientTimeUpdated struct {
	Paused      bool    `json:"paused"`
	CurrentTime float64 `json:"current_time"`
	UserUpdated bool    `json:"user_updated"`
}

type ClientStateUpdated struct {
	Paused      *bool    `json:"paused"`
	CurrentTime *float64 `json:"current_time"`
	UserUpdated bool     `json:"user_updated"`
}

type ClientSendMessage struct {
	Message string `json:"message"`
}

type ClientReceiveMessage struct {
	Username string `json:"username"`
	Message  string `json:"message"`
}

type ClientQueueMedia struct {
	Title string `json:"title"`
	URL   string `json:"url"`
}

type RoomMediaQueued struct {
	Title string `json:"title"`
	URL   string `json:"url"`
}

type RoomData struct {
	NowPlaying *NowPlayingMedia `json:"now_playing"`
	Queue      []QueuedMedia    `json:"queue"`
}
