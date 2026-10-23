package connection

import (
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type SafeConn struct {
	conn *websocket.Conn
	mu   sync.Mutex
	ID   int64
}

func NewSafeConn(conn *websocket.Conn) *SafeConn {
	return &SafeConn{
		conn: conn,
		mu:   sync.Mutex{},
		ID:   time.Now().UnixNano(),
	}
}

func (sc *SafeConn) WriteMessage(messageType int, data []byte) error {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	if err := sc.conn.SetWriteDeadline(time.Now().Add(30 * time.Second)); err != nil {
		return err
	}
	return sc.conn.WriteMessage(messageType, data)
}

func (sc *SafeConn) WriteJSON(v interface{}) error {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	if err := sc.conn.SetWriteDeadline(time.Now().Add(30 * time.Second)); err != nil {
		return err
	}
	return sc.conn.WriteJSON(v)
}

// Best-effort terminal shutdown notification. A stalled data writer must never
// prevent revocation from closing the connection.
func (sc *SafeConn) TryWriteJSON(v interface{}) {
	if !sc.mu.TryLock() {
		return
	}
	defer sc.mu.Unlock()
	_ = sc.conn.SetWriteDeadline(time.Now().Add(time.Second))
	_ = sc.conn.WriteJSON(v)
}

func (sc *SafeConn) WriteControl(kind int, data []byte, deadline time.Time) error {
	return sc.conn.WriteControl(kind, data, deadline)
}

func (sc *SafeConn) Close() error {
	// gorilla permits Close concurrently with reads/writes. Do not wait for
	// the write mutex: cancellation must interrupt a blocked network write.
	return sc.conn.Close()
}
func (sc *SafeConn) ReadMessage() (int, []byte, error) {
	return sc.conn.ReadMessage()
}
func (sc *SafeConn) ReadJSON(v interface{}) error {
	return sc.conn.ReadJSON(v)
}
func (sc *SafeConn) SetReadDeadline(t time.Time) error {
	return sc.conn.SetReadDeadline(t)
}
func (sc *SafeConn) GetConn() *websocket.Conn {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	return sc.conn
}
