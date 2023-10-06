package internal

type streamManager struct {
	Subscribers map[string]chan []byte
}

func newStreamManager() *streamManager {
	return &streamManager{
		Subscribers: make(map[string]chan []byte),
	}
}

// Subscribes client to http stream
func (s *streamManager) Subscribe(id string) chan []byte {
	channel := make(chan []byte)
	s.Subscribers[id] = channel
	return channel
}

// Unsubscribes client
func (s *streamManager) Unsubscribe(id string) {
	close(s.Subscribers[id])
	delete(s.Subscribers, id)
}

// Gracefully close manager
func (s *streamManager) CloseManager() {
	for id, sub := range s.Subscribers {
		close(sub)
		delete(s.Subscribers, id)
	}
}

// Transmit message to all connected clients
func (s *streamManager) Write(msg []byte) {
	for _, sub := range s.Subscribers {
		sub <- msg
	}
}
