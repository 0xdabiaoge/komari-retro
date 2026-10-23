package terminal

import (
	"encoding/json"
	"github.com/gorilla/websocket"
	"github.com/komari-monitor/komari/web/connection"
	"time"
)

const outputLimit = 256 << 10
const outputWindow = 64 << 10

type outputChunk struct {
	Seq  uint64 `json:"seq"`
	Data []byte `json:"data"`
}

// Session state is protected by TerminalSessionsMutex. changed broadcasts
// queue/connection changes; at most outputLimit bytes await browser rendering.
func signalLocked(s *TerminalSession) {
	if s.changed != nil {
		close(s.changed)
	}
	s.changed = make(chan struct{})
}
func enqueueOutput(id string, s *TerminalSession, data []byte) bool {
	for len(data) > 0 {
		n := len(data)
		if n > 16384 {
			n = 16384
		}
		TerminalSessionsMutex.Lock()
		if TerminalSessions[id] != s {
			TerminalSessionsMutex.Unlock()
			return false
		}
		if s.outputBytes+n > outputLimit {
			ch := s.changed
			TerminalSessionsMutex.Unlock()
			<-ch
			continue
		}
		s.nextSeq++
		s.output = append(s.output, outputChunk{s.nextSeq, append([]byte(nil), data[:n]...)})
		s.outputBytes += n
		signalLocked(s)
		TerminalSessionsMutex.Unlock()
		data = data[n:]
	}
	return true
}
func acknowledgeLocked(s *TerminalSession, seq uint64) bool {
	if seq > s.sentSeq {
		return false
	}
	for len(s.output) > 0 && s.output[0].Seq <= seq {
		s.outputBytes -= len(s.output[0].Data)
		s.output[0].Data = nil
		s.output = s.output[1:]
	}
	if seq > s.ackedSeq {
		s.ackedSeq = seq
	}
	signalLocked(s)
	return true
}
func writeBrowser(id string, s *TerminalSession, b *connection.SafeConn) {
	defer detachBrowser(id, s, b)
	for {
		TerminalSessionsMutex.Lock()
		if TerminalSessions[id] != s || s.Browser != b {
			TerminalSessionsMutex.Unlock()
			return
		}
		var next *outputChunk
		if len(s.output) == 0 && s.shellExited {
			TerminalSessionsMutex.Unlock()
			revokeTerminal(id, s)
			return
		}
		inflight := 0
		for i := range s.output {
			c := &s.output[i]
			if c.Seq <= s.sentSeq {
				inflight += len(c.Data)
			} else {
				next = c
				break
			}
		}
		if next == nil || inflight >= outputWindow {
			ch := s.changed
			TerminalSessionsMutex.Unlock()
			<-ch
			continue
		}
		chunk := *next
		s.sentSeq = chunk.Seq
		flow := s.flow
		TerminalSessionsMutex.Unlock()
		var err error
		if flow {
			err = b.WriteJSON(struct {
				Type string `json:"type"`
				outputChunk
			}{"output", chunk})
		} else {
			err = b.WriteMessage(websocket.BinaryMessage, chunk.Data)
		}
		if err != nil {
			return
		}
		if !flow {
			TerminalSessionsMutex.Lock()
			if s.Browser == b {
				acknowledgeLocked(s, chunk.Seq)
			}
			TerminalSessionsMutex.Unlock()
		}
	}
}
func readBrowser(id string, s *TerminalSession, b *connection.SafeConn) {
	defer detachBrowser(id, s, b)
	for {
		kind, data, err := b.ReadMessage()
		if err != nil {
			return
		}
		TerminalSessionsMutex.Lock()
		valid := TerminalSessions[id] == s && s.Browser == b
		a := s.Agent
		TerminalSessionsMutex.Unlock()
		if !valid {
			return
		}
		if s.Authorized != nil && !s.Authorized() {
			revokeTerminal(id, s)
			return
		}
		if kind == websocket.TextMessage {
			var cmd struct {
				Type string `json:"type"`
				Seq  uint64 `json:"seq"`
				Cols int    `json:"cols"`
				Rows int    `json:"rows"`
			}
			if json.Unmarshal(data, &cmd) != nil {
				continue
			}
			switch cmd.Type {
			case "ack":
				TerminalSessionsMutex.Lock()
				ok := s.Browser != b || acknowledgeLocked(s, cmd.Seq)
				TerminalSessionsMutex.Unlock()
				if !ok {
					revokeTerminal(id, s)
					return
				}
				continue
			case "close":
				revokeTerminal(id, s)
				return
			case "heartbeat":
				if a == nil {
					dispatchTerminal(id, s)
				}
				continue
			case "resize":
				if cmd.Cols < 1 || cmd.Cols > 1000 || cmd.Rows < 1 || cmd.Rows > 500 {
					continue
				}
			default:
				continue
			}
		}
		if a != nil {
			if err := a.WriteMessage(kind, data); err != nil {
				detachAgent(id, s, a)
			}
		}
	}
}

