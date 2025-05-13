package ws

// Used for when the server receives a message from the client.
// Use the stored information on the client to SendMessage back with username and content.
type ClientSendMessage struct {
	Message string `json:"message"`
}

// Used for sending a message to the client.
type ClientReceiveMessage struct {
	Username string `json:"username"`
	Message  string `json:"message"`
}
