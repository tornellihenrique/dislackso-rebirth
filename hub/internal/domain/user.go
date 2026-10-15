package domain

import "time"

type StatusPref string

const (
	StatusOnline    StatusPref = "online"
	StatusIdle      StatusPref = "idle"
	StatusDND       StatusPref = "dnd"
	StatusInvisible StatusPref = "invisible"
)

func (s StatusPref) Valid() bool {
	switch s {
	case StatusOnline, StatusIdle, StatusDND, StatusInvisible:
		return true
	default:
		return false
	}
}

type User struct {
	ID              ID
	Username        string
	Email           string
	EmailVerifiedAt *time.Time
	PasswordHash    string
	DisplayName     string
	AvatarFileID    ID
	StatusPref      StatusPref
	CustomStatus    string
	IsInstanceAdmin bool
	DisabledAt      *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func (u User) EmailVerified() bool { return u.EmailVerifiedAt != nil }

func (u User) Disabled() bool { return u.DisabledAt != nil }