// Keep the Agent reader alive while the browser reconnects. A full bounded
// queue pauses reads, propagating backpressure over the existing TCP stream.
func ForwardTerminal(id string) {
	TerminalSessionsMutex.Lock()
	s := TerminalSessions[id]
	if s == nil || s.Agent == nil {
		TerminalSessionsMutex.Unlock()
		return
	}
	a := s.Agent
	TerminalSessionsMutex.Unlock()
	defer detachAgent(id, s, a)
	for {
		kind, data, err := a.ReadMessage()
		if err != nil {
			return
		}
		if s.Authorized != nil && !s.Authorized() {
			revokeTerminal(id, s)
			return
		}
		if kind == websocket.TextMessage {
			var c struct {
				Type           string `json:"type"`
				TerminalResume bool   `json:"terminalResume"`
			}
			if json.Unmarshal(data, &c) == nil && c.Type == "capabilities" {
				TerminalSessionsMutex.Lock()
				s.resumeCapable = c.TerminalResume
				TerminalSessionsMutex.Unlock()
				continue
			}
			if json.Unmarshal(data, &c) == nil && c.Type == "exit" {
				TerminalSessionsMutex.Lock()
				s.shellExited = true
				signalLocked(s)
				TerminalSessionsMutex.Unlock()
				return
			}
		}
		if kind == websocket.BinaryMessage || kind == websocket.TextMessage {
			if !enqueueOutput(id, s, data) {
				return
			}
		}
	}
}
func detachBrowser(id string, s *TerminalSession, b *connection.SafeConn) {
	TerminalSessionsMutex.Lock()
	if TerminalSessions[id] != s || s.Browser != b {
		TerminalSessionsMutex.Unlock()
		return
	}
	s.Browser = nil
	s.sentSeq = s.ackedSeq
	signalLocked(s)
	s.expiry = time.AfterFunc(5*time.Minute, func() {
		TerminalSessionsMutex.Lock()
		expired := TerminalSessions[id] == s && s.Browser == nil
		if expired {
			s.expiring = true
		}
		TerminalSessionsMutex.Unlock()
		if expired {
			revokeTerminal(id, s)
		}
	})
	TerminalSessionsMutex.Unlock()
	_ = b.Close()
}
func detachAgent(id string, s *TerminalSession, a *connection.SafeConn) {
	TerminalSessionsMutex.Lock()
	if TerminalSessions[id] != s || s.Agent != a {
		TerminalSessionsMutex.Unlock()
		return
	}
	s.Agent = nil
	s.agentDeadline = time.Now().Add(5 * time.Minute)
	// Legacy Agents cannot distinguish shell exit from a transient disconnect.
	// End their channel rather than silently creating a replacement shell.
	if !s.resumeCapable {
		s.shellExited = true
	}
	b := s.Browser
	signalLocked(s)
	TerminalSessionsMutex.Unlock()
	_ = a.Close()
	TerminalSessionsMutex.Lock()
	exited := s.shellExited
	TerminalSessionsMutex.Unlock()
	if b != nil && !exited {
		_ = b.WriteJSON(map[string]string{"type": "reconnecting", "message": "节点终端通道断开，正在恢复；节点断线期间的输出可能不完整。"})
	}
}
