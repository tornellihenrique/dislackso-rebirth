package domain

import (
	"slices"
	"strings"
	"time"
)

// VoiceState is a user connected, or connecting, to a voice channel. The hub
// keeps voice states in memory only; a user has at most one.
type VoiceState struct {
	UserID    ID
	GuildID   ID
	ChannelID ID
	// SessionID is new on every join and travels in the media token, so
	// media server events about an older connection can be told apart.
	SessionID  string
	SelfMute   bool
	SelfDeaf   bool
	ServerMute bool
	ServerDeaf bool
	Streaming  bool
	// Connected is false until the media server reports the participant.
	Connected bool
	JoinedAt  time.Time
}

const voiceRoomPrefix = "vc_"

// VoiceRoom names the media server room of a voice channel.
func VoiceRoom(channelID ID) string { return voiceRoomPrefix + channelID.String() }

// ParseVoiceRoom returns the channel of a room named by VoiceRoom.
func ParseVoiceRoom(room string) (ID, bool) {
	rest, ok := strings.CutPrefix(room, voiceRoomPrefix)
	if !ok {
		return 0, false
	}
	id, err := ParseID(rest, "room")
	return id, err == nil && id != 0
}

// MediaSource is a kind of track a participant publishes.
type MediaSource string

const (
	SourceMicrophone       MediaSource = "microphone"
	SourceCamera           MediaSource = "camera"
	SourceScreenShare      MediaSource = "screen_share"
	SourceScreenShareAudio MediaSource = "screen_share_audio"
)

// VoicePermissions are what a participant may do in a voice room.
type VoicePermissions struct {
	Publish   []MediaSource
	Subscribe bool
}

// VoicePermissionsFor applies server mute (no microphone) and server deafen
// (no subscribing) to the sources a voice channel allows.
func VoicePermissionsFor(serverMute, serverDeaf bool) VoicePermissions {
	p := VoicePermissions{Subscribe: !serverDeaf}
	if !serverMute {
		p.Publish = append(p.Publish, SourceMicrophone)
	}
	p.Publish = append(p.Publish, SourceScreenShare, SourceScreenShareAudio)
	return p
}

// Without takes sources out of what may be published.
func (p VoicePermissions) Without(sources ...MediaSource) VoicePermissions {
	out := VoicePermissions{Subscribe: p.Subscribe}
	for _, s := range p.Publish {
		if !slices.Contains(sources, s) {
			out.Publish = append(out.Publish, s)
		}
	}
	return out
}
