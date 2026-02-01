package room

import (
	"sync"
	"time"
)

// Element represents a whiteboard element (rectangle, ellipse, arrow, text, etc.)
type Element struct {
	ID      string                 `json:"id"`
	Type    string                 `json:"type"`
	X       float64                `json:"x"`
	Y       float64                `json:"y"`
	Width   float64                `json:"width,omitempty"`
	Height  float64                `json:"height,omitempty"`
	Angle   float64                `json:"angle,omitempty"`
	Stroke  string                 `json:"stroke,omitempty"`
	Background string              `json:"background,omitempty"`
	Fill    string                 `json:"fill,omitempty"`
	Data    map[string]interface{} `json:"data,omitempty"`
}

// User represents a participant in a room
type User struct {
	ID        string    `json:"id"`
	Username  string    `json:"username"`
	Color     string    `json:"color"`
	ConnID    string    `json:"connId"`
	JoinedAt  time.Time `json:"joinedAt"`
	LastSeen  time.Time `json:"lastSeen"`
	IsIdle    bool      `json:"isIdle"`
}

// Cursor represents the current cursor position of a user
type Cursor struct {
	UserID  string  `json:"userId"`
	X       float64 `json:"x"`
	Y       float64 `json:"y"`
	Updated time.Time `json:"updated"`
}

// Room represents a collaborative whiteboard room
type Room struct {
	ID           string
	Elements     []Element
	Participants map[string]*User  // userID -> User
	Cursors      map[string]*Cursor // userID -> Cursor
	CreatedAt    time.Time
	LastActivity time.Time
	mu           sync.RWMutex
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
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Participants[user.ID] = user
	r.LastActivity = time.Now()
}

// RemoveUser removes a user from the room
func (r *Room) RemoveUser(userID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.Participants, userID)
	delete(r.Cursors, userID)
	r.LastActivity = time.Now()
}

// GetParticipants returns a copy of the participants list
func (r *Room) GetParticipants() []*User {
	r.mu.RLock()
	defer r.mu.RUnlock()

	users := make([]*User, 0, len(r.Participants))
	for _, user := range r.Participants {
		users = append(users, user)
	}
	return users
}

// GetParticipantCount returns the number of participants
func (r *Room) GetParticipantCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.Participants)
}

// AddElements adds new elements to the room
func (r *Room) AddElements(elements []Element) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Elements = append(r.Elements, elements...)
	r.LastActivity = time.Now()
}

// UpdateElements updates existing elements in the room
func (r *Room) UpdateElements(elements []Element) {
	r.mu.Lock()
	defer r.mu.Unlock()

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
	r.mu.Lock()
	defer r.mu.Unlock()

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
	r.mu.RLock()
	defer r.mu.RUnlock()

	elements := make([]Element, len(r.Elements))
	copy(elements, r.Elements)
	return elements
}

// UpdateCursor updates a user's cursor position
func (r *Room) UpdateCursor(userID string, x, y float64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Cursors[userID] = &Cursor{
		UserID:  userID,
		X:       x,
		Y:       y,
		Updated: time.Now(),
	}
	r.LastActivity = time.Now()
}

// GetCursors returns a copy of the cursors map
func (r *Room) GetCursors() []*Cursor {
	r.mu.RLock()
	defer r.mu.RUnlock()

	cursors := make([]*Cursor, 0, len(r.Cursors))
	for _, cursor := range r.Cursors {
		cursors = append(cursors, cursor)
	}
	return cursors
}

// UpdateActivity updates the last activity timestamp
func (r *Room) UpdateActivity() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.LastActivity = time.Now()
}

// IsInactive checks if the room is inactive based on the given timeout
func (r *Room) IsInactive(timeout time.Duration) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return time.Since(r.LastActivity) > timeout
}

// HasCapacity checks if the room has capacity for more users
func (r *Room) HasCapacity(maxCapacity int) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.Participants) < maxCapacity
}
