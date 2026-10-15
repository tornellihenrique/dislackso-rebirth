package domain

// PresenceStatus is what other users see.
type PresenceStatus string

const (
	PresenceOffline PresenceStatus = "offline"
	PresenceOnline  PresenceStatus = "online"
	PresenceIdle    PresenceStatus = "idle"
	PresenceDND     PresenceStatus = "dnd"
)

type Presence struct {
	UserID ID
	Status PresenceStatus
	// CustomStatus is empty while offline.
	CustomStatus string
}

// EffectivePresence combines the user's preference with their connections:
// offline without one, offline when invisible, idle when every connection is idle.
func EffectivePresence(userID ID, pref StatusPref, custom string, connections, idle int) Presence {
	p := Presence{UserID: userID, Status: PresenceOffline}
	if connections == 0 || pref == StatusInvisible {
		return p
	}
	switch {
	case pref == StatusDND:
		p.Status = PresenceDND
	case pref == StatusIdle || idle == connections:
		p.Status = PresenceIdle
	default:
		p.Status = PresenceOnline
	}
	p.CustomStatus = custom
	return p
}
