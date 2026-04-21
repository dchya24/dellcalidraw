# Backend Development Phases

## Overview

This document breaks down the backend development into detailed, manageable phases. Each phase builds upon the previous one and delivers specific functionality that can be tested and validated.

---

## MVP Phases (Phase 1-5)

### Phase 1: Foundation & Basic WebSocket
**Goal**: Establish project structure and basic real-time communication

**Deliverables**:
- Go project setup with proper module structure
- WebSocket server initialization using `gorilla/websocket`
- HTTP server using `go-chi/chi` with health endpoint
- Connection lifecycle management (connect/disconnect)
- Structured logging with `zap`
- Environment configuration with `viper`

**Technology Stack**:
- **HTTP Framework**: `github.com/go-chi/chi/v5`
- **WebSocket**: `github.com/gorilla/websocket`
- **Logging**: `go.uber.org/zap`
- **Config**: `github.com/spf13/viper`
- **UUID**: `github.com/google/uuid`

**Technical Tasks**:
- [ ] Initialize Go module: `go mod init github.com/you/excalidraw-be`
- [ ] Set up project structure (cmd/, internal/, pkg/)
- [ ] Create HTTP server with chi router
- [ ] Set up WebSocket upgrader with `gorilla/websocket`
- [ ] Create health check endpoint (`GET /health`)
- [ ] Implement connection event handlers
- [ ] Add structured logging (logrus/zap)
- [ ] Set up config management (viper)
- [ ] Add graceful shutdown handling

**Project Structure**:
```
excalidraw-be/
├── cmd/
│   └── server/
│       └── main.go           # Application entry point
├── internal/
│   ├── config/
│   │   └── config.go         # Configuration management
│   ├── websocket/
│   │   ├── handler.go        # WebSocket connection handler
│   │   └── upgrader.go       # WebSocket upgrader setup
│   ├── middleware/
│   │   └── logging.go        # HTTP middleware
│   └── models/
│       └── types.go          # Shared data structures
├── pkg/
│   └── util/
│       └── util.go           # Utility functions
├── go.mod
├── go.sum
├── .env                      # Environment variables
└── config.yaml               # Configuration file
```

**Example Code - WebSocket Upgrader Setup**:
```go
// internal/websocket/upgrader.go
package websocket

import (
    "github.com/gorilla/websocket"
    "log/slog"
)

var Upgrader = websocket.Upgrader{
    ReadBufferSize:  1024,
    WriteBufferSize: 1024,
    CheckOrigin: func(r *http.Request) bool {
        // Allow all origins in development, restrict in production
        return true
    },
}

// Connection represents a WebSocket connection
type Connection struct {
    ID     string
    Conn   *websocket.Conn
    Send   chan []byte
    RoomID string
    UserID string
}

// NewConnection creates a new WebSocket connection
func NewConnection(conn *websocket.Conn, roomID, userID string) *Connection {
    return &Connection{
        ID:     uuid.New().String(),
        Conn:   conn,
        Send:   make(chan []byte, 256),
        RoomID: roomID,
        UserID: userID,
    }
}
```

**Example Code - Health Check Endpoint**:
```go
// cmd/server/main.go
package main

import (
    "encoding/json"
    "net/http"

    "github.com/go-chi/chi/v5"
)

func healthHandler(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]string{
        "status": "healthy",
        "version": "1.0.0",
    })
}

func main() {
    r := chi.NewRouter()
    r.Get("/health", healthHandler)

    // WebSocket endpoint
    r.Get("/ws", websocketHandler)

    http.ListenAndServe(":8080", r)
}
```

**Success Criteria**:
- Client can connect to WebSocket server
- Health endpoint returns 200 OK with JSON response
- Connections are logged properly
- Graceful shutdown works (SIGTERM/SIGINT handling)

**Estimated Time**: 1-2 days

---

### Phase 2: In-Memory Room Management
**Goal**: Enable room creation, joining, and basic state management

