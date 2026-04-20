# Next Phase Analysis

## 📊 Current Project Status

**Latest Completed Phase:** Phase 8 - PostgreSQL Database Integration (2026-04-20)

### ✅ What's Complete
- WebSocket connection with exponential backoff reconnection (1s→30s)
- Heartbeat/Ping-Pong system (10s interval, 10s timeout)
- Message queue for offline support (max 100 messages, auto-flush)
- Promise-based acknowledgment system (10s timeout)
- 4-state connection management (disconnected/connecting/connected/reconnecting)
- Auto re-join room on WebSocket reconnect
- Enhanced collaboration panel with visual connection states
- Backend ping/pong handlers
- Element synchronization with delta updates
- Cursor tracking and rendering with coordinate transformation
- Selection awareness with overlay visualization
- Auto-join from URL with invite dialog
- Conflict resolution UI panel
- **PostgreSQL database with connection pooling** (Phase 8)
- **Embedded migration system** (golang-migrate + embed.FS) (Phase 8)
- **Throttled persistence** (3-second batch UPSERT) (Phase 8)
- **Initial scene loading** from database on room join (Phase 8)
- **Graceful degradation** - runs without DB if unavailable (Phase 8)
- **Docker Compose** PostgreSQL 16 service (Phase 8)

### 📈 Overall Progress
- **Real-Time Collaboration**: ~75% Complete
- **Persistence**: ~50% Complete
- **Overall Project**: ~60% Complete

---

## 🎯 Critical Gaps Analysis

### 🔴 High Priority (Blockers for Production)

| Gap Category | Current State | Impact | Priority |
| -------------- | -------------- | ------- | --------- |
| **No File Storage** | Files only on client-side | Cannot share images across sessions | 🔴 Critical |
| **No Message Encryption** | Plaintext WebSocket messages | Privacy/security vulnerability | 🔴 Critical |

### ~~🔴 Critical~~ (Resolved in Phase 8)

| Gap Category | Previous State | Resolution |
| -------------- | -------------- | ----------- |
| ~~No Database Persistence~~ | ~~In-memory only, all data lost on restart~~ | PostgreSQL with UPSERT, 3s throttled persistence |

### 🟡 Medium Priority (Feature Complete)

| Gap Category | Current State | Impact | Priority |
| -------------- | -------------- | ------- | --------- |
| **Idle Detection** | No idle state tracking | Poor user presence awareness | 🟡 Medium |
| **User Following** | No viewport sync | Limited collaborative navigation | 🟡 Medium |
| **REST API** | Partial implementation | No HTTP access to data | 🟡 Medium |

### 🟢 Low Priority (Nice to Have)

- Connection quality indicator (latency, packet loss)
- Backend acknowledgment for critical messages
- Selective element broadcasting optimization
- Performance metrics and monitoring

---

## ~~🚀 Recommended Next Phase~~ (Completed)

### ~~**Phase 8: PostgreSQL Database Integration**~~ ✅ COMPLETED (2026-04-20)

**Why This Phase?**
1. **Critical for Production**: Current implementation loses all data on server restart
2. **Foundation for Persistence**: Required for all subsequent persistence features
3. **Security Requirement**: Cannot store sensitive room data without proper persistence
4. **User Experience**: Users expect their drawings to persist across sessions

**Estimated Effort:** 3-5 days
**Risk Level:** Medium (requires database setup and migration)
**Dependencies:** None

---

## 🚀 Recommended Next Phase

### **Phase 9: File Storage (S3/MinIO Integration)**

**Why This Phase?**
1. **Critical for Production**: Users cannot share images across sessions
2. **Depends on Phase 8**: Database schema already has `room_files` table ready
3. **User Experience**: Images are core to whiteboard usage

**Estimated Effort:** 3-5 days
**Risk Level:** Medium
**Dependencies:** Phase 8 (Complete)

---

## ✅ Phase 8: PostgreSQL Database Integration (COMPLETED)

**Date:** 2026-04-20

### 🎯 Objectives

1. Set up PostgreSQL database with proper schema
2. Implement connection pooling and migration system
3. Add room and element persistence with throttling
4. Implement initial scene loading from database
5. Ensure data recovery on server restart

### 🛠 Implementation Steps

#### Step 1: Database Setup
- [ ] Install PostgreSQL driver (`lib/pq` for Go)
- [ ] Configure PostgreSQL database connection
- [ ] Set up environment variables for database credentials
- [ ] Create database connection pool
- [ ] Test connection and error handling

