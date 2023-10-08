package internal

import "sync"

// Thread safe struct for managing multiple listeners on http stream or
// SSE listeners.
type streamManager struct {
	Subscribers map[string]chan []byte
	managerLock sync.Mutex
}

func newStreamManager() *streamManager {
	return &streamManager{
		Subscribers: make(map[string]chan []byte),
	}
}

// Subscribes client to http stream
func (s *streamManager) Subscribe(id string) chan []byte {
	s.managerLock.Lock()
	channel := make(chan []byte)
	s.Subscribers[id] = channel
	s.managerLock.Unlock()
	return channel
}

// Unsubscribes client
func (s *streamManager) Unsubscribe(id string) {
	s.managerLock.Lock()

	// As we remove subsribers automatically if stream ended, we need to check
	// for nil pointer in case client tries to remove itself as well
	if s.Subscribers[id] != nil {
		close(s.Subscribers[id])
		delete(s.Subscribers, id)
	}
	s.managerLock.Unlock()
}

// Gracefully close manager
func (s *streamManager) CloseManager() {
	s.managerLock.Lock()
	for id, sub := range s.Subscribers {
		close(sub)
		delete(s.Subscribers, id)
	}
	s.managerLock.Unlock()
}

// Transmit message to all connected clients
func (s *streamManager) Write(msg []byte) {
	s.managerLock.Lock()
	for _, sub := range s.Subscribers {
		sub <- msg
	}
	s.managerLock.Unlock()
}
