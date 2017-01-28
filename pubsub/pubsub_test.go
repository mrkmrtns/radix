package pubsub

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	. "testing"
	"time"

	radix "github.com/mediocregopher/radix.v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func randStr() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

func testConn(t *T) radix.Conn {
	c, err := radix.Dial("tcp", "localhost:6379")
	require.Nil(t, err)
	return c
}

func publish(t *T, c radix.Conn, ch, msg string) {
	require.Nil(t, radix.Cmd("PUBLISH", ch, msg).Run(c))
}

func assertMsgRead(t *T, msgCh <-chan Message) Message {
	select {
	case m := <-msgCh:
		return m
	case <-time.After(5 * time.Second):
		t.Fatal("timedout reading")
	}
	panic("shouldn't get here")
}

func assertMsgNoRead(t *T, msgCh <-chan Message) {
	select {
	case msg, ok := <-msgCh:
		if !ok {
			assert.Fail(t, "msgCh closed")
		} else {
			assert.Fail(t, "unexpected Message off msgCh", "msg:%#v", msg)
		}
	default:
	}
}

func TestSubscribe(t *T) {
	pubC := testConn(t)
	c := New(testConn(t))
	msgCh := make(chan Message, 1)

	ch1, ch2, msgStr := randStr(), randStr(), randStr()
	require.Nil(t, c.Subscribe(msgCh, ch1, ch2))

	count := 100
	wg := new(sync.WaitGroup)

	wg.Add(1)
	go func() {
		for i := 0; i < count; i++ {
			publish(t, pubC, ch1, msgStr)
		}
		wg.Done()
	}()

	wg.Add(1)
	go func() {
		for i := 0; i < count; i++ {
			msg := assertMsgRead(t, msgCh)
			assert.Equal(t, Message{
				Type:    "message",
				Channel: ch1,
				Message: []byte(msgStr),
			}, msg)
		}
		wg.Done()
	}()

	wg.Add(1)
	go func() {
		for i := 0; i < count; i++ {
			require.Nil(t, c.Ping())
		}
		wg.Done()
	}()
	wg.Wait()

	require.Nil(t, c.Unsubscribe(msgCh, ch1))
	publish(t, pubC, ch1, msgStr)
	publish(t, pubC, ch2, msgStr)
	msg := assertMsgRead(t, msgCh)
	assert.Equal(t, Message{
		Type:    "message",
		Channel: ch2,
		Message: []byte(msgStr),
	}, msg)

	c.Close()
	assert.NotNil(t, c.Ping())
	assert.NotNil(t, c.Ping())
	assert.NotNil(t, c.Ping())
	publish(t, pubC, ch2, msgStr)
	time.Sleep(250 * time.Millisecond)
	assertMsgNoRead(t, msgCh)
}

func TestPSubscribe(t *T) {
	pubC := testConn(t)
	c := New(testConn(t))
	msgCh := make(chan Message, 1)

	p1, p2, msgStr := randStr()+"_*", randStr()+"_*", randStr()
	ch1, ch2 := p1+"_"+randStr(), p2+"_"+randStr()
	require.Nil(t, c.PSubscribe(msgCh, p1, p2))

	count := 100
	wg := new(sync.WaitGroup)

	wg.Add(1)
	go func() {
		for i := 0; i < count; i++ {
			publish(t, pubC, ch1, msgStr)
		}
		wg.Done()
	}()

	wg.Add(1)
	go func() {
		for i := 0; i < count; i++ {
			msg := assertMsgRead(t, msgCh)
			assert.Equal(t, Message{
				Type:    "pmessage",
				Pattern: p1,
				Channel: ch1,
				Message: []byte(msgStr),
			}, msg)
		}
		wg.Done()
	}()

	wg.Add(1)
	go func() {
		for i := 0; i < count; i++ {
			require.Nil(t, c.Ping())
		}
		wg.Done()
	}()

	wg.Wait()

	require.Nil(t, c.PUnsubscribe(msgCh, p1))
	publish(t, pubC, ch1, msgStr)
	publish(t, pubC, ch2, msgStr)
	msg := assertMsgRead(t, msgCh)
	assert.Equal(t, Message{
		Type:    "pmessage",
		Pattern: p2,
		Channel: ch2,
		Message: []byte(msgStr),
	}, msg)

	c.Close()
	assert.NotNil(t, c.Ping())
	assert.NotNil(t, c.Ping())
	assert.NotNil(t, c.Ping())
	publish(t, pubC, ch2, msgStr)
	time.Sleep(250 * time.Millisecond)
	assertMsgNoRead(t, msgCh)
}