**Technology Stack**:
- **Room ID Generation**: `github.com/oklog/ulid/v2` (10-character format)
- **Concurrency**: `sync.RWMutex` for thread-safe room access
- **Storage**: `map[string]*Room` in-memory

**Deliverables**:
- In-memory room storage with proper mutex locking
- Room creation with unique ID generation (ULID-based)
- Room join/leave functionality
- Room state management (elements array)
- Basic room activity tracking
- Auto-cleanup for inactive rooms (goroutine-based)

**Technical Tasks**:
- [ ] Create Room struct with thread-safe operations
- [ ] Implement RoomManager with `map[string]*Room` and `sync.RWMutex`
- [ ] Add room ID generation using ULID (10-character format)
- [ ] Implement `join_room` WebSocket message handler
- [ ] Implement `leave_room` WebSocket message handler
- [ ] Add room state structure (elements, participants)
- [ ] Implement inactive room cleanup goroutine (1 hour timeout)
- [ ] Add room capacity limits (configurable via config.yaml)

**Example Code - Room Structure**:
```go
// internal/room/room.go
package room

import (
    "sync"
    "time"
)

type Element struct {
    ID      string                 `json:"id"`
    Type    string                 `json:"type"`
    X       float64                `json:"x"`
    Y       float64                `json:"y"`
    // ... other element fields
    Data    map[string]interface{} `json:"data"`
}

type User struct {
    ID       string    `json:"id"`
    Username string    `json:"username"`
    Color    string    `json:"color"`
    ConnID   string    `json:"connId"`
    JoinedAt time.Time `json:"joinedAt"`
}

type Room struct {
    ID            string
    Elements      []Element
    Participants  map[string]*User  // userID -> User
    CreatedAt     time.Time
    LastActivity  time.Time
    mu            sync.RWMutex
}

func NewRoom(id string) *Room {
    return &Room{
        ID:           id,
        Elements:     make([]Element, 0),
        Participants: make(map[string]*User),
        CreatedAt:    time.Now(),
        LastActivity: time.Now(),
    }
}

func (r *Room) AddUser(user *User) {
    r.mu.Lock()
    defer r.mu.Unlock()
    r.Participants[user.ID] = user
    r.LastActivity = time.Now()
}

func (r *Room) RemoveUser(userID string) {
    r.mu.Lock()
    defer r.mu.Unlock()
    delete(r.Participants, userID)
    r.LastActivity = time.Now()
}

func (r *Room) GetParticipants() []*User {
    r.mu.RLock()
    defer r.mu.RUnlock()
    
    users := make([]*User, 0, len(r.Participants))
    for _, user := range r.Participants {
        users = append(users, user)
    }
    return users
}
```

**Example Code - Room Manager**:
```go
// internal/room/manager.go
package room

import (
    "sync"
    "time"
)

type RoomManager struct {
    rooms map[string]*Room
    mu    sync.RWMutex
}

func NewRoomManager() *RoomManager {
    rm := &RoomManager{
        rooms: make(map[string]*Room),
    }
    go rm.cleanupInactiveRooms()
    return rm
}

func (rm *RoomManager) GetOrCreateRoom(roomID string) *Room {
    rm.mu.Lock()
    defer rm.mu.Unlock()
    
    if room, exists := rm.rooms[roomID]; exists {
        room.LastActivity = time.Now()
        return room
    }
    
    room := NewRoom(roomID)
    rm.rooms[roomID] = room
    return room
}

func (rm *RoomManager) cleanupInactiveRooms() {
    ticker := time.NewTicker(10 * time.Minute)
    defer ticker.Stop()
    
    for range ticker.C {
        rm.mu.Lock()
        for id, room := range rm.rooms {
            if time.Since(room.LastActivity) > time.Hour {
                delete(rm.rooms, id)
            }
        }
        rm.mu.Unlock()
    }
}
```

