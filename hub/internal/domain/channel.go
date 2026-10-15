package domain

import (
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

type ChannelType string

const (
	ChannelCategory ChannelType = "category"
	ChannelText     ChannelType = "text"
	ChannelVoice    ChannelType = "voice"
)

func (t ChannelType) Valid() bool {
	return t == ChannelCategory || t == ChannelText || t == ChannelVoice
}

type Channel struct {
	ID      ID
	GuildID ID
	// ParentID is the category holding the channel; zero at the top level.
	ParentID ID
	Type     ChannelType
	Name     string
	Topic    string
	// Position orders siblings: categories among categories, other channels
	// within their parent.
	Position        int
	UserLimit       int
	DrawingAllowed  bool
	MaxShareQuality string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

const (
	ChannelNameMaxLen  = 100
	ChannelTopicMaxLen = 1024
	VoiceUserLimitMax  = 99
)

// SharePresets are the screen share presets from ARCHITECTURE §9.5, lowest first.
var SharePresets = []string{"720p30", "1080p30", "1080p60", "1440p30", "1440p60"}

// NormalizeChannelName trims space; text channel names are also lowercased
// with dashes for spaces, as in "#release-notes".
func NormalizeChannelName(t ChannelType, name string) string {
	name = strings.TrimSpace(name)
	if t != ChannelText {
		return name
	}
	return strings.Join(strings.FieldsFunc(strings.ToLower(name), func(r rune) bool {
		return unicode.IsSpace(r) || r == '-'
	}), "-")
}

func ValidateChannelName(field, name string) *FieldViolation {
	switch n := utf8.RuneCountInString(name); {
	case n == 0:
		return &FieldViolation{field, ReasonFieldRequired}
	case n > ChannelNameMaxLen:
		return &FieldViolation{field, ReasonFieldTooLong}
	}
	if hasControl(name) {
		return &FieldViolation{field, ReasonFieldInvalidFormat}
	}
	return nil
}

func ValidateChannelTopic(field, topic string) *FieldViolation {
	if utf8.RuneCountInString(topic) > ChannelTopicMaxLen {
		return &FieldViolation{field, ReasonFieldTooLong}
	}
	if strings.ContainsFunc(topic, func(r rune) bool { return r != '\n' && isControl(r) }) {
		return &FieldViolation{field, ReasonFieldInvalidFormat}
	}
	return nil
}

func ValidateUserLimit(field string, limit int) *FieldViolation {
	if limit < 0 || limit > VoiceUserLimitMax {
		return &FieldViolation{field, ReasonFieldInvalid}
	}
	return nil
}

// ValidateSharePreset accepts a known preset or empty (the instance maximum).
func ValidateSharePreset(field, preset string) *FieldViolation {
	if preset != "" && !slices.Contains(SharePresets, preset) {
		return &FieldViolation{field, ReasonFieldInvalid}
	}
	return nil
}
