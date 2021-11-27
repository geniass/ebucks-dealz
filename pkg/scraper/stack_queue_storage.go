package scraper

import (
	"sync"
)

// StackQueueStorage is a very simple FILO stack storage backend for the colly queue.
type StackQueueStorage struct {
	lock  *sync.RWMutex
	stack [][]byte
}

func (s *StackQueueStorage) Init() error {
	s.lock = &sync.RWMutex{}
	return nil
}

func (s *StackQueueStorage) AddRequest(r []byte) error {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.stack = append(s.stack, r)

	return nil
}

func (s *StackQueueStorage) GetRequest() ([]byte, error) {
	s.lock.RLock()
	defer s.lock.RUnlock()

	n := len(s.stack) - 1
	r := s.stack[n]
	s.stack = s.stack[:n]

	return r, nil
}

func (s *StackQueueStorage) QueueSize() (int, error) {
	return len(s.stack), nil
}
