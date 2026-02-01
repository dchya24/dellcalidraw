package room

import (
	"sync"
	"time"
)

// RoomManager manages all rooms in memory
type RoomManager struct {
	rooms map[string]*Room
	mu    sync.RWMutex
}

// NewRoomManager creates a new room manager
func NewRoomManager() *RoomManager {
	rm := &RoomManager{
		rooms: make(map[string]*Room),
	}
	return rm
}

// StartCleanup starts the background cleanup goroutine
func (rm *RoomManager) StartCleanup(interval, timeout time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for range ticker.C {
			rm.cleanupInactiveRooms(timeout)
		}
	}()
}

// cleanupInactiveRooms removes rooms that have been inactive for too long
func (rm *RoomManager) cleanupInactiveRooms(timeout time.Duration) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	for id, room := range rm.rooms {
		if room.IsInactive(timeout) && room.GetParticipantCount() == 0 {
			delete(rm.rooms, id)
		}
	}
}

// GetOrCreateRoom gets an existing room or creates a new one
func (rm *RoomManager) GetOrCreateRoom(roomID string) *Room {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if room, exists := rm.rooms[roomID]; exists {
		room.UpdateActivity()
		return room
	}

	room := NewRoom(roomID)
	rm.rooms[roomID] = room
	return room
}

// GetRoom gets a room by ID (returns nil if not found)
func (rm *RoomManager) GetRoom(roomID string) *Room {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	if room, exists := rm.rooms[roomID]; exists {
		room.UpdateActivity()
		return room
	}
	return nil
}

// DeleteRoom removes a room from memory
func (rm *RoomManager) DeleteRoom(roomID string) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	delete(rm.rooms, roomID)
}

// GetRoomCount returns the number of active rooms
func (rm *RoomManager) GetRoomCount() int {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	return len(rm.rooms)
}

// GetStats returns statistics about all rooms
func (rm *RoomManager) GetStats() map[string]interface{} {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	totalParticipants := 0
	for _, room := range rm.rooms {
		totalParticipants += room.GetParticipantCount()
	}

	return map[string]interface{}{
		"total_rooms":        len(rm.rooms),
		"total_participants": totalParticipants,
	}
}