**WebSocket Events**:
```go
// Message types
type WSMessage struct {
    Type    string                 `json:"type"`
    Payload map[string]interface{} `json:"payload"`
}

// Client → Server
type JoinRoomPayload struct {
    RoomID  string `json:"roomId"`
    Username string `json:"username"`
}

type LeaveRoomPayload struct {
    RoomID string `json:"roomId"`
}

// Server → Client
type RoomStatePayload struct {
    Elements     []Element `json:"elements"`
    Participants []*User   `json:"participants"`
}

type UserJoinedPayload struct {
    UserID   string `json:"userId"`
    Username string `json:"username"`
    Color    string `json:"color"`
}

type UserLeftPayload struct {
    UserID string `json:"userId"`
}
```

**Success Criteria**:
- Users can create/join rooms by ID
- Room state is maintained in memory with thread-safe access
- New users receive current room state on join
- Rooms auto-cleanup after 1 hour of inactivity
- Concurrent access handled properly with mutexes

**Estimated Time**: 2-3 days

---

### Phase 3: Real-Time Element Synchronization
**Goal**: Enable multiple users to draw together and see changes in real-time

**Deliverables**:
- Element broadcasting (create, update, delete)
- Delta updates (only send changed elements)
- Basic conflict resolution (last-write-wins for MVP)
- Optimized update frequency
- Element validation and sanitization

**Technical Tasks**:
- [ ] Implement element data structure
- [ ] Add `update_elements` WebSocket event
- [ ] Implement delta update logic
- [ ] Add element validation (prevent malicious data)
- [ ] Implement element CRUD operations
- [ ] Add rate limiting for element updates
- [ ] Implement basic conflict resolution
- [ ] Add element size limits per room

**WebSocket Events**:
```typescript
// Client → Server
update_elements: {
  roomId: string
  changes: {
    added?: Element[]
    updated?: Element[]
    deleted?: string[]  // element IDs
  }
}

// Server → Client
elements_updated: {
  userId: string
  changes: {
    added?: Element[]
    updated?: Element[]
    deleted?: string[]
  }
}
```

**Success Criteria**:
- Drawing changes sync to all users in room
- Only changed elements are transmitted (deltas)
- No data loss with concurrent edits
- Malicious data is rejected

**Estimated Time**: 3-4 days

---

### Phase 4: User Awareness (Cursors & Presence)
**Goal**: Users can see each other's cursors and online status

**Deliverables**:
- Real-time cursor position broadcasting
- Throttled cursor updates (10-20 times/sec)
- User presence tracking (online/idle/offline)
- Unique user colors and display names
- Participant list management

**Technical Tasks**:
- [ ] Add user identity management (session-based)
- [ ] Implement cursor position structure
- [ ] Add `cursor_move` WebSocket event
- [ ] Implement cursor throttling
- [ ] Add idle detection (timeout on no activity)
- [ ] Implement user color assignment
- [ ] Add participant count tracking
- [ ] Create `user_joined`/`user_left` broadcasts

**WebSocket Events**:
```typescript
// Client → Server
cursor_move: {
  roomId: string
  position: { x: number, y: number }
}

// Server → Client
cursor_updated: {
  userId: string
  username: string
  color: string
  position: { x: number, y: number }
}

user_joined: {
  userId: string
  username: string
  color: string
}

user_left: {
  userId: string
}
```

**Success Criteria**:
- Users see each other's cursors in real-time
- Cursors update smoothly (throttled appropriately)
- Participant list shows all active users
- Users appear offline after idle timeout

**Estimated Time**: 2-3 days

---

### Phase 5: Room Link Management & URL Routing
**Goal**: Shareable room links with auto-join functionality

**Deliverables**:
- Room URL generation with room ID parameter
- URL parameter parsing on client-side
- Auto-join on page load with room ID
- Optional: QR code generation for mobile sharing
- Clean URL handling

