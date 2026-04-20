package room

import (
	"log/slog"
	"sync"
	"time"

	"github.com/you/excalidraw-be/internal/database"
)

type RoomManager struct {
	rooms     map[string]*Room
	roomDBIDs map[string]string
	pm        *PersistenceManager
	mu        sync.RWMutex
}

func NewRoomManager() *RoomManager {
	rm := &RoomManager{
		rooms:     make(map[string]*Room),
		roomDBIDs: make(map[string]string),
	}
	return rm
}

func (rm *RoomManager) SetPersistenceManager(pm *PersistenceManager) {
	rm.pm = pm
	slog.Info("Persistence manager attached to room manager")
}

func (rm *RoomManager) HasPersistence() bool {
	return rm.pm != nil
}

func (rm *RoomManager) StartCleanup(interval, timeout time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for range ticker.C {
			rm.cleanupInactiveRooms(timeout)
		}
	}()
}

func (rm *RoomManager) cleanupInactiveRooms(timeout time.Duration) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	for id, room := range rm.rooms {
		if room.IsInactive(timeout) && room.GetParticipantCount() == 0 {
			if rm.pm != nil {
				if dbID, ok := rm.roomDBIDs[id]; ok {
					rm.pm.SaveRoomSnapshot(dbID, room.GetElements())
				}
			}
			delete(rm.rooms, id)
			delete(rm.roomDBIDs, id)
		}
	}
}

func (rm *RoomManager) GetOrCreateRoom(roomID string) *Room {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if room, exists := rm.rooms[roomID]; exists {
		room.UpdateActivity()
		return room
	}

	room := NewRoom(roomID)
	rm.rooms[roomID] = room

	if rm.pm != nil {
		go func() {
			dbID, err := rm.pm.GetOrCreateRoomDBID(roomID)
			if err != nil {
				slog.Error("Failed to create room in database", "roomKey", roomID, "error", err)
				return
			}
			rm.mu.Lock()
			rm.roomDBIDs[roomID] = dbID
			rm.mu.Unlock()

			elements, err := rm.pm.LoadElements(dbID)
			if err != nil {
				slog.Error("Failed to load elements from database", "roomID", roomID, "error", err)
				return
			}
			if len(elements) > 0 {
				room.Mu.Lock()
				room.Elements = elements
				room.Mu.Unlock()
				slog.Info("Loaded elements from database", "roomID", roomID, "count", len(elements))
			}
		}()
	}

	return room
}

func (rm *RoomManager) GetRoom(roomID string) *Room {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	if room, exists := rm.rooms[roomID]; exists {
		room.UpdateActivity()
		return room
	}
	return nil
}

func (rm *RoomManager) DeleteRoom(roomID string) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if rm.pm != nil {
		if dbID, ok := rm.roomDBIDs[roomID]; ok {
			go func() {
				if err := rm.pm.db.DeleteRoom(dbID); err != nil {
					slog.Error("Failed to delete room from database", "roomID", roomID, "error", err)
				}
			}()
		}
	}

	delete(rm.rooms, roomID)
	delete(rm.roomDBIDs, roomID)
}

func (rm *RoomManager) GetRoomDBID(roomID string) string {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	return rm.roomDBIDs[roomID]
}

func (rm *RoomManager) QueuePersistence(roomID string, added, updated []Element, deleted []string) {
	if rm.pm == nil {
		return
	}

	dbID := rm.GetRoomDBID(roomID)
	if dbID == "" {
		slog.Warn("No DB ID for room, skipping persistence", "roomID", roomID)
		return
	}

	if len(added) > 0 {
		rm.pm.QueueElementUpdates(dbID, added)
	}
	if len(updated) > 0 {
		rm.pm.QueueElementUpdates(dbID, updated)
	}
	if len(deleted) > 0 {
		rm.pm.QueueElementDeletion(dbID, deleted)
	}
}

func (rm *RoomManager) FlushRoom(roomID string) {
	if rm.pm == nil {
		return
	}

	dbID := rm.GetRoomDBID(roomID)
	if dbID == "" {
		return
	}

	rm.pm.FlushRoom(dbID)
}

func (rm *RoomManager) GetRoomCount() int {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	return len(rm.rooms)
}

func (rm *RoomManager) GetStats() map[string]interface{} {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	totalParticipants := 0
	for _, room := range rm.rooms {
		totalParticipants += room.GetParticipantCount()
	}

	stats := map[string]interface{}{
		"total_rooms":         len(rm.rooms),
		"total_participants":  totalParticipants,
		"persistence_enabled": rm.pm != nil,
	}

	return stats
}

func (rm *RoomManager) StopPersistence() {
	if rm.pm != nil {
		rm.mu.RLock()
		for roomID, room := range rm.rooms {
			dbID := rm.roomDBIDs[roomID]
			if dbID != "" {
				rm.pm.SaveRoomSnapshot(dbID, room.GetElements())
			}
		}
		rm.mu.RUnlock()
		rm.pm.Stop()
	}
}

func InitPersistence(db *database.PostgresClient, rm *RoomManager, flushInterval time.Duration) error {
	pm := NewPersistenceManager(db, flushInterval)
	rm.SetPersistenceManager(pm)
	return nil
}
