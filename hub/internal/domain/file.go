package domain

import "time"

type FilePurpose string

const (
	FileAvatar    FilePurpose = "avatar"
	FileGuildIcon FilePurpose = "guild_icon"
)

func (p FilePurpose) Valid() bool { return p == FileAvatar || p == FileGuildIcon }

// File is an uploaded file. Its bytes live in the blob store under
// StorageKey, which is derived from the content.
type File struct {
	ID           ID
	OwnerID      ID
	Purpose      FilePurpose
	SHA256       []byte
	Size         int64
	Mime         string
	OriginalName string
	Width        int
	Height       int
	StorageKey   string
	CreatedAt    time.Time
}

// FileURL is the same-origin download path; the name segment is cosmetic.
func FileURL(id ID, name string) string {
	if id == 0 {
		return ""
	}
	return "/files/" + id.String() + "/" + name
}