**Technical Tasks**:
- [ ] Implement URL structure (`?room={roomId}`)
- [ ] Add URL parsing on app initialization
- [ ] Auto-join room when URL parameter present
- [ ] Update URL without page reload
- [ ] Add "copy link" functionality (backend generates link)
- [ ] Optional: QR code generation endpoint
- [ ] Test link sharing across different devices

**API Endpoints**:
```typescript
// Optional: Backend generates shareable link
GET /api/rooms/:id/link
Response: { shareUrl: string, qrCode?: string }
```

**Success Criteria**:
- Users can share room links
- Opening link auto-joins the room
- Links work across devices
- QR code generation works (optional)

**Estimated Time**: 1-2 days

---

## MVP Completion Checklist

After Phase 5, the MVP is complete with:
- ✅ Real-time collaboration (drawing sync)
- ✅ User awareness (cursors, presence)
- ✅ Shareable room links
- ✅ In-memory room storage
- ✅ Guest access (no authentication)
- ✅ Single server deployment

---

## Post-MVP Phases (Phase 6-10)

### Phase 6: Selection & Interaction Awareness
**Goal**: Users can see what others are selecting/editing

**Deliverables**:
- Selected element broadcasting
- Visual indicator for multiple users editing same element
- Viewport position sync (optional)
- Interaction notifications

**Technical Tasks**:
- [ ] Add `selection_change` WebSocket event
- [ ] Track selected elements per user
- [ ] Broadcast selection changes to room
- [ ] Implement conflict visual feedback
- [ ] Optional: Add viewport sync

**WebSocket Events**:
```typescript
// Client → Server
selection_change: {
  roomId: string
  selectedElementIds: string[]
}

// Server → Client
selection_updated: {
  userId: string
  username: string
  selectedElementIds: string[]
}
```

**Success Criteria**:
- Users see what others have selected
- Visual feedback when multiple users select same element
- No performance degradation

**Estimated Time**: 2-3 days

---

### Phase 7: Performance Optimization & Scaling Prep
**Goal**: Optimize for higher concurrency and prepare for scaling

**Deliverables**:
- Message batching for high-frequency updates
- Advanced connection pooling
- Performance monitoring and metrics
- Rate limiting improvements
- Load testing infrastructure

**Technical Tasks**:
- [ ] Implement message batching (combine rapid updates)
- [ ] Add connection pooling optimizations
- [ ] Create metrics collection (active rooms, users, messages)
- [ ] Implement adaptive rate limiting
- [ ] Add performance monitoring (CPU, memory)
- [ ] Create load testing suite
- [ ] Optimize serialization/deserialization

**Metrics to Track**:
```typescript
{
  activeRooms: number
  activeConnections: number
  messagesPerSecond: number
  avgLatency: number
  memoryUsage: number
  cpuUsage: number
}
```

**Success Criteria**:
- Support 50+ concurrent users per room
- Handle 500+ messages per second
- Minimal latency (< 100ms)
- Memory usage stable under load

**Estimated Time**: 3-4 days

---

### Phase 8: Database Persistence ✅ COMPLETED (2026-04-20)
**Goal**: Room and user data persists across server restarts

**Deliverables**:
- PostgreSQL 16 integration with connection pooling
- Room state persistence with throttled 3-second batching
- Embedded migration system (golang-migrate + embed.FS)
- Automatic save/load on room events
- Graceful degradation when database unavailable

**Technical Tasks**:
- [x] Set up PostgreSQL with lib/pq driver
- [x] Design database schema (rooms, room_elements, room_files)
- [x] Create PostgresClient with connection pooling
- [x] Implement room save/load from database (UPSERT pattern)
- [x] Add throttled persistence (3-second interval)
- [x] Create embedded migration system
- [x] Add database connection pooling (25 max open, 5 idle)
- [x] Integrate persistence into RoomManager and WebSocket handlers
- [x] Add Docker Compose PostgreSQL service

