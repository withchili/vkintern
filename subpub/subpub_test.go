package subpub

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestPublishAndSubscribe(t *testing.T) {
	sp := NewSubPub()
	defer sp.Close(context.Background())

	done := make(chan struct{})
	_, err := sp.Subscribe("foo", func(msg interface{}) {
		if msg.(int) != 42 {
			t.Errorf("expected 42, got %v", msg)
		}
		close(done)
	})
	if err != nil {
		t.Fatalf("subscribe error: %v", err)
	}

	if err := sp.Publish("foo", 42); err != nil {
		t.Fatalf("publish error: %v", err)
	}

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for message")
	}
}

func TestMultipleSubscribers(t *testing.T) {
	sp := NewSubPub()
	defer sp.Close(context.Background())

	var c1, c2 int32
	sp.Subscribe("bar", func(msg interface{}) {
		atomic.AddInt32(&c1, 1)
	})
	sp.Subscribe("bar", func(msg interface{}) {
		atomic.AddInt32(&c2, 1)
	})

	for i := 0; i < 5; i++ {
		sp.Publish("bar", i)
	}
	time.Sleep(100 * time.Millisecond)

	if atomic.LoadInt32(&c1) != 5 || atomic.LoadInt32(&c2) != 5 {
		t.Fatalf("counters wrong: %d, %d", c1, c2)
	}
}

func TestSlowSubscriberDoesNotBlock(t *testing.T) {
	sp := NewSubPub()
	defer sp.Close(context.Background())

	var fastCnt int32
	slowDone := make(chan struct{})

	sp.Subscribe("topic", func(msg interface{}) {
		atomic.AddInt32(&fastCnt, 1)
	})
	sp.Subscribe("topic", func(msg interface{}) {
		time.Sleep(800 * time.Millisecond)
		if msg.(int) == 9 {
			close(slowDone)
		}
	})

	start := time.Now()
	for i := 0; i < 10; i++ {
		sp.Publish("topic", i)
	}

	for atomic.LoadInt32(&fastCnt) < 10 {
		time.Sleep(10 * time.Millisecond)
	}
	elapsed := time.Since(start)
	if elapsed > 300*time.Millisecond {
		t.Fatalf("fast subscriber blocked, took %v", elapsed)
	}

	<-slowDone
}

func TestUnsubscribe(t *testing.T) {
	sp := NewSubPub()
	defer sp.Close(context.Background())

	var cnt int32
	sub, _ := sp.Subscribe("a", func(msg interface{}) { atomic.AddInt32(&cnt, 1) })

	sp.Publish("a", 1)
	time.Sleep(20 * time.Millisecond)

	sub.Unsubscribe()
	sp.Publish("a", 2)
	time.Sleep(20 * time.Millisecond)

	if atomic.LoadInt32(&cnt) != 1 {
		t.Fatalf("expected 1 message, got %d", cnt)
	}
}

func TestCloseGraceful(t *testing.T) {
	sp := NewSubPub()

	var cnt int32
	sp.Subscribe("g", func(msg interface{}) {
		time.Sleep(50 * time.Millisecond)
		atomic.AddInt32(&cnt, 1)
	})

	for i := 0; i < 5; i++ {
		sp.Publish("g", i)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	if err := sp.Close(ctx); err != nil {
		t.Fatalf("unexpected close error: %v", err)
	}

	if atomic.LoadInt32(&cnt) != 5 {
		t.Fatalf("not all messages processed, got %d", cnt)
	}
}

func TestCloseTimeout(t *testing.T) {
	sp := NewSubPub()
	sp.Subscribe("h", func(msg interface{}) {
		time.Sleep(200 * time.Millisecond)
	})

	sp.Publish("h", 1)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if err := sp.Close(ctx); err == nil {
		t.Fatal("expected deadline exceeded error")
	}
}
