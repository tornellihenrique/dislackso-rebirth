package fakes

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/domain"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/ports"
)

// MediaCall is one call the hub made to the media server.
type MediaCall struct {
	Op          string
	Room        string
	Identity    string
	Permissions domain.VoicePermissions
}

// Media is a media server whose tokens spell out their contents and whose
// rooms are whatever Rooms holds.
type Media struct {
	mu    sync.Mutex
	Down  bool
	Calls []MediaCall
	Rooms map[string][]ports.MediaParticipant
}

func (m *Media) URL() string { return "/lk" }

func (m *Media) Available() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return !m.Down
}

// JoinToken returns "room|identity|metadata|sources|subscribe|ttl".
func (m *Media) JoinToken(j ports.MediaJoin) (string, error) {
	sources := make([]string, len(j.Permissions.Publish))
	for i, s := range j.Permissions.Publish {
		sources[i] = string(s)
	}
	return fmt.Sprintf("%s|%s|%s|%s|%t|%s", j.Room, j.Identity, j.Metadata,
		strings.Join(sources, ","), j.Permissions.Subscribe, j.TTL), nil
}

func (m *Media) record(c MediaCall) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Calls = append(m.Calls, c)
}

func (m *Media) RemoveParticipant(_ context.Context, room, identity string) error {
	m.record(MediaCall{Op: "remove", Room: room, Identity: identity})
	return nil
}

func (m *Media) SetPermissions(_ context.Context, room, identity string, p domain.VoicePermissions) error {
	m.record(MediaCall{Op: "permissions", Room: room, Identity: identity, Permissions: p})
	return nil
}

func (m *Media) ListRooms(context.Context) ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	rooms := make([]string, 0, len(m.Rooms))
	for r := range m.Rooms {
		rooms = append(rooms, r)
	}
	slices.Sort(rooms)
	return rooms, nil
}

func (m *Media) ListParticipants(_ context.Context, room string) ([]ports.MediaParticipant, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return slices.Clone(m.Rooms[room]), nil
}

// TakeCalls returns the calls so far and forgets them.
func (m *Media) TakeCalls() []MediaCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	calls := m.Calls
	m.Calls = nil
	return calls
}

// ICE issues Servers, or fails with Err.
type ICE struct {
	Servers []ports.IceServer
	Err     error
}

func (ICE) Name() string { return "fake" }

func (i ICE) Issue(context.Context, domain.ID, time.Duration) ([]ports.IceServer, error) {
	return i.Servers, i.Err
}