**Actual Schema**:
```sql
CREATE TABLE rooms (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    key        VARCHAR(64) NOT NULL UNIQUE,
    name       VARCHAR(255),
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE room_elements (
    id         SERIAL PRIMARY KEY,
    room_id    UUID NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
    element_id VARCHAR(255) NOT NULL,
    version    INTEGER DEFAULT 1,
    data       JSONB NOT NULL,
    updated_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(room_id, element_id)
);

CREATE TABLE room_files (
    id         SERIAL PRIMARY KEY,
    room_id    UUID NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
    file_id    VARCHAR(255) NOT NULL,
    mime_type  VARCHAR(100),
    size       BIGINT,
    created_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(room_id, file_id)
);
```

**Files Created**:
- `internal/database/database.go` - PostgresClient
- `internal/database/migrate.go` - Embedded migrations
- `internal/database/repository.go` - CRUD operations
- `internal/room/persistence.go` - PersistenceManager

**Files Modified**:
- `internal/config/config.go` - DatabaseConfig
- `internal/room/manager.go` - Persistence integration
- `internal/websocket/handler.go` - Element persistence queuing
- `cmd/server/main.go` - DB initialization
- `docker-compose.yml` / `docker-compose.dev.yml` - PostgreSQL service

---

### Phase 9: File Storage (MinIO/S3) ✅ COMPLETED (2026-04-21)
**Goal**: Server-side file storage for image uploads in whiteboard rooms

**Deliverables**:
- MinIO/S3-compatible storage client
- HTTP file upload/download/delete endpoints
- WebSocket file event broadcasting
- Database file metadata tracking
- Docker Compose MinIO service

**Technical Tasks**:
- [x] Create storage client with `minio-go/v7` SDK
- [x] Add `StorageConfig` to config system
- [x] Implement HTTP file upload handler (multipart, max 50MB)
- [x] Implement HTTP file download handler (streaming)
- [x] Implement HTTP file delete handler
- [x] Add file metadata repository (database/files.go)
- [x] Add database migration for `storage_key` column
- [x] Add WebSocket `file_uploaded` / `file_deleted` events
- [x] Add MinIO service to Docker Compose (prod + dev)
- [x] Graceful degradation when storage unavailable

**API Endpoints**:
```
POST   /api/rooms/{roomId}/files          - Upload file (multipart)
GET    /api/rooms/{roomId}/files/{fileId}  - Download file
DELETE /api/rooms/{roomId}/files/{fileId}  - Delete file
GET    /api/rooms/{roomId}/files           - List room files
```

**WebSocket Events**:
```
Client → Server:
  file_uploaded: { roomId, fileId, url, mimeType, size, storageKey }
  file_deleted:  { roomId, fileId }

Server → Client:
  file_upload_confirmed: { fileId, url, mimeType, size, storageKey }
  file_uploaded:         { userId, username, fileId, url, mimeType, size }
  file_deleted:          { userId, fileId }
```

**Files Created**:
- `internal/storage/storage.go` - Storage client
- `internal/database/files.go` - File metadata repository
- `internal/database/migrations/000002_file_storage.up.sql` - Migration
- `internal/database/migrations/000002_file_storage.down.sql` - Rollback
- `internal/websocket/handler_files.go` - WebSocket file handlers
- `cmd/server/file_handlers.go` - HTTP file handlers

**Files Modified**:
- `internal/config/config.go` - StorageConfig
- `config.yaml` - Storage section
- `cmd/server/main.go` - Storage init, file routes
- `internal/websocket/handler.go` - File message routing
- `internal/websocket/types.go` - File message types
- `.env.example` - Storage env vars
- `docker-compose.yml` / `docker-compose.dev.yml` - MinIO service

**Dependencies Added**:
- `github.com/minio/minio-go/v7` - MinIO/S3 Go SDK

---

### Phase 10: User Authentication & Authorization ✅ COMPLETED (2026-04-21)
**Goal**: Persistent user accounts with secure authentication

**Deliverables**:
- User registration and login
- JWT-based authentication with refresh token rotation
- Protected routes with JWT middleware
- User profile management
- WebSocket token authentication

