package gateway

import (
	"time"

	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/domain"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/events"
)

type buffered struct {
	seq  uint64
	at   time.Time
	data []byte
}

// session is one identified gateway session. It outlives its connection by
// the resume window, buffering dispatches so a reconnecting client misses
// nothing. Every field is guarded by Server.mu.
type session struct {
	id            string
	userID        domain.ID
	authSessionID domain.ID
	topics        map[events.Topic]bool

	// ready is set once Ready went out; earlier events wait in pending and
	// get their seq after it.
	ready   bool
	pending [][]byte
	seq     uint64
	replay  []buffered

	conn   *connection
	expiry *time.Timer
}

// push sequences an encoded event and sends it if a connection is attached.
func (s *session) push(event []byte, now time.Time, limit int, window time.Duration) {
	if !s.ready {
		s.pending = append(s.pending, event)
		return
	}
	s.seq++
	frame := dispatchFrame(s.seq, event)
	s.replay = append(s.replay, buffered{seq: s.seq, at: now, data: frame})
	s.trim(now, limit, window)
	if s.conn != nil {
		s.conn.send(frame)
	}
}

// trim keeps at most limit frames and none older than the resume window.
func (s *session) trim(now time.Time, limit int, window time.Duration) {
	drop := 0
	for drop < len(s.replay) && (len(s.replay)-drop > limit || now.Sub(s.replay[drop].at) > window) {
		drop++
	}
	if drop > 0 {
		s.replay = append(s.replay[:0], s.replay[drop:]...)
	}
}

// ack forgets frames the client confirmed processing.
func (s *session) ack(lastSeq uint64) {
	drop := 0
	for drop < len(s.replay) && s.replay[drop].seq <= lastSeq {
		drop++
	}
	if drop > 0 {
		s.replay = append(s.replay[:0], s.replay[drop:]...)
	}
}

// missed returns the frames after lastSeq, or false when some of them are
// no longer buffered.
func (s *session) missed(lastSeq uint64) ([][]byte, bool) {
	if lastSeq > s.seq {
		return nil, false
	}
	if lastSeq == s.seq {
		return nil, true
	}
	if len(s.replay) == 0 || s.replay[0].seq > lastSeq+1 {
		return nil, false
	}
	var out [][]byte
	for _, b := range s.replay {
		if b.seq > lastSeq {
			out = append(out, b.data)
		}
	}
	return out, true
}