#### Step 2: Schema Migration
- [ ] Create migration system (e.g., `golang-migrate` or custom)
- [ ] Write migration for `rooms` table
- [ ] Write migration for `room_elements` table
- [ ] Write migration for `room_files` table
- [ ] Add indexes on `room_id` columns
- [ ] Add cascading delete constraints
- [ ] Run initial migration

#### Step 3: Repository Layer
- [ ] Create `PostgresClient` struct with `sql.DB`
- [ ] Implement `BatchSaveElements()` with UPSERT
- [ ] Implement `GetElements()` by room_id
- [ ] Implement `GetRoom()` by room_id
- [ ] Implement `CreateRoom()` on room creation
- [ ] Implement `DeleteRoom()` with cascade
- [ ] Add transaction support for multi-operations

#### Step 4: Room Integration
- [ ] Add database connection to Room struct
- [ ] Implement `StartPersistence()` with 3-second timer
- [ ] Add element update channel in Room
- [ ] Throttle persistence to batch updates
- [ ] Handle connection errors gracefully
- [ ] Add logging for persistence operations

#### Step 5: Initial Scene Loading
- [ ] Implement `LoadInitialScene()` on room join
- [ ] Fetch all elements from database
- [ ] Convert database elements to Excalidraw format
- [ ] Send initial scene to new user
- [ ] Handle empty room scenario
- [ ] Add error handling for database failures

#### Step 6: Integration with Existing Code
- [ ] Update `Room.Create()` to save to database
- [ ] Update `Room.AddElements()` to queue persistence
- [ ] Update `Room.UpdateElements()` to queue persistence
- [ ] Update `Room.DeleteElements()` to queue persistence
- [ ] Update user join handler to load from database
- [ ] Update user leave handler to trigger final persistence

### 📊 Database Schema

```sql
-- Rooms table
CREATE TABLE rooms (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    key        VARCHAR(64) NOT NULL,
    name       VARCHAR(255),
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- Room elements (scene data)
CREATE TABLE room_elements (
    id         SERIAL PRIMARY KEY,
    room_id    UUID NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
    element_id VARCHAR(255) NOT NULL,
    version    INTEGER DEFAULT 1,
    data       JSONB NOT NULL,
    updated_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(room_id, element_id)
);

CREATE INDEX idx_room_elements_room_id ON room_elements(room_id);

-- Room files (images)
CREATE TABLE room_files (
    id         SERIAL PRIMARY KEY,
    room_id    UUID NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
    file_id    VARCHAR(255) NOT NULL,
    s3_key     VARCHAR(512) NOT NULL,
    size       BIGINT,
    mime_type  VARCHAR(100),
    created_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(room_id, file_id)
);

CREATE INDEX idx_room_files_room_id ON room_files(room_id);
```

### 🔧 Go Implementation

**PostgresClient Structure:**
```go
type PostgresClient struct {
    db *sql.DB
}

func NewPostgresClient(dsn string) (*PostgresClient, error) {
    db, err := sql.Open("postgres", dsn)
    if err != nil {
        return nil, err
    }

    // Connection pool settings
    db.SetMaxOpenConns(25)
    db.SetMaxIdleConns(5)
    db.SetConnMaxLifetime(5 * time.Minute)

    return &PostgresClient{db: db}, nil
}
```

**Batch Save Elements:**
```go
func (p *PostgresClient) BatchSaveElements(roomID string, updates []ElementUpdate) error {
    tx, err := p.db.Begin()
    if err != nil {
        return err
    }
    defer tx.Rollback()

    stmt, err := tx.Prepare(`
        INSERT INTO room_elements (room_id, element_id, data, version, updated_at)
        VALUES ($1, $2, $3, $4, NOW())
        ON CONFLICT (room_id, element_id)
        DO UPDATE SET data = $3, version = $4, updated_at = NOW()
    `)
    if err != nil {
        return err
    }
    defer stmt.Close()

    for _, u := range updates {
        _, err := stmt.Exec(roomID, u.ElementID, u.Data, u.Version)
        if err != nil {
            return err
        }
    }

    return tx.Commit()
}
```

**Throttled Persistence:**
```go
func (r *Room) StartPersistence(db *PostgresClient, interval time.Duration) {
    ticker := time.NewTicker(interval)
    pending := make([]ElementUpdate, 0)

    go func() {
        for {
            select {
            case update := <-r.elementCh:
                pending = append(pending, update)
            case <-ticker.C:
                if len(pending) > 0 {
                    if err := db.BatchSaveElements(r.ID, pending); err != nil {
                        log.Printf("Failed to save elements: %v", err)
                    } else {
                        pending = pending[:0]
                    }
                }
            }
        }
    }()
}
```