**Technical Tasks**:
- [x] Design user database schema (users + refresh_tokens tables)
- [x] Implement password hashing (bcrypt)
- [x] Create JWT token generation/validation (HS256)
- [x] Add registration endpoint
- [x] Add login endpoint
- [x] Create protected middleware (JWTMiddleware)
- [x] Add user profile endpoints (GET/PUT /me, GET /{id})
- [x] Implement session refresh tokens with rotation
- [x] Add WebSocket token auth via query parameter
- [ ] Implement OAuth flow (Google/GitHub) - deferred to future phase

**API Endpoints**:
```
POST /api/auth/register    - Create new account
POST /api/auth/login       - Login with email/password
POST /api/auth/logout      - Invalidate session
POST /api/auth/refresh     - Refresh JWT token

GET  /api/users/me         - Get current user profile (protected)
PUT  /api/users/me         - Update profile (protected)
GET  /api/users/{id}       - Get public user info (protected)
```

**WebSocket Authentication**:
```
ws://localhost:8080/ws?token=<JWT_ACCESS_TOKEN>
// Server validates token on connection, falls back to guest mode
```

**Files Created**:
- `internal/auth/auth.go` - Auth service (JWT, bcrypt)
- `internal/auth/middleware.go` - JWT middleware
- `internal/database/users.go` - User repository
- `internal/database/migrations/000003_user_auth.up.sql` - Migration
- `internal/database/migrations/000003_user_auth.down.sql` - Rollback
- `cmd/server/auth_handlers.go` - Auth HTTP handlers

**Files Modified**:
- `internal/config/config.go` - AuthConfig
- `config.yaml` - Auth section
- `cmd/server/main.go` - Auth init, routes
- `internal/websocket/handler.go` - Token auth, auth fields
- `.env.example` - Auth env vars

**Dependencies Added**:
- `github.com/golang-jwt/jwt/v5` - JWT library
- `golang.org/x/crypto` - Bcrypt password hashing

---

### Phase 11: Advanced Room Features
**Goal**: Enhanced room management and collaboration features

**Deliverables**:
- Private/password-protected rooms
- Room permissions (owner, editor, viewer)
- Room creation restrictions
- Room metadata and settings
- Version history and undo/redo

**Technical Tasks**:
- [ ] Implement password protection for rooms
- [ ] Create permission system (roles)
- [ ] Add room settings (title, description)
- [ ] Implement room ownership
- [ ] Create element change history
- [ ] Add undo/redo functionality
- [ ] Implement room templates
- [ ] Add room cloning

**Permission Levels**:
```typescript
enum RoomRole {
  OWNER = 'owner',      // Full control, can delete room
  EDITOR = 'editor',    // Can edit elements
  VIEWER = 'viewer'     // Read-only access
}

// Room permissions
{
  canEdit: boolean
  canInvite: boolean
  canDelete: boolean
  canChangeSettings: boolean
}
```

**API Endpoints**:
```typescript
// Room management
POST /api/rooms - Create room (auth required)
PUT /api/rooms/:id/settings - Update room settings
POST /api/rooms/:id/protect - Set password
DELETE /api/rooms/:id - Delete room (owner only)

// Permissions
PUT /api/rooms/:id/permissions - Update user permissions
GET /api/rooms/:id/permissions - Get room permissions

// History
GET /api/rooms/:id/history - Get element change history
POST /api/rooms/:id/undo - Undo last action
POST /api/rooms/:id/redo - Redo action
```

**Success Criteria**:
- Rooms can be password-protected
- Permission system works correctly
- Owners can manage room settings
- Version history tracks changes
- Undo/redo works across sessions

**Estimated Time**: 5-7 days

---

## Advanced Features Phases (Phase 12-16)

### Phase 12: Multi-Server Scaling
**Goal**: Scale horizontally across multiple server instances

