package room

import (
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/you/excalidraw-be/internal/database"
)

type PersistenceManager struct {
	db          *database.PostgresClient
	pending     map[string][]database.ElementUpdate
	mu          sync.Mutex
	flushTicker *time.Ticker
	done        chan struct{}
}

func NewPersistenceManager(db *database.PostgresClient, flushInterval time.Duration) *PersistenceManager {
	pm := &PersistenceManager{
		db:      db,
		pending: make(map[string][]database.ElementUpdate),
		done:    make(chan struct{}),
	}

	pm.flushTicker = time.NewTicker(flushInterval)
	go pm.flushLoop()

	slog.Info("Persistence manager started", "flushInterval", flushInterval)
	return pm
}

func (pm *PersistenceManager) Stop() {
	pm.flushTicker.Stop()
	close(pm.done)
	pm.FlushAll()
	slog.Info("Persistence manager stopped")
}

func (pm *PersistenceManager) GetOrCreateRoomDBID(roomKey string) (string, error) {
	return pm.db.GetOrCreateRoom(roomKey)
}

func (pm *PersistenceManager) QueueElementUpdates(roomDBID string, elements []Element) {
	pm.mu.Lock()
	for _, elem := range elements {
		data, err := json.Marshal(elem)
		if err != nil {
			slog.Error("Failed to marshal element for persistence", "elementID", elem.ID, "error", err)
			continue
		}
		version := elem.Version
		if version == 0 {
			version = 1
		}
		pm.pending[roomDBID] = append(pm.pending[roomDBID], database.ElementUpdate{
			ElementID: elem.ID,
			Data:      data,
			Version:   version,
		})
	}
	pm.mu.Unlock()
}

func (pm *PersistenceManager) QueueElementDeletion(roomDBID string, elementIDs []string) {
	if len(elementIDs) == 0 {
		return
	}

	go func() {
		if err := pm.db.DeleteElements(roomDBID, elementIDs); err != nil {
			slog.Error("Failed to delete elements from database", "roomDBID", roomDBID, "error", err)
		}
	}()
}

func (pm *PersistenceManager) FlushRoom(roomDBID string) {
	pm.mu.Lock()
	updates := pm.pending[roomDBID]
	delete(pm.pending, roomDBID)
	pm.mu.Unlock()

	if len(updates) == 0 {
		return
	}

	if err := pm.db.BatchSaveElements(roomDBID, updates); err != nil {
		slog.Error("Failed to flush room elements", "roomDBID", roomDBID, "error", err)
		pm.mu.Lock()
		pm.pending[roomDBID] = append(updates, pm.pending[roomDBID]...)
		pm.mu.Unlock()
	}
}

func (pm *PersistenceManager) FlushAll() {
	pm.mu.Lock()
	allPending := pm.pending
	pm.pending = make(map[string][]database.ElementUpdate)
	pm.mu.Unlock()

	for roomDBID, updates := range allPending {
		if len(updates) == 0 {
			continue
		}
		if err := pm.db.BatchSaveElements(roomDBID, updates); err != nil {
			slog.Error("Failed to flush room elements", "roomDBID", roomDBID, "error", err)
		}
	}
}

func (pm *PersistenceManager) SaveRoomSnapshot(roomDBID string, elements []Element) {
	if len(elements) == 0 {
		return
	}

	rawElements := make([]json.RawMessage, 0, len(elements))
	for _, elem := range elements {
		data, err := json.Marshal(elem)
		if err != nil {
			slog.Error("Failed to marshal element for snapshot", "elementID", elem.ID, "error", err)
			continue
		}
		rawElements = append(rawElements, data)
	}

	go func() {
		if err := pm.db.SaveAllElementsRaw(roomDBID, rawElements); err != nil {
			slog.Error("Failed to save room snapshot", "roomDBID", roomDBID, "error", err)
		}
	}()
}

func (pm *PersistenceManager) LoadElements(roomDBID string) ([]Element, error) {
	rawElements, err := pm.db.GetRawElements(roomDBID)
	if err != nil {
		return nil, err
	}

	elements := make([]Element, 0, len(rawElements))
	for _, raw := range rawElements {
		var elem Element
		if err := json.Unmarshal(raw, &elem); err != nil {
			slog.Warn("Failed to unmarshal element from database, skipping", "error", err)
			continue
		}
		elements = append(elements, elem)
	}

	return elements, nil
}

func (pm *PersistenceManager) flushLoop() {
	for {
		select {
		case <-pm.flushTicker.C:
			pm.flushAllPending()
		case <-pm.done:
			return
		}
	}
}

func (pm *PersistenceManager) flushAllPending() {
	pm.mu.Lock()
	allPending := pm.pending
	pm.pending = make(map[string][]database.ElementUpdate)
	pm.mu.Unlock()

	for roomDBID, updates := range allPending {
		if len(updates) == 0 {
			continue
		}
		if err := pm.db.BatchSaveElements(roomDBID, updates); err != nil {
			slog.Error("Periodic flush failed", "roomDBID", roomDBID, "error", err)
			pm.mu.Lock()
			pm.pending[roomDBID] = append(updates, pm.pending[roomDBID]...)
			pm.mu.Unlock()
		} else {
			slog.Debug("Periodic flush completed", "roomDBID", roomDBID, "count", len(updates))
		}
	}
}
