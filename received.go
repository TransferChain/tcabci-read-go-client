package tcabcireadgoclient

// Received message
type Received struct {
	MessageType    int
	ReadingMessage []byte
	Err            error
}
