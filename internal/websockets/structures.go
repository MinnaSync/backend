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