### 🧠 Technical Decisions

**Why PostgreSQL?**
- Mature, battle-tested relational database
- Excellent JSONB support for element data
- Strong ACID compliance for data integrity
- Good performance for read/write operations
- Widely supported with excellent Go drivers

**Why Throttled Persistence (3s)?**
- Balances data safety with performance
- Prevents excessive database writes during drawing
- Reduces database connection pressure
- Matches spec requirements for production-grade apps

**Why UPSERT Instead of INSERT?**
- Handles both new and existing elements
- Reduces code complexity
- More efficient than DELETE + INSERT
- Maintains element versions for conflict tracking

### ⚠️ Risks & Mitigations

| Risk | Impact | Mitigation |
| ----- | ------- | ----------- |
| **Database Connection Failure** | Cannot save/load data | Implement retry logic, graceful degradation |
| **Performance Degradation** | Slow element sync | Add indexing, optimize queries, consider caching |
| **Data Corruption** | Lost/damaged drawings | Add data validation, implement backup strategy |
| **Migration Failures** | Cannot update schema | Use transaction-backed migrations, rollback on failure |
| **Connection Pool Exhaustion** | Cannot handle concurrent users | Tune pool settings, monitor connections |

### ✅ Phase 8 Completion Status

- [x] All rooms persisted to PostgreSQL
- [x] Elements save with 3-second throttle
- [x] Initial scene loads correctly on room join
- [x] Data survives server restart
- [x] Database queries use UPSERT for efficiency
- [x] Connection pool configured (25 max open, 5 idle)
- [x] Graceful handling of database failures (runs without DB)
- [x] Migration system works for schema updates (embedded in binary)
- [x] Snapshot persistence on room cleanup and server shutdown

### 📁 Files Created/Modified (Phase 8)

**New Backend Files:**
- `internal/database/database.go` - PostgresClient with connection pooling
- `internal/database/migrate.go` - Embedded migration system
- `internal/database/migrations/000001_init_schema.up.sql` - Schema
- `internal/database/migrations/000001_init_schema.down.sql` - Rollback
- `internal/database/repository.go` - CRUD operations
- `internal/room/persistence.go` - PersistenceManager with throttled flush

**Modified Backend Files:**
- `internal/config/config.go` - Added DatabaseConfig
- `config.yaml` - Added database section
- `internal/room/manager.go` - Integrated persistence, room DB ID tracking
- `internal/websocket/handler.go` - Queues persistence on element changes
- `cmd/server/main.go` - DB init, migration, graceful shutdown

**Infrastructure:**
- `docker-compose.yml` - Added PostgreSQL 16 service
- `docker-compose.dev.yml` - Added PostgreSQL for dev

### 🔄 Follow-up Phases

After Phase 8 completion:
1. **Phase 9**: File Storage (S3/MinIO integration) ← **NEXT**
2. **Phase 10**: File Encryption (AES-GCM for files)
3. **Phase 11**: REST API Completion (CRUD endpoints)
4. **Phase 12**: Message Encryption (AES-GCM for WebSocket)
5. **Phase 13**: Idle Detection (User presence)
6. **Phase 14**: User Following (Viewport sync)

---

## 📝 Notes

### Why Not Start with Encryption?
While encryption is a critical security gap, database persistence should come first because:
1. **Data persistence is more fundamental** - Even encrypted data is lost without persistence
2. **Easier to test** - Persistence can be verified immediately, encryption requires complex testing
3. **Lower risk** - Database issues are easier to debug than encryption bugs
4. **Better user experience** - Users expect data persistence more than encryption initially

### Recommended Deployment Order
1. **Phase 8**: Database (Foundation)
2. **Phase 9-11**: File storage & API (Core features)
3. **Phase 12**: Encryption (Security hardening)
4. **Phase 13-14**: Advanced features (UX polish)

---

## 🎯 Conclusion

**Phase 8 - PostgreSQL Database Integration: COMPLETED**

The most critical gap (no data persistence) has been resolved. All room and element data is now persisted to PostgreSQL with 3-second throttled batching, and initial scene loading restores data on server restart.

**Recommended Next Phase: Phase 9 - File Storage (S3/MinIO)**

**Expected Outcome After Phase 9:**
- Real-time collaboration remains ~75% complete
- Persistence increases from ~50% to ~65% complete
- Overall project completion increases from ~60% to ~68% complete
- Application supports image/file sharing across sessions
