package subpub

import (
	"context"
	"errors"
	"sync"
)

// MessageHandler is a callback function that processes messages delivered to subscribers.
type MessageHandler func(msg interface{})

type Subscription interface {
	// Unsubscribe will remove interest in the current subject subscription is for.
	Unsubscribe()
}

type SubPub interface {
	// Subscribe creates an asynchronous queue subscriber on the given subject.
	Subscribe(subject string, cb MessageHandler) (Subscription, error)

	// Publish publishes the msg argument to the given subject.
	Publish(subject string, msg interface{}) error

	// Close will shut down sub-pub system.
	// May be blocked by data delivery until the context is canceled.
	Close(ctx context.Context) error
}

func NewSubPub() SubPub {
	return &subpub{
		subs: make(map[string][]*subscription),
	}
}

type subscription struct {
	parent  *subpub
	subject string
	cb      MessageHandler
	ch      chan interface{}
	done    chan struct{}
}

func (s *subscription) Unsubscribe() {
	s.parent.mu.Lock()
	subs := s.parent.subs[s.subject]
	for i, sub := range subs {
		if sub == s {
			subs[i] = subs[len(subs)-1]
			subs = subs[:len(subs)-1]
			break
		}
	}
	if len(subs) == 0 {
		delete(s.parent.subs, s.subject)
	} else {
		s.parent.subs[s.subject] = subs
	}
	s.parent.mu.Unlock()
	close(s.done)
}

type subpub struct {
	subs   map[string][]*subscription
	mu     sync.Mutex
	wg     sync.WaitGroup
	closed bool
}

func (s *subpub) Subscribe(subject string, cb MessageHandler) (Subscription, error) {
	sub := &subscription{
		parent:  s,
		subject: subject,
		cb:      cb,
		ch:      make(chan interface{}, 16),
		done:    make(chan struct{}),
	}
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil, errors.New("already closed")
	}
	s.subs[subject] = append(s.subs[subject], sub)
	s.mu.Unlock()
	s.wg.Add(1)
	go func(sub *subscription) {
		defer sub.parent.wg.Done()
		for {
			select {
			case msg := <-sub.ch:
				sub.cb(msg)
			case <-sub.done:
				for {
					select {
					case msg := <-sub.ch:
						sub.cb(msg)
					default:
						return
					}
				}
			}
		}
	}(sub)
	return sub, nil
}

func (s *subpub) Publish(subject string, msg interface{}) error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return errors.New("already closed")
	}
	defer s.mu.Unlock()
	for _, sub := range s.subs[subject] {
		select {
		case sub.ch <- msg:
		default:
			go func(sub *subscription, msg interface{}) {
				sub.ch <- msg
			}(sub, msg)
		}
	}
	return nil
}

func (s *subpub) Close(ctx context.Context) error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	for _, subs := range s.subs {
		for _, sub := range subs {
			close(sub.done)
		}
	}
	s.mu.Unlock()
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