**Deliverables**:
- Redis pub/sub for cross-server communication
- Load balancer configuration
- Sticky sessions for WebSocket
- Server health monitoring
- Graceful shutdown handling

**Technical Tasks**:
- [ ] Set up Redis for pub/sub
- [ ] Configure load balancer (Nginx/HAProxy)
- [ ] Implement sticky sessions
- [ ] Add server health checks
- [ ] Create graceful shutdown logic
- [ ] Implement cross-server room state sync
- [ ] Add server auto-scaling

**Architecture**:
```
Client → Load Balancer → [Server 1, Server 2, Server 3]
                         ↓
                       Redis (Pub/Sub)
```

**Success Criteria**:
- Multiple servers handle same rooms
- Zero downtime during deployments
- Auto-scaling based on load
- < 1 second failover time

**Estimated Time**: 7-10 days

---

### Phase 13: File Storage & Export
**Goal**: Server-side file storage and advanced export options

**Deliverables**:
- File upload to cloud storage (S3)
- Server-side rendering for exports
- Batch export functionality
- Export queue for large rooms
- File versioning

**Technical Tasks**:
- [ ] Integrate S3 or similar storage
- [ ] Implement file upload endpoint
- [ ] Add server-side rendering (Puppeteer)
- [ ] Create export queue (Bull/Agenda)
- [ ] Add export status tracking
- [ ] Implement file versioning
- [ ] Add CDN integration for exports

**API Endpoints**:
```typescript
// File operations
POST /api/rooms/:id/export - Request export
GET /api/rooms/:id/export/:id - Get export status
GET /api/rooms/:id/files - List saved files
POST /api/files/upload - Upload file to room
DELETE /api/files/:id - Delete file
```

**Success Criteria**:
- Users can upload files to rooms
- Server-side export works for complex rooms
- Export queue handles high volume
- Files are served via CDN

**Estimated Time**: 5-7 days

---

### Phase 14: Room Discovery & Directory
**Goal**: Public room directory with search and filtering

**Deliverables**:
- Public room listing
- Search by name/tags
- Room categories
- Featured/popular rooms
- Room analytics

**Technical Tasks**:
- [ ] Create room directory page
- [ ] Implement search functionality
- [ ] Add room tags/categories
- [ ] Create trending algorithm
- [ ] Add room analytics tracking
- [ ] Implement room recommendations
- [ ] Add reporting/moderation tools

**API Endpoints**:
```typescript
// Discovery
GET /api/rooms - List public rooms (paginated)
GET /api/rooms/search?q=query - Search rooms
GET /api/rooms/trending - Get trending rooms
GET /api/rooms/categories - List categories
GET /api/rooms/:id/analytics - Room statistics
```

**Success Criteria**:
- Users can discover public rooms
- Search returns relevant results
- Trending algorithm works correctly
- Analytics provide useful insights

**Estimated Time**: 4-5 days

---

### Phase 15: Advanced Collaboration Features
**Goal**: Enhanced collaboration with comments, chat, and more

**Deliverables**:
- In-room commenting system
- Real-time chat
- @mentions and notifications
- Video/audio integration (WebRTC)
- Collaborative cursors with labels

**Technical Tasks**:
- [ ] Implement comment system (per element)
- [ ] Add real-time chat
- [ ] Create @mention functionality
- [ ] Add notification system
- [ ] Integrate WebRTC for video/audio
- [ ] Add screen sharing
- [ ] Implement collaborative labels

**WebSocket Events**:
```typescript
// Comments
comment_add: { elementId, text, userId }
comment_delete: { commentId }

// Chat
chat_message: { roomId, userId, message, timestamp }

// Notifications
notification: { type, message, userId }
```

**API Endpoints**:
```typescript
// Comments
GET /api/rooms/:id/comments - Get all comments
POST /api/rooms/:id/comments - Add comment
DELETE /api/comments/:id - Delete comment

// Notifications
GET /api/notifications - Get user notifications
PUT /api/notifications/:id/read - Mark as read
```

