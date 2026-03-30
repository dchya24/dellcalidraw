package room

import (
	"sync"
	"time"
)

// Element represents a whiteboard element (rectangle, ellipse, arrow, text, etc.)
type Element struct {
	ID              string                 `json:"id"`
	Type            string                 `json:"type"`
	X               float64                `json:"x"`
	Y               float64                `json:"y"`
	Width           float64                `json:"width,omitempty"`
	Height          float64                `json:"height,omitempty"`
	Angle           float64                `json:"angle,omitempty"`
	StrokeColor     string                 `json:"strokeColor,omitempty"`
	BackgroundColor string                 `json:"backgroundColor,omitempty"`
	FillStyle       string                 `json:"fillStyle,omitempty"`
	StrokeWidth     int                    `json:"strokeWidth,omitempty"`
	StrokeStyle     string                 `json:"strokeStyle,omitempty"`
	Roughness       int                    `json:"roughness,omitempty"`
	Opacity         int                    `json:"opacity,omitempty"`
	Seed            int64                  `json:"seed,omitempty"`
	Version         int                    `json:"version,omitempty"`
	VersionNonce    int                    `json:"versionNonce,omitempty"`
	IsDeleted       bool                   `json:"isDeleted,omitempty"`
	GroupIds        []string               `json:"groupIds,omitempty"`
	FrameId         string                 `json:"frameId,omitempty"`
	BoundElements   []BoundElement         `json:"boundElements,omitempty"`
	Updated         int64                  `json:"updated,omitempty"`
	Link            string                 `json:"link,omitempty"`
	Locked          bool                   `json:"locked,omitempty"`
	Data            map[string]interface{} `json:"data,omitempty"`
}

// BoundElement represents an element bound to another element
type BoundElement struct {
	ID   string `json:"id"`
	Type string `json:"type"`
}

// User represents a participant in a room
type User struct {
	ID       string    `json:"id"`
	Username string    `json:"username"`
	Color    string    `json:"color"`
	ConnID   string    `json:"connId"`
	JoinedAt time.Time `json:"joinedAt"`
	LastSeen time.Time `json:"lastSeen"`
	IsIdle   bool      `json:"isIdle"`
}

// Cursor represents the current cursor position of a user
type Cursor struct {
	UserID  string    `json:"userId"`
	X       float64   `json:"x"`
	Y       float64   `json:"y"`
	Updated time.Time `json:"updated"`
}

// Room represents a collaborative whiteboard room
type Room struct {
	ID           string
	Elements     []Element
	Participants map[string]*User    // userID -> User
	Cursors      map[string]*Cursor  // userID -> Cursor
	SelectedIDs  map[string][]string // userID -> selected element IDs (Phase 6)
	CreatedAt    time.Time
	LastActivity time.Time
	Mu           sync.RWMutex
}

// NewRoom creates a new room with the given ID
func NewRoom(id string) *Room {
	return &Room{
		ID:           id,
		Elements:     make([]Element, 0),
		Participants: make(map[string]*User),
		Cursors:      make(map[string]*Cursor),
		CreatedAt:    time.Now(),
		LastActivity: time.Now(),
	}
}

// AddUser adds a user to the room
func (r *Room) AddUser(user *User) {
	r.Mu.Lock()
	defer r.Mu.Unlock()
	r.Participants[user.ID] = user
	r.LastActivity = time.Now()
}

// RemoveUser removes a user from the room
func (r *Room) RemoveUser(userID string) {
	r.Mu.Lock()
	defer r.Mu.Unlock()
	delete(r.Participants, userID)
	delete(r.Cursors, userID)
	r.LastActivity = time.Now()
}

// GetParticipants returns a copy of the participants list
func (r *Room) GetParticipants() []*User {
	r.Mu.RLock()
	defer r.Mu.RUnlock()

	users := make([]*User, 0, len(r.Participants))
	for _, user := range r.Participants {
		users = append(users, user)
	}
	return users
}

// GetParticipantCount returns the number of participants
func (r *Room) GetParticipantCount() int {
	r.Mu.RLock()
	defer r.Mu.RUnlock()
	return len(r.Participants)
}

