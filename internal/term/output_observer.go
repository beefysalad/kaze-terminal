package term

import (
	"sync"
	"time"
)

type outputObserver struct{ call func() }
type outputObservers struct{ list []*outputObserver }

// AddOutputObserver subscribes to applied output. Callbacks run outside the
// session lock and must not block: use a nonblocking send to a buffered channel.
// Removal is idempotent. An in-flight callback may finish after removal.
func (s *Session) AddOutputObserver(call func()) func() {
	entry := &outputObserver{call: call}
	s.observerMu.Lock()
	next := &outputObservers{}
	if old := s.observers.Load(); old != nil {
		next.list = append(next.list, old.list...)
	}
	next.list = append(next.list, entry)
	s.observers.Store(next)
	s.observerMu.Unlock()
	var once sync.Once
	return func() {
		once.Do(func() {
			s.observerMu.Lock()
			defer s.observerMu.Unlock()
			old := s.observers.Load()
			next := &outputObservers{}
			for _, item := range old.list {
				if item != entry {
					next.list = append(next.list, item)
				}
			}
			if len(next.list) == 0 {
				s.observers.Store(nil)
			} else {
				s.observers.Store(next)
			}
		})
	}
}

// OutputObserverCount reports current subscriptions without taking Session.Lock.
func (s *Session) OutputObserverCount() int {
	if list := s.observers.Load(); list != nil {
		return len(list.list)
	}
	return 0
}

// LastOutputTime is the last applied program output, or zero before any output.
func (s *Session) LastOutputTime() time.Time {
	if n := s.lastOutput.Load(); n != 0 {
		return time.Unix(0, n)
	}
	return time.Time{}
}
func (s *Session) notifyOutput() {
	s.lastOutput.Store(time.Now().UnixNano())
	if list := s.observers.Load(); list != nil {
		for _, entry := range list.list {
			entry.call()
		}
	}
	s.notify()
}
