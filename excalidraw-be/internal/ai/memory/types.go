package memory

import (
	"time"

	"github.com/google/uuid"
)

const (
	KindSummary = "summary"
	KindRaw     = "raw"

	OwnerUser = "user"
	OwnerRoom = "room"
)

type MemoryEntry struct {
	ID        uuid.UUID
	OwnerType string // OwnerUser | OwnerRoom
	OwnerID   string
	TabID     *uuid.UUID
	Kind      string // KindSummary | KindRaw
	Content   string
	Embedding []float32
	Metadata  map[string]any
	CreatedAt time.Time

	// Populated by TopK, ignored on insert.
	Distance float64
}

type Owner struct {
	Type string
	ID   string
}

func UserOwner(userID string) Owner { return Owner{Type: OwnerUser, ID: userID} }
func RoomOwner(roomID string) Owner { return Owner{Type: OwnerRoom, ID: roomID} }