// AddElements adds new elements to the room
func (r *Room) AddElements(elements []Element) {
	r.Mu.Lock()
	defer r.Mu.Unlock()
	r.Elements = append(r.Elements, elements...)
	r.LastActivity = time.Now()
}

// UpdateElements updates existing elements in the room
func (r *Room) UpdateElements(elements []Element) {
	r.Mu.Lock()
	defer r.Mu.Unlock()

	elementMap := make(map[string]Element)
	for _, elem := range r.Elements {
		elementMap[elem.ID] = elem
	}

	for _, elem := range elements {
		elementMap[elem.ID] = elem
	}

	r.Elements = make([]Element, 0, len(elementMap))
	for _, elem := range elementMap {
		r.Elements = append(r.Elements, elem)
	}
	r.LastActivity = time.Now()
}

// DeleteElements removes elements by their IDs
func (r *Room) DeleteElements(elementIDs []string) {
	r.Mu.Lock()
	defer r.Mu.Unlock()

	idSet := make(map[string]bool)
	for _, id := range elementIDs {
		idSet[id] = true
	}

	filtered := make([]Element, 0, len(r.Elements))
	for _, elem := range r.Elements {
		if !idSet[elem.ID] {
			filtered = append(filtered, elem)
		}
	}
	r.Elements = filtered
	r.LastActivity = time.Now()
}

// GetElements returns a copy of the elements list
func (r *Room) GetElements() []Element {
	r.Mu.RLock()
	defer r.Mu.RUnlock()

	elements := make([]Element, len(r.Elements))
	copy(elements, r.Elements)
	return elements
}

// UpdateCursor updates a user's cursor position
func (r *Room) UpdateCursor(userID string, x, y float64) {
	r.Mu.Lock()
	defer r.Mu.Unlock()
	r.Cursors[userID] = &Cursor{
		UserID:  userID,
		X:       x,
		Y:       y,
		Updated: time.Now(),
	}
	r.LastActivity = time.Now()
}

// GetSelectedIDs returns selected element IDs for a user
func (r *Room) GetSelectedIDs(userID string) []string {
	r.Mu.RLock()
	defer r.Mu.Unlock()

	if selected, exists := r.SelectedIDs[userID]; exists {
		return selected
	}
	return []string{}
}

// UpdateSelectedIDs updates a user's selected element IDs
func (r *Room) UpdateSelectedIDs(userID string, elementIDs []string) {
	r.Mu.RLock()
	defer r.Mu.RUnlock()

	// Make a copy to avoid modifying the original slice
	ids := make([]string, len(elementIDs))
	copy(ids, elementIDs)

	r.SelectedIDs[userID] = ids
	r.LastActivity = time.Now()
}

// ClearSelectedIDs removes all selected elements for a user
func (r *Room) ClearSelectedIDs(userID string) {
	r.Mu.RLock()
	defer r.Mu.RUnlock()

	delete(r.SelectedIDs, userID)
	r.LastActivity = time.Now()
}

// GetCursors returns a copy of the cursors map
func (r *Room) GetCursors() []*Cursor {
	r.Mu.RLock()
	defer r.Mu.RUnlock()

	cursors := make([]*Cursor, 0, len(r.Cursors))
	for _, cursor := range r.Cursors {
		cursors = append(cursors, cursor)
	}
	return cursors
}

// UpdateActivity updates the last activity timestamp
func (r *Room) UpdateActivity() {
	r.Mu.Lock()
	defer r.Mu.Unlock()
	r.LastActivity = time.Now()
}

// IsInactive checks if the room is inactive based on the given timeout
func (r *Room) IsInactive(timeout time.Duration) bool {
	r.Mu.RLock()
	defer r.Mu.RUnlock()
	return time.Since(r.LastActivity) > timeout
}

// HasCapacity checks if the room has capacity for more users
func (r *Room) HasCapacity(maxCapacity int) bool {
	r.Mu.RLock()
	defer r.Mu.RUnlock()
	return len(r.Participants) < maxCapacity
}