**Success Criteria**:
- Users can comment on elements
- Real-time chat works smoothly
- @mentions trigger notifications
- Video/audio integration works

**Estimated Time**: 7-10 days

---

### Phase 16: Advanced Security & Compliance
**Goal**: Enterprise-grade security and GDPR compliance

**Deliverables**:
- Advanced rate limiting
- DDoS protection
- Content moderation
- GDPR compliance tools
- Audit logging
- Data export/deletion

**Technical Tasks**:
- [ ] Implement advanced rate limiting (per user/IP)
- [ ] Add DDoS protection
- [ ] Create content moderation system
- [ ] Implement GDPR data export
- [ ] Add GDPR data deletion
- [ ] Create comprehensive audit logs
- [ ] Add security headers (CSP, HSTS)
- [ ] Implement 2FA for accounts

**API Endpoints**:
```typescript
// GDPR compliance
GET /api/users/me/gdpr/export - Export all user data
DELETE /api/users/me/gdpr/delete - Request deletion
POST /api/users/me/2fa/enable - Enable 2FA
GET /api/audit-logs - Get audit logs
```

**Success Criteria**:
- Rate limiting prevents abuse
- DDoS attacks are mitigated
- GDPR export/deletion works
- Comprehensive audit logging
- 2FA implementation works

**Estimated Time**: 5-7 days

---

## Phase Summary

### MVP Completion (Phases 1-5)
**Total Estimated Time**: 11-17 days
**Delivers**: Functional real-time collaboration app with guest access

### Post-MVP (Phases 6-10)
**Total Estimated Time**: 25-35 days
**Delivers**: Persistent data, file storage, authentication, advanced features

### Advanced Features (Phases 12-16)
**Total Estimated Time**: 28-39 days
**Delivers**: Enterprise-ready, scalable, feature-rich platform

---

## Phase Dependencies

```
Phase 1 (Foundation)
    ↓
Phase 2 (Room Management)
    ↓
Phase 3 (Element Sync) ← Requires: Phase 2
    ↓
Phase 4 (User Awareness) ← Requires: Phase 2
    ↓
Phase 5 (Room Links) ← Requires: Phase 2
    ↓
[MVP COMPLETE]

Phase 6 (Selection) ← Requires: Phase 3, 4
    ↓
Phase 7 (Performance) ← Requires: Phase 3, 4
    ↓
Phase 8 (Database) ← Requires: Phase 7
    ↓
Phase 9 (File Storage) ← Requires: Phase 8
    ↓
Phase 10 (Auth) ← Requires: Phase 8
    ↓
Phase 11 (Advanced Rooms) ← Requires: Phase 8, 10
    ↓
Phase 12 (Scaling) ← Requires: Phase 8
    ↓
Phase 13 (File Export) ← Requires: Phase 9
    ↓
Phase 14 (Discovery) ← Requires: Phase 11
    ↓
Phase 15 (Collaboration) ← Requires: Phase 10
    ↓
Phase 16 (Security) ← Requires: Phase 10
```

---

## Recommendations

### For MVP (Phases 1-5):
- Focus on getting core features working
- Keep architecture simple
- Test thoroughly with multiple users
- Document API and WebSocket events

### For Post-MVP (Phases 6-10):
- Add database before authentication
- Implement caching layer early
- Create comprehensive test suite
- Set up CI/CD pipeline

### For Advanced Features (Phases 11-15):
- Plan scaling strategy early
- Use managed services when possible
- Implement monitoring and alerting
- Focus on user experience

---

## Next Steps

1. **Start with Phase 1**: Set up the project foundation
2. **Create task tracking**: Use GitHub Issues or similar
3. **Set up milestones**: Group phases into releases
4. **Define acceptance criteria**: Clear success metrics for each phase
5. **Plan testing**: Unit tests, integration tests, load tests
6. **Prepare deployment**: Staging environment for testing
