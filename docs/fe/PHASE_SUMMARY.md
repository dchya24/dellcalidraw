# Whiteboard Project - Phase Summary

## ✅ Phase 10: User Authentication & Authorization Completed
**Date:** 2026-04-21

### 🛠 Features Implemented

#### 1. Auth Service (`internal/auth/auth.go`)
- JWT access token generation with HS256 signing
- Refresh token generation with rotation (old tokens revoked on use)
- Password hashing with bcrypt (DefaultCost)
- Token validation with claims extraction
- Configurable TTL: access 15min, refresh 7 days

#### 2. JWT Middleware (`internal/auth/middleware.go`)
- `JWTMiddleware` validates Bearer token from Authorization header
- Injects `userID`, `username`, `email` into request context
- Returns 401 for missing/invalid tokens
- `GetUserIDFromContext()` helper for downstream handlers

#### 3. Database Schema (`internal/database/migrations/000003_user_auth.up.sql`)
- `users` table: UUID PK, unique username, unique email, bcrypt password, avatar URL, timestamps
- `refresh_tokens` table: UUID PK, user FK (cascade), unique token, expiry, revoked flag
- Indexes on email, username, user_id, token for fast lookups

#### 4. User Repository (`internal/database/users.go`)
- `CreateUser()` - Register new user with hashed password
- `GetUserByEmail()` / `GetUserByUsername()` / `GetUserByID()` - Lookups
- `UpdateUserProfile()` - Update username and avatar
- `SaveRefreshToken()` - Persist refresh token
- `GetRefreshToken()` - Validate refresh token
- `RevokeRefreshToken()` - Revoke single token
- `RevokeAllUserTokens()` - Revoke all user tokens (security)
- `CleanExpiredTokens()` - Cleanup expired/revoked tokens

#### 5. Auth HTTP Handlers (`cmd/server/auth_handlers.go`)
- **POST `/api/auth/register`** - Create account
  - Validates username (3+ chars), email, password (8+ chars)
  - Checks for duplicate email/username
  - Returns JWT token pair + user profile
- **POST `/api/auth/login`** - Login
  - Email + password validation
  - Returns JWT token pair + user profile
- **POST `/api/auth/refresh`** - Refresh access token
  - Validates refresh token, revokes old, issues new pair
- **POST `/api/auth/logout`** - Logout
  - Revokes refresh token
- **GET `/api/users/me`** (protected) - Get current user profile
- **PUT `/api/users/me`** (protected) - Update username/avatar
- **GET `/api/users/{id}`** (protected) - Get public user info

#### 6. WebSocket Token Auth (`internal/websocket/handler.go`)
- Accept `?token=<JWT>` query parameter on WebSocket connect
- Validates token, extracts userID and username
- Authenticated users' identity carried into room join
- Falls back to guest mode for invalid/missing tokens

#### 7. Config (`internal/config/config.go`)
- `AuthConfig` with `secret_key`, `access_token_ttl`, `refresh_token_ttl`
- Environment variables: `EXCALIDRAW_AUTH_*`
- Defaults: 15min access, 7-day refresh

### 🧠 Technical Decisions & Challenges

**Decision 1: JWT over Session Cookies**
- Stateless authentication scales better
- No server-side session storage needed
- Token contains all needed claims

**Decision 2: Refresh Token Rotation**
- Old refresh token revoked when issuing new pair
- Prevents token reuse attacks
- Refresh tokens stored in database for revocation capability

**Decision 3: Bcrypt with DefaultCost**
- Industry standard for password hashing
- DefaultCost (10) provides good security/performance balance

**Decision 4: Graceful Degradation**
- Auth routes only registered when database available
- WebSocket accepts connections without tokens (guest mode)
- Backward compatible with existing unauthenticated flow

**Decision 5: Context-Based Auth Info**
- Middleware injects user info into request context
- Handlers extract via type assertion
- Clean separation between auth and business logic

### 📊 Comparison: Before vs After

| Feature | Before (Phase 9) | After (Phase 10) |
|---------|-------------------|-------------------|
| User Accounts | ❌ None | ✅ Register/login |
| Authentication | ❌ Guest only | ✅ JWT + guest fallback |
| Password Security | ❌ N/A | ✅ Bcrypt hashing |
| Token Management | ❌ N/A | ✅ Access + refresh rotation |
| Protected Routes | ❌ None | ✅ JWT middleware |
| User Profiles | ❌ None | ✅ CRUD endpoints |
| WebSocket Auth | ❌ None | ✅ Token query param |
| Token Revocation | ❌ N/A | ✅ Per-token + all-user |

### 📁 Files Created

**Backend (Go):**
- `excalidraw-be/internal/auth/auth.go` - Auth service (JWT, bcrypt)
- `excalidraw-be/internal/auth/middleware.go` - JWT middleware
- `excalidraw-be/internal/database/users.go` - User repository
- `excalidraw-be/internal/database/migrations/000003_user_auth.up.sql` - Migration
- `excalidraw-be/internal/database/migrations/000003_user_auth.down.sql` - Rollback
- `excalidraw-be/cmd/server/auth_handlers.go` - Auth HTTP handlers

### 📁 Files Modified

**Backend (Go):**
- `excalidraw-be/internal/config/config.go` - Added AuthConfig
- `excalidraw-be/config.yaml` - Added auth section
- `excalidraw-be/cmd/server/main.go` - Auth init, routes, hub update
- `excalidraw-be/internal/websocket/handler.go` - Token auth, auth fields
- `excalidraw-be/.env.example` - Auth env vars

**Dependencies:**
- `github.com/golang-jwt/jwt/v5` - JWT library
- `golang.org/x/crypto` - Bcrypt password hashing

### ✅ Build Verification

- ✅ Backend: `go build ./cmd/server` passes
- ✅ Backend: `go vet ./...` passes
- ✅ Backend: `go fmt ./...` passes

### ⏭️ Next Steps

- **Phase 11**: Advanced Room Features (password protection, roles, permissions)
- Frontend: Implement login/register UI
- Frontend: Store JWT tokens, add auth headers to API calls
- Frontend: Pass token to WebSocket connection

---

## ✅ Phase 9: File Storage (MinIO/S3) Completed
**Date:** 2026-04-21

### 🛠 Features Implemented

#### 1. Storage Client (`internal/storage/storage.go`)
- MinIO/S3-compatible storage client using `minio-go/v7` SDK
- Auto-creates bucket on startup if not exists
- Upload, download, delete, presigned URL operations
- Configurable endpoint, credentials, region, SSL
- Object stat/metadata retrieval

#### 2. Storage Configuration (`internal/config/config.go`)
- `StorageConfig` struct with all S3/MinIO parameters
- Environment variable overrides via `EXCALIDRAW_STORAGE_*`
- Defaults: `localhost:9000`, `minioadmin` credentials, `excalidraw-files` bucket

#### 3. Database Migration (`internal/database/migrations/000002_file_storage.up.sql`)
- Adds `storage_key` column to `room_files` table
- Index on `storage_key` for fast lookups
- Reversible down migration

#### 4. File Repository (`internal/database/files.go`)
- `SaveFileRecord()` - Persist file metadata (MIME type, size, storage key)
- `GetFileRecord()` - Retrieve file metadata by room and file ID
- `DeleteFileRecord()` - Remove file metadata from database
- `ListFileRecords()` - List all files for a room
- UPSERT pattern for file record persistence

#### 5. HTTP File Handlers (`cmd/server/file_handlers.go`)
- **POST `/api/rooms/{roomId}/files`** - Upload file (multipart form, max 50MB)
  - MIME type validation (image/* only)
  - Auto-detect content type from extension/magic bytes
  - UUID-based file ID generation
  - Storage key: `{roomId}/{fileId}`
  - Saves metadata to database
- **GET `/api/rooms/{roomId}/files/{fileId}`** - Download file
  - Streams file from MinIO to client
  - Cache-Control headers (24h)
  - Content-Type from storage metadata
- **DELETE `/api/rooms/{roomId}/files/{fileId}`** - Delete file
  - Removes from MinIO storage
  - Removes metadata from database
- **GET `/api/rooms/{roomId}/files`** - List room files
  - Returns all file metadata with URLs

#### 6. WebSocket File Events (`internal/websocket/handler_files.go`)
- `file_uploaded` - Client notifies server after successful upload
  - Broadcasts to other participants with user info
  - Confirms upload to sender
- `file_deleted` - Client notifies server after file deletion
  - Broadcasts deletion to all participants

#### 7. Docker Compose Updates
- MinIO service added to both `docker-compose.yml` and `docker-compose.dev.yml`
- MinIO Console on port 9001 (web UI)
- MinIO API on port 9000
- Health check using `mc ready local`
- Persistent volume for MinIO data
- Backend depends on MinIO health check
- Storage environment variables configured

### 🧠 Technical Decisions & Challenges

**Decision 1: MinIO over raw S3**
- MinIO provides S3-compatible API for local development
- Same SDK works for both MinIO and AWS S3
- Easy migration path to cloud storage

**Decision 2: Graceful Degradation**
- Server starts without storage (logs warning)
- File upload routes only registered when storage available
- Allows development without MinIO running locally

**Decision 3: Storage Key Pattern**
- `{roomId}/{fileId}` organizes files by room
- Enables bulk operations per room
- UUID file IDs prevent collisions

**Decision 4: Image-Only Upload**
- MIME type whitelist: `image/png`, `image/jpeg`, `image/gif`, `image/webp`, `image/svg+xml`, `image/bmp`
- Prevents malicious file uploads
- Matches Excalidraw's image element support

**Decision 5: Max File Size 50MB**
- Reasonable limit for whiteboard images
- Multipart form parsing with size enforcement
- Matches typical diagram/image requirements

### 📊 Comparison: Before vs After

| Feature | Before (Phase 8) | After (Phase 9) |
|---------|-------------------|-----------------|
| File Storage | ❌ None | ✅ MinIO/S3 |
| Image Upload | ❌ Not supported | ✅ HTTP multipart |
| File Download | ❌ Not supported | ✅ Streaming from storage |
| File Deletion | ❌ Not supported | ✅ Storage + DB cleanup |
| File Metadata | ❌ `room_files` unused | ✅ Full metadata tracking |
| WebSocket File Events | ❌ None | ✅ Upload/delete broadcast |
| MinIO Console | ❌ None | ✅ Port 9001 |

### 📁 Files Created

**Backend (Go):**
- `excalidraw-be/internal/storage/storage.go` - Storage client
- `excalidraw-be/internal/database/files.go` - File metadata repository
- `excalidraw-be/internal/database/migrations/000002_file_storage.up.sql` - Migration
- `excalidraw-be/internal/database/migrations/000002_file_storage.down.sql` - Rollback
- `excalidraw-be/internal/websocket/handler_files.go` - WebSocket file handlers
- `excalidraw-be/cmd/server/file_handlers.go` - HTTP file handlers

### 📁 Files Modified

**Backend (Go):**
- `excalidraw-be/internal/config/config.go` - Added StorageConfig
- `excalidraw-be/config.yaml` - Added storage section
- `excalidraw-be/cmd/server/main.go` - Storage init, file routes
- `excalidraw-be/internal/websocket/handler.go` - File message routing
- `excalidraw-be/internal/websocket/types.go` - File message types
- `excalidraw-be/.env.example` - Storage env vars

**Infrastructure:**
- `docker-compose.yml` - Added MinIO service
- `docker-compose.dev.yml` - Added MinIO service

**Dependencies:**
- `github.com/minio/minio-go/v7` - MinIO/S3 Go SDK

### ✅ Build Verification

- ✅ Backend: `go build ./cmd/server` passes
- ✅ Backend: `go vet ./...` passes
- ✅ Backend: `go fmt ./...` passes
- ✅ Frontend: `npm run build` passes
- ✅ No Go compilation errors
- ✅ No TypeScript errors

### ⏭️ Next Steps

- **Phase 10**: File Encryption (AES-GCM)
- **Phase 11**: REST API Completion (CRUD endpoints)
- Frontend: Integrate file upload UI with Excalidraw image elements
- Consider adding presigned URL support for direct browser-to-MinIO uploads

---

## ✅ Phase 8: PostgreSQL Database Integration (Backend) Completed
**Date:** 2026-04-20

### 🛠 Features Implemented

#### 1. PostgreSQL Database Client (`internal/database/database.go`)
- Connection pooling via `database/sql` (25 max open, 5 idle)
- Configurable via environment variables (`EXCALIDRAW_DATABASE_*`)
- Connection health check with `Ping()`
- Graceful `Close()` for shutdown

#### 2. Embedded Migration System (`internal/database/migrate.go`)
- SQL migrations embedded in Go binary via `embed.FS`
- Uses `golang-migrate/migrate` library
- Auto-applies on server startup
- Supports up/down migrations

#### 3. Database Schema (`internal/database/migrations/`)
- `rooms` table: UUID primary key, unique room key, name, timestamps
- `room_elements` table: JSONB element data, UPSERT on conflict, indexed by room_id
- `room_files` table: File metadata with room reference (ready for Phase 9)
- Cascading deletes: deleting a room removes all elements and files

#### 4. Repository Layer (`internal/database/repository.go`)
- `GetOrCreateRoom()` - Find or create room by key
- `BatchSaveElements()` - UPSERT multiple elements in single transaction
- `DeleteElements()` - Remove elements by ID in transaction
- `GetRawElements()` - Load all elements as raw JSON
- `SaveAllElementsRaw()` - Full snapshot replacement (used on cleanup)
- `DeleteRoom()` - Remove room with cascade

#### 5. Persistence Manager (`internal/room/persistence.go`)
- 3-second throttled batch persistence
- Pending queue per room with mutex protection
- Periodic flush loop (background goroutine)
- `FlushRoom()` for immediate persistence on user leave
- `FlushAll()` for graceful shutdown
- `SaveRoomSnapshot()` for inactive room cleanup
- `LoadElements()` for initial scene loading on room join

#### 6. Room Manager Integration (`internal/room/manager.go`)
- `roomDBIDs` map tracks mapping of room key → database UUID
- `QueuePersistence()` called after every element add/update/delete
- `FlushRoom()` called when user leaves room
- `StopPersistence()` flushes all rooms on server shutdown
- Initial scene loading from database when room is first accessed
- Snapshot persistence when inactive rooms are cleaned up

#### 7. WebSocket Handler Integration (`internal/websocket/handler.go`)
- Element updates queue persistence after applying changes
- User leave triggers room flush to database
- User disconnect triggers room flush

#### 8. Docker Compose Updates
- PostgreSQL 16 Alpine service with health check
- Backend depends on PostgreSQL health check
- Database environment variables configured
- Persistent volume for database data
- Both production and development compose files updated

### 🧠 Technical Decisions & Challenges

**Decision 1: Graceful Degradation**
- Server starts without database and runs in-memory-only mode
- Log warning instead of crash when DB unavailable
- Allows development without PostgreSQL running locally

**Decision 2: Throttled Persistence (3s)**
- Balances data safety with write performance
- Prevents excessive DB writes during active drawing
- Batches multiple element updates into single transaction
- Failed flushes re-queue for next cycle

**Decision 3: UPSERT Pattern**
- `INSERT ... ON CONFLICT DO UPDATE` handles both new and existing elements
- Eliminates need for separate create/update logic
- Maintains element versions for conflict tracking

**Decision 4: No Import Cycles**
- `database` package uses `json.RawMessage` (no `room.Element` dependency)
- `room/persistence` handles marshaling between `room.Element` and JSON
- Clean package dependency graph

**Decision 5: Embedded Migrations**
- SQL files compiled into binary via `go:embed`
- No external migration files needed at runtime
- Single binary deployment

### 📊 Comparison: Before vs After

| Feature | Before (Phase 7.1) | After (Phase 8) |
|---------|---------------------|-----------------|
| Data Persistence | ❌ In-memory only | ✅ PostgreSQL |
| Server Restart | ❌ All data lost | ✅ Data survives |
| Element Storage | ❌ Memory only | ✅ JSONB in PostgreSQL |
| Room Cleanup | ❌ Data destroyed | ✅ Snapshot to DB first |
| Migration System | ❌ None | ✅ Embedded in binary |
| Connection Pooling | ❌ N/A | ✅ 25 max / 5 idle |
| DB Failure Handling | ❌ N/A | ✅ Graceful degradation |

### 📁 Files Created

**Backend (Go):**
- `excalidraw-be/internal/database/database.go`
- `excalidraw-be/internal/database/migrate.go`
- `excalidraw-be/internal/database/migrations/000001_init_schema.up.sql`
- `excalidraw-be/internal/database/migrations/000001_init_schema.down.sql`
- `excalidraw-be/internal/database/repository.go`
- `excalidraw-be/internal/room/persistence.go`

### 📁 Files Modified

**Backend (Go):**
- `excalidraw-be/internal/config/config.go` - Added DatabaseConfig
- `excalidraw-be/config.yaml` - Added database section
- `excalidraw-be/internal/room/manager.go` - Full rewrite with persistence
- `excalidraw-be/internal/websocket/handler.go` - Persistence queuing
- `excalidraw-be/cmd/server/main.go` - DB init, migration, shutdown

**Infrastructure:**
- `docker-compose.yml` - Added PostgreSQL 16
- `docker-compose.dev.yml` - Added PostgreSQL 16

**Dependencies:**
- `github.com/lib/pq` - PostgreSQL driver
- `github.com/golang-migrate/migrate/v4` - Migration library

### ✅ Build Verification

- ✅ Backend: `go build ./cmd/server` passes
- ✅ Backend: `go vet ./...` passes
- ✅ Backend: `go fmt ./...` passes
- ✅ Frontend: `npm run build` passes
- ✅ No Go compilation errors
- ✅ No import cycle errors

### ⏭️ Next Steps

- **Phase 9**: File Storage (S3/MinIO integration for images)
- **Phase 10**: File Encryption (AES-GCM)
- **Phase 11**: REST API Completion (CRUD endpoints)
- Consider adding database connection monitoring/metrics

---

## 🔧 Phase 7.1: Enhanced WebSocket & Connection Stability
**Date:** 2026-03-31

### 🛠 Features Implemented

#### 1. Enhanced WebSocket Service (`src/services/websocket.ts`)

**Major Rewrite dengan Fitur Socket.io-like:**

- **Exponential Backoff Reconnection**:
  - Delay meningkat secara eksponensial: 1s → 2s → 4s → 8s... (max 30s)
  - Jitter acak (0-1000ms) untuk mencegah thundering herd
  - Maksimum 10 attempts (dari 5)
  
- **Heartbeat/Ping-Pong System**:
  - Interval: 10 detik
  - Timeout: 10 detik
  - Auto-reconnect jika tidak ada respons pong
  - Deteksi silent disconnects (WiFi drop, laptop sleep)

- **Message Queue (Offline Support)**:
  - Max 100 messages dalam queue
  - Auto-flush saat reconnect berhasil
  - Message retry dengan increment counter
  - Prevents data loss during temporary disconnections

- **Acknowledgment System (Socket.io emit)**:
  - `emit(type, payload)` mengembalikan Promise
  - 10 detik timeout untuk acknowledgment
  - Support request-response pattern
  - Error handling untuk failed acknowledgments

- **Connection State Management**:
  - 4 states: `disconnected` | `connecting` | `connected` | `reconnecting`
  - Real-time reconnect attempt counter
  - Connection quality monitoring

- **Improved Event System**:
  - `on(event, handler)` - Subscribe to events
  - `once(event, handler)` - One-time subscription
  - `emit(type, payload)` - Send with acknowledgment
  - `onError(handler)` - Error event subscription
  - Better unsubscribe functions

#### 2. Enhanced Room Service (`src/services/roomService.ts`)

**Perbaikan Major:**

- **Tab Visibility Bug Fix**:
  - Removed automatic disconnect saat tab hidden
  - Only disconnect saat `beforeunload` (page close)
  - Users tetap connected saat switch tabs
  - Solves: "Can't connect with same room in different tabs"

- **Auto Re-join Room**:
  - Automatic re-join room saat WebSocket reconnect berhasil
  - Users tidak perlu manual click "Join Room" lagi
  - Seamless reconnection experience

- **Heartbeat Integration**:
  - Heartbeat enabled secara default
  - Configurable via `enableHeartbeat` option
  - Better detection of zombie connections

- **Enhanced Error Handling**:
  - Error event subscription via `onError()`
  - Error propagation ke UI layer
  - Better error messages dengan context

- **Connection State API**:
  - `getConnectionState()` - Get current state
  - `getReconnectAttempts()` - Get attempt count
  - Real-time state updates via `onConnectionChange()`

- **Manual Reconnect Support**:
  - `reconnect()` method untuk force reconnect
  - Useful untuk "Retry" button di UI
  - Resets reconnect counter

#### 3. Enhanced Collaboration Panel (`src/components/CollaborationPanel.tsx`)

**UI Improvements:**

- **Visual Connection States**:
  - Connected: Green badge dengan WiFi icon
  - Connecting: Blue badge dengan animated spinner
  - Reconnecting: Yellow badge dengan "attempt X/10"
  - Disconnected: Gray badge dengan WiFiOff icon

- **Error Display**:
  - AlertCircle icon untuk error messages
  - Auto-dismiss setelah 5 detik
  - Red background untuk visibility

- **Reconnect Button**:
  - "Force Reconnect" button saat stuck di reconnecting
  - Manual control untuk users
  - Immediate feedback

- **Real-time Updates**:
  - Connection state update setiap 1 detik
  - Reconnect attempt counter live display
  - Better user feedback

#### 4. Backend Heartbeat Support (`excalidraw-be/internal/websocket/handler.go`)

**Server-side Enhancements:**

- **Ping/Pong Handlers**:
  - Respond ke client ping dengan pong
  - Accept client pong responses
  - Logging dengan emoji indicators
  - Better debugging

### 🧠 Technical Decisions & Challenges

**Challenge 1: Tab Switching Disconnect**
- **Problem**: Users disconnect saat switch tabs karena `visibilitychange` handler
- **Solution**: Removed visibility-based disconnect, hanya gunakan `beforeunload`
- **Result**: Users tetap connected saat multitasking

**Challenge 2: Silent Disconnects**
- **Problem**: WebSocket tidak detect disconnect saat WiFi drop atau laptop sleep
- **Solution**: Implementasi heartbeat dengan ping/pong setiap 10 detik
- **Result**: Auto-reconnect dalam 10-40 detik setelah connection loss

**Challenge 3: Data Loss During Reconnect**
- **Problem**: Messages yang dikirim saat offline hilang
- **Solution**: Message queue dengan auto-flush saat reconnect
- **Result**: 0% data loss untuk messages dalam queue (max 100)

**Challenge 4: Socket.io Library Unavailable di Go**
- **Problem**: No maintained Socket.io library untuk Go backend
- **Solution**: Re-implement Socket.io features di WebSocket native:
  - Event emitter pattern
  - Acknowledgment system
  - Room management
  - Heartbeat mechanism
- **Result**: Socket.io-like API tanpa pindah tech stack

### ⚡ Performance Improvements

- **Exponential backoff**: Mengurangi server load saat reconnect storm
- **Jitter**: Prevents thundering herd problem
- **Message batching**: Queue flush mengirim multiple messages sekaligus
- **Heartbeat**: Hanya 2 messages per menit (minimal overhead)

### 📊 Comparison: Before vs After

| Feature | Before | After |
|---------|--------|-------|
| Reconnection Delay | Linear 3s | Exponential 1s→30s |
| Max Reconnect Attempts | 5 | 10 |
| Heartbeat | ❌ None | ✅ 10s interval |
| Offline Messages | ❌ Lost | ✅ Queued (max 100) |
| Ack System | ❌ None | ✅ Promise-based |
| Connection State | Boolean | 4 states + counter |
| Tab Switching | ❌ Disconnect | ✅ Stay connected |
| Auto Re-join | ❌ Manual | ✅ Automatic |
| Error Events | ❌ None | ✅ Event-based |

### 🐛 Bug Fixes

1. **Tab Visibility Fix**: Users tidak lagi disconnect saat switch tabs
2. **Silent Disconnect**: Heartbeat mendeteksi connection loss yang tidak terdeteksi sebelumnya
3. **Reconnection Loop**: Fixed potential infinite loop saat max attempts reached
4. **Race Conditions**: Better handling untuk rapid connect/disconnect events

### 📁 Files Modified

**Frontend:**
- `src/services/websocket.ts` - Major rewrite (176 → 547 lines)
- `src/services/roomService.ts` - Enhanced dengan heartbeat & reconnection (256 lines)
- `src/components/CollaborationPanel.tsx` - UI improvements (282 lines)

**Backend:**
- `excalidraw-be/internal/websocket/handler.go` - Ping/pong handlers (lines 167-210)

### ✅ Build Verification

- ✅ Frontend: `npm run build` passes
- ✅ Backend: `go build ./cmd/server` passes
- ✅ No TypeScript errors
- ✅ No linting errors

### ⏭️ Next Steps

- Test multi-tab collaboration dengan enhanced WebSocket
- Monitor connection stability metrics
- Consider adding connection quality indicator (latency, packet loss)
- Implement backend acknowledgment untuk critical messages (optional)

---

## ✅ Phase 7: Real-Time Collaboration Frontend Integration Completed
**Date:** 2026-03-12

### 🛠 Features Implemented

- **Selection Awareness System** (`src/services/selectionService.ts`):
  - Real-time selection synchronization across users
  - Debounced selection updates (100ms) to minimize network traffic
  - Remote selection tracking with user color labels
  - Automatic cleanup when users leave

- **Selection Overlay Component** (`src/components/SelectionOverlay.tsx`):
  - Visual feedback showing which elements other users have selected
  - Colored borders around selected elements
  - Username labels for each selection
  - SVG overlay with proper z-index layering

- **Auto-Join from URL** (`src/components/Whiteboard.tsx`):
  - Auto-connect to room when URL contains `?room={roomId}` query parameter
  - Seamless sharing experience - users just open the link and are connected
  - Checks URL params on tab change for reconnection

- **Enhanced Conflict UI** (`src/components/Whiteboard.tsx`):
  - Improved conflict warnings showing which user made changes
  - Change count (e.g., "Alice made 3 changes")
  - Extended notification duration (4 seconds)
  - Participant lookup for accurate usernames

- **Updated Type Definitions** (`src/types/websocket.ts`):
  - Added `SelectionChangePayload` for client→server
  - Added `SelectionUpdatedPayload` for server→client
  - Added `ElementSelection` interface for remote selection tracking

### 🛠 Features Implemented

### 🛠 Features Implemented
- **WebSocket Connection Service** (`src/services/websocket.ts`):
  - WebSocket client with automatic reconnection (up to 5 attempts)
  - Message handler registration system
  - Connection state monitoring
  - Graceful disconnect handling

- **Room Service** (`src/services/roomService.ts`):
  - Room join/leave operations
  - Participant tracking and synchronization
  - Auto-join from URL query parameter (`?room={roomId}`)
  - Page unload handlers for proper cleanup
  - Username generation and persistence

- **Element Sync Service** (`src/services/elementSyncService.ts`):
  - Delta-based element synchronization (added/updated/deleted)
  - Element validation before sending to backend
  - Rate limiting integration
  - Local element cache management

- **Cursor Service** (`src/services/cursorService.ts`):
  - Real-time cursor position broadcasting
  - Throttled to 20 updates per second
  - Remote cursor tracking and rendering
  - 5-second timeout for inactive cursors

- **Room URL Utilities** (`src/utils/roomURL.ts`):
  - Parse room ID from URL parameters
  - Generate shareable room links
  - Copy room link to clipboard

- **Updated CollaborationPanel** (`src/components/CollaborationPanel.tsx`):
  - Real connection status indicators (connected/connecting/disconnected)
  - Live participant list with colors
  - Join/Leave room buttons
  - Copy room link functionality
  - Regenerate room ID

### 🧠 Technical Decisions & Challenges
- **Singleton pattern** for services to ensure single WebSocket connection
- **Event-driven architecture** using Set-based event listeners
- **Automatic reconnection** with exponential backoff
- **Page unload detection** using `beforeunload`, `unload`, and `visibilitychange` events
- **10-second timeout** in backend for faster disconnect detection

### ⚠️ Known Issues / TODOs
- Selection polling every 200ms may have slight delay between actual selection change and sync
- Multiple users selecting same element shows multiple borders (no conflict resolution UI yet)
- No viewport position sync (optional feature from Phase 6)

### 🏗️ Next Steps
- Test with multiple concurrent users (3+)
- Add conflict resolution UI for concurrent edits to same element
- Implement viewport position sync (optional)
- Performance testing with 100+ elements and multiple users
- **ALL PHASE 3-6 INTEGRATION COMPLETE** 🎉

---

## 🐛 Bug Fixes for Phase 7 Integration
**Date:** 2026-03-12

### Issues Fixed
1. **TypeScript error with selectedElementIds**: Fixed by handling both array and Set-like object types from Excalidraw API
2. **Selection not syncing**: Implemented selectionService with proper debouncing and WebSocket integration
3. **Auto-join not working**: Uncommented and implemented URL parameter checking for room auto-connection
4. **Generic conflict warnings**: Enhanced to show specific username and change count

---

## ✅ Phase 6: Real-Time Collaboration Backend Integration Completed
**Date:** 2026-03-12

### 🛠 Features Implemented

- **WebSocket Connection Service** (`src/services/websocket.ts`):
  - WebSocket client with automatic reconnection (up to 5 attempts)
  - Message handler registration system
  - Connection state monitoring
  - Graceful disconnect handling

- **Room Service** (`src/services/roomService.ts`):
  - Room join/leave operations
  - Participant tracking and synchronization
  - Auto-join from URL query parameter (`?room={roomId}`)
  - Page unload handlers for proper cleanup
  - Username generation and persistence

- **Element Sync Service** (`src/services/elementSyncService.ts`):
  - Delta-based element synchronization (added/updated/deleted)
  - Element validation before sending to backend
  - Rate limiting integration
  - Local element cache management

- **Cursor Service** (`src/services/cursorService.ts`):
  - Real-time cursor position broadcasting
  - Throttled to 20 updates per second
  - Remote cursor tracking and rendering
  - 5-second timeout for inactive cursors

- **Room URL Utilities** (`src/utils/roomURL.ts`):
  - Parse room ID from URL parameters
  - Generate shareable room links
  - Copy room link to clipboard

- **Updated CollaborationPanel** (`src/components/CollaborationPanel.tsx`):
  - Real connection status indicators (connected/connecting/disconnected)
  - Live participant list with colors
  - Join/Leave room buttons
  - Copy room link functionality
  - Regenerate room ID

### 🧠 Technical Decisions & Challenges
- **Singleton pattern** for services to ensure single WebSocket connection
- **Event-driven architecture** using Set-based event listeners
- **Automatic reconnection** with exponential backoff
- **Page unload detection** using `beforeunload`, `unload`, and `visibilitychange` events
- **10-second timeout** in backend for faster disconnect detection

### ⚠️ Known Issues / TODOs
- ~~Element sync not yet integrated with Excalidraw canvas (needs onChange handler integration)~~ ✅ Fixed in Phase 7
- ~~Cursor rendering component not yet implemented~~ ✅ Fixed in Phase 7
- ~~No visual feedback for element conflicts yet~~ ✅ Fixed in Phase 7

### 🐛 Bug Fixes
1. **Participant list not updating**: Fixed by adding new participants to local state when `user_joined` event received
2. **User not appearing in own list**: Fixed by properly handling `room_state` event
3. **Participants not removed on refresh**: Fixed by implementing page unload handlers
4. **WebSocket hijacker error**: Fixed by moving `/ws` route before `AllowContentType` middleware

### ⏭️ Next Steps
- ~~Integrate element sync with Excalidraw `onChange` handler~~ ✅ Done in Phase 7
- ~~Implement remote cursor rendering component~~ ✅ Done in Phase 7
- ~~Add conflict resolution UI~~ ✅ Enhanced in Phase 7
- Test with multiple concurrent users
- ~~INTEGRATION INCOMPLETE~~ ✅ Complete in Phase 7

---

## ✅ Phase 5: Room Link Management & URL Routing Completed
**Date:** 2026-03-12

### 🛠 Features Implemented
- **Room Link Generation** (`cmd/server/main.go:127-143`):
  - HTTP endpoint: `GET /api/rooms/:id/link`
  - Returns shareable URL format: `{origin}?room={roomId}`
  - Supports frontend room link generation

- **WebSocket Room Link** (`internal/websocket/handler.go:145-175`):
  - `get_room_link` WebSocket message
  - Returns share URL + optional QR code
  - Client can request room link via WebSocket

- **Message Types** (`internal/websocket/types.go:90-102`):
  - `GetRoomLinkPayload` - Request room link
  - `RoomLinkPayload` - Response with shareUrl and qrCode

### 🧠 Technical Decisions & Challenges
- **Simple HTTP endpoint**: Uses chi URL params to generate shareable URLs
- **WebSocket optional**: Room link can be retrieved via WebSocket or HTTP
- **No QR code**: Optional Phase 5 feature not yet implemented (low priority)
- **Client-side URL parsing**: Frontend parses `?room={roomId}` from URL (frontend responsibility)

### ⚠️ Known Issues / TODOs
- QR code generation not implemented (optional for Phase 5)
- No auto-join on page load (frontend responsibility)
- Frontend needs to implement: auto-join from URL parameter

### 🏗️ Next Steps
- Phase 6: Selection & Interaction Awareness

---

## ✅ Phase 6: Selection & Interaction Awareness Completed
**Date:** 2026-03-12

### 🛠 Features Implemented
- **Selection Tracking** (`internal/room/room.go:44`):
  - Added `SelectedIDs` map[string][]string to Room struct
  - Per-user selection tracking (userID → selected element IDs)
  - Thread-safe with mutex protection

- **Room Selection Methods** (`internal/room/room.go:195-220`):
  - `UpdateSelectedIDs(userID, elementIDs)` - Update user selections
  - `GetSelectedIDs(userID)` - Get user's selections
  - `ClearSelectedIDs(userID)` - Clear user selections

- **WebSocket Event** (`internal/websocket/handler.go:177-210`):
  - `selection_change` event handler
  - Broadcasts selection updates to all participants
  - Includes user info (userId, username, color)

- **Message Types** (`internal/websocket/types.go:93-110`):
  - `SelectionChangePayload` - Client sends selection changes
  - `SelectionUpdatedPayload` - Server broadcasts updates

### 🧠 Technical Decisions & Challenges
- **Map-based tracking**: Using `map[string][]string` for O(1) lookup per user
- **Broadcast to all**: Selection updates sent to all participants including sender
- **No conflict resolution**: Simple broadcasting (future phases may add OT/CRDT)
- **Thread-safe**: All room operations protected by `sync.RWMutex`

### ⚠️ Known Issues / TODOs
- No automatic conflict resolution (manual resolution required)
- No visual feedback mechanism yet (frontend responsibility)
- No viewport sync (optional Phase 6 feature)

### 🏗️ Next Steps
- Implement visual feedback for multiple users editing same element
- Add viewport position sync (optional)
- Phase 7: Performance Optimization & Scaling Prep

---

## ✅ Backend Phase 3: Real-Time Element Synchronization Completed
**Date:** 2026-03-12

### 🛠 Features Implemented
- **Element Synchronization** (`internal/websocket/handler.go:278-341`):
  - Delta-based element updates (added, updated, deleted)
  - Element validation before processing
  - Rate limiting (20 msg/sec, 100 msg/10sec window)
  - Element count limit (5000 max per room)

- **Element Validation** (`internal/room/validator.go`):
  - Element type validation (rectangle, ellipse, arrow, line, freedraw, text, image)
  - Coordinate range validation (0-100000)
  - Size validation (0-100000 for width/height)
  - Data size validation (max 10KB per element)
  - Text length validation (max 1000 characters)
  - Element count validation (max 5000 per room)

- **Element CRUD Operations** (`internal/room/room.go:101-148`):
  - Add elements to room
  - Update existing elements (last-write-wins)
  - Delete elements by ID
  - Get elements (thread-safe copy)

### 🧠 Technical Decisions & Challenges
- **Rate limiting**: Two-tier system (per-second and per-window) prevents spam while allowing bursts
- **Last-write-wins**: Simple conflict resolution for MVP (future phases will add OT/CRDT)
- **Delta updates**: Only send changed elements (added/updated/deleted) to minimize bandwidth
- **Thread-safe operations**: All room operations protected by `sync.RWMutex`
- **Broadcast to all participants**: Elements updated to all users except sender

### ⚠️ Known Issues / TODOs
- No conflict resolution UI for concurrent edits to same element
- Element validation is basic (no deep inspection of Data field)
- No element-level permissions (any user can edit any element)

### 🏗️ Next Steps
- Phase 6: Selection & Interaction Awareness
- Add conflict resolution UI
- Implement operational transformation or CRDT for production

---

## ✅ Backend Phase 4: User Awareness (Cursors & Presence) Completed
**Date:** 2026-03-12

### 🛠 Features Implemented
- **Cursor Tracking** (`internal/websocket/handler.go:343-392`):
  - Real-time cursor position broadcasting
  - Throttled to 20 updates per second
  - Auto-assign user colors (10 predefined colors)
  - Silent skip for throttled updates

- **Cursor Management** (`internal/room/room.go:160-183`):
  - Cursor storage per user in room
  - Update cursor position
  - Get cursors (thread-safe copy)
  - Auto-removed when user leaves room

- **User Presence** (`internal/websocket/handler.go:185-276`):
  - User join notifications (user_joined event)
  - User leave notifications (user_left event)
  - Participant list in room_state
  - Auto-remove user from room on disconnect

- **Cursor Rate Limiting** (`internal/websocket/ratelimit.go:96-128`):
  - Separate rate limiter for cursor updates
  - 20 updates per second max
  - Time-based throttling (min interval calculation)

### 🧠 Technical Decisions & Challenges
- **Separate cursor rate limiter**: Higher frequency allowed than element updates (20 vs 20/sec same, but cursor is lightweight)
- **Auto-assign colors**: Simple color assignment (will make random/color cycling in future)
- **Silent throttle**: Cursor updates are silently skipped when rate-limited to avoid spam
- **5-second timeout**: Cursors removed after 5 seconds of inactivity (cleanup in backend)

### ⚠️ Known Issues / TODOs
- User colors are not random yet (using last color in array)
- No idle detection for user presence (only cursor timeout)
- No user avatar/profile images
- No user status indicators (online/away/offline)

### 🏗️ Next Steps
- Implement idle detection for user presence
- Add random color generation for users
- Phase 6: Selection awareness (what users are selecting/editing)

---

## ✅ Phase 5: UI/UX Polish & Advanced Features Completed
**Date:** 2026-01-01

### 🛠 Features Implemented
- Project setup dengan Vite + React + TypeScript
- Tailwind CSS integration via `@tailwindcss/vite` plugin
- Fullscreen Excalidraw canvas component
- `excalidrawAPI` ref untuk kontrol programatik

### 🧠 Technical Decisions & Challenges
- Menggunakan Vite 7.x dengan React template TypeScript
- Tailwind CSS v4 dengan plugin vite (bukan postcss config lama)
- Struktur folder: `src/components/` untuk komponen UI

### ⚠️ Known Issues / TODOs
- Node.js version warning (requires 20.19+ atau 22.12+, current 20.17.0)
- Excalidraw bundle size cukup besar (~1.3MB)

### ⏭️ Next Steps
- Implementasi Multi-Tab System dengan Zustand

---

## ✅ Phase 2: Multi-Tab Management (Core Logic) Completed
**Date:** 2026-01-01

### 🛠 Features Implemented
- **Zustand Store** (`useWhiteboardStore.ts`):
  - State management untuk tabs array dan activeTabId
  - Actions: addTab, removeTab, renameTab, setActiveTab, saveTabState
  - Helper: getActiveTab untuk akses tab aktif
  
- **TabBar Component** (`TabBar.tsx`):
  - UI tab bar di bawah canvas (spreadsheet-style)
  - Add tab button dengan icon Plus
  - Close tab button (X) dengan hover visibility
  - Double-click to rename tab (inline editing)
  - Visual indicator untuk active tab
  
- **Tab Switching Logic**:
  - Auto-save state sebelum switch tab
  - Load elements dari tab baru via `updateScene()`
  - Reset undo history dengan `api.history.clear()`
  - Real-time state sync via `onChange` handler

### 🧠 Technical Decisions & Challenges
- Tab bar diletakkan di bawah canvas untuk konsistensi dengan spreadsheet apps (Excel, Google Sheets)
- Menggunakan `nanoid` untuk generate unique tab IDs
- State disimpan per-tab: elements, appState, files
- Undo history di-clear saat switch tab untuk mencegah cross-tab undo

### ⚠️ Known Issues / TODOs
- Belum ada persistence (localStorage) - data hilang saat refresh
- Belum ada konfirmasi saat delete tab dengan konten

### ⏭️ Next Steps
- Phase 3: Persistence & File Operations (localStorage auto-save, export/import)

---

## ✅ Phase 3: Persistence & File Operations Completed
**Date:** 2026-01-01

### 🛠 Features Implemented
- **Auto-save dengan Zustand Persist**:
  - State otomatis tersimpan ke localStorage
  - Data tabs dan activeTabId di-persist
  - Restore otomatis saat refresh browser
  
- **Toolbar Component** (`Toolbar.tsx`):
  - Import button untuk load file .excalidraw/.json
  - Export dropdown menu dengan 3 opsi:
    - Export JSON (.excalidraw) - semua tabs
    - Export PNG - tab aktif saja
    - Export SVG - tab aktif saja
  
- **Store Actions Baru**:
  - `loadFromFile()` - untuk import data dari file
  - `getExportData()` - untuk export semua tabs

### 🧠 Technical Decisions & Challenges
- Menggunakan `zustand/middleware/persist` untuk auto-save (lebih reliable dari manual debounce)
- Export PNG/SVG menggunakan `exportToBlob` dan `exportToSvg` dari @excalidraw/excalidraw
- Toolbar diposisikan absolute di kanan atas canvas dengan z-index tinggi

### ⚠️ Known Issues / TODOs
- Belum ada drag-and-drop untuk import (hanya via button)

### ⏭️ Next Steps
- Phase 4: Real-time Collaboration Setup

---

## 🔧 Phase 3 Bug Fixes
**Date:** 2026-01-01

### Issues Fixed
1. **Auto-save tidak bekerja**:
   - Root cause: Zustand hydration belum selesai saat Excalidraw render
   - Fix: Tunggu hydration selesai sebelum render Excalidraw (`isReady` state)

2. **Import .excalidraw tidak bekerja**:
   - Root cause: Export format tidak sesuai dengan native Excalidraw
   - Fix: Export menggunakan format native `{type: "excalidraw", elements, appState, files}`

3. **TypeError collaborators.forEach**:
   - Root cause: `appState.collaborators` adalah `Map` yang tidak bisa di-serialize ke JSON
   - Fix: Remove `collaborators` dari appState sebelum save (`const { collaborators, ...safeAppState } = appState`)

4. **Infinite loop saat menggambar**:
   - Root cause: `onChange` → `saveTabState` → `activeTab` berubah → effect re-run → `updateScene` → `onChange`
   - Fix: Gunakan `initialData` prop instead of `updateScene` dalam effect, dan load hanya sekali saat hydration

---

## ✅ Phase 4: Real-time Collaboration Setup Completed
**Date:** 2026-01-31

### 🛠 Features Implemented
- **Room System**:
  - Setiap tab sekarang memiliki `roomId` unik (10-character string)
  - Room ID di-generate otomatis saat tab dibuat
  - Room ID tersimpan bersama state tab (persistent via localStorage)

- **CollaborationPanel Component** (`CollaborationPanel.tsx`):
  - UI panel untuk manajemen kolaborasi
  - Menampilkan Room ID dari tab aktif
  - Copy room link button (copies URL with `?room={roomId}` query param)
  - Regenerate room ID button untuk membuat room baru
  - Collapsible panel dengan smooth transition

- **Store Actions Baru**:
  - `regenerateRoomId(id)` - generate new room ID untuk tab tertentu
  - `getActiveTabRoomId()` - dapatkan room ID dari tab aktif

- **UI Updates**:
  - Collaboration button di toolbar kiri atas
  - Panel muncul saat button diklik

### 🧠 Technical Decisions & Challenges
- Room ID menggunakan `nanoid(10)` untuk generate string pendek yang unik
- Room ID diperbarui secara otomatis saat tab aktif berubah (reactive via Zustand)
- Format share URL: `{origin}?room={roomId}` untuk memudahkan sharing
- Full real-time collaboration (WebSocket server) belum diimplementasikan - ini adalah foundation untuk future development

### ⚠️ Known Issues / TODOs
- Real-time syncing belum aktif - perlu WebSocket server (Socket.io/Yjs)
- Belum ada user awareness (cursor, username, presence)
- Room link belum otomatis join saat dibuka

### 🏗️ Next Steps
- Phase 5: UI/UX Polish & Advanced Features
  - Custom sidebar untuk aset management
  - Dark/Light mode toggle
  - Performance optimization untuk large canvas

---

## ✅ Phase 5: UI/UX Polish & Advanced Features Completed
**Date:** 2026-01-31

### 🛠 Features Implemented
- **Dark/Light Mode Toggle**:
  - Theme store (`useThemeStore.ts`) dengan Zustand persist
  - Theme toggle button di toolbar (Sun/Moon icon)
  - Sinksronisasi tema antara UI Tailwind dan Excalidraw canvas
  - Persistent theme preference di localStorage

- **Themed Components**:
  - Toolbar dengan full dark mode support (background, borders, text colors)
  - TabBar dengan dark mode styling
  - Dropdown panel collaboration dengan adaptive theme

- **Sidebar Foundation**:
  - Sidebar toggle button di toolbar
  - State management untuk sidebar open/close

### 🧠 Technical Decisions & Challenges
- Theme state disimpan di Zustand dengan persist middleware
- Excalidraw theme sync dilakukan via `excalidrawAPI.updateScene({ appState: { theme } })`
- Menggunakan conditional Tailwind classes dengan `theme === "dark"` check
- Dark mode menggunakan gray-700/800 shades untuk consistency

### ⚠️ Known Issues / TODOs
- Sidebar panel belum diimplementasikan (hanya toggle button)
- Performance optimization belum ditambahkan
- Full real-time collaboration masih memerlukan WebSocket server

### 🏗️ Project Status
**All 5 Phases Completed!** 🎉
- Phase 1: Foundation & Basic Canvas ✅
- Phase 2: Multi-Tab Management ✅
- Phase 3: Persistence & File Operations ✅
- Phase 4: Real-time Collaboration Setup ✅
- Phase 5: UI/UX Polish & Advanced Features ✅

---

## 📋 Consolidated Known Issues & TODOs

### 🔴 High Priority
- **Real-time collaboration sync** - Requires WebSocket server implementation (Socket.io/Yjs)
- **User awareness features** - Show other users' cursors and presence on canvas
- **Room link auto-join** - Automatically join room when URL contains `?room={roomId}` query parameter

### 🟡 Medium Priority
- ~~**Sidebar panel implementation** - Currently only has toggle button, need actual panel content~~ ✅ Done
- ~~**Performance optimization** - Canvas optimization for large drawings with thousands of elements~~ ✅ Done
- ~~**Drag-and-drop import** - Currently import only works via button click~~ ✅ Done
- ~~**Delete confirmation** - No confirmation dialog when deleting tab with content~~ ✅ Done

### 🟢 Low Priority / Nice to Have
- ~~**Node.js version** - Upgrade to 20.19+ or 22.12+ (current 20.17.0)~~ ✅ Done (upgraded to 22.18.0)
- ~~**Bundle size** - Excalidraw bundle size is large (~1.3MB) - Consider dynamic import or code splitting~~ ✅ Done
- ~~**Keyboard shortcuts** - Add keyboard shortcuts for common actions (Ctrl+S, Ctrl+Z, etc.)~~ ✅ Done

---

## 🔧 Medium Priority Improvements Completed
**Date:** 2026-01-31

### 🛠 Features Implemented

- **Delete Confirmation Dialog**:
  - `ConfirmDialog` component with warning icon and customizable text
  - Shows only when tab has content (elements.length > 0)
  - Empty tabs can be deleted without confirmation
  - Works with both button click and keyboard shortcuts (Delete/Backspace)

- **Sidebar Panel** (`Sidebar.tsx`):
  - Left-aligned panel showing all sheets overview
  - Displays for each sheet:
    - Sheet title with icon
    - Sheet count
    - Last modified timestamp
  - Visual indicator for active sheet
  - Smooth slide-in animation with backdrop
  - Fully themed for dark/light mode
  - **Click entire card to switch files** (improved UX)
  - Rename and delete actions with stopPropagation to prevent unwanted file switches
  - New file creation button in header

- **Drag-and-Drop Import**:
  - Drop .excalidraw or .json files directly onto canvas to import
  - Supports both native Excalidraw format (single tab) and custom multi-tab format
  - Automatically creates new tab with imported data
  - Saves current state before importing to prevent data loss
  - Error handling with user-friendly alert for invalid files
  - File type detection by extension (.excalidraw, .json) and MIME type

- **Performance Optimization**:
  - **Debounced auto-save** using lodash.debounce with 500ms delay
  - Reduces state write operations during continuous drawing/editing
  - Prevents performance degradation with large element counts
  - Critical saves (tab switching, adding tabs) still happen immediately
  - Maintains data integrity while improving responsiveness

- **Tab List Dropdown** (`TabBar.tsx`):
  - List icon button in TabBar to show dropdown menu of all tabs
  - Dropdown appears above TabBar (bottom-full positioning for bottom-positioned TabBar)
  - High z-index (100) to appear above whiteboard canvas
  - Click any tab in dropdown to switch to it
  - Delete button (X) on each tab item in dropdown
  - Closes when clicking outside or after selecting/deleting
  - Fully themed for dark/light mode
  - Scrollable (max-height 384px) for many tabs
  - Visual indicator for active tab in dropdown

- **Import/Export Bug Fix**:
  - Added `loadNativeExcalidraw` function to Zustand store
  - Fixes "loadNativeExcalidraw is not a function" error
  - Properly handles native Excalidraw format import
  - Supports three import formats:
    - Multi-tab custom format (with `tabs` and `activeTabId`)
    - Native Excalidraw format (with `type: "excalidraw"`, `elements`, `appState`, `files`)
    - Simple array of elements (legacy format)
  - Native format loads into active tab without creating new tabs

### 🧠 Technical Decisions & Challenges
- Delete confirmation checks tab content before showing dialog to avoid unnecessary prompts
- Sidebar uses fixed positioning with z-index layering to appear above canvas
- Both components reuse existing theme store for consistent styling
- Sidebar closes when clicking backdrop (expected UX pattern)
- Moved onClick handler to parent div for better click target area
- Added stopPropagation on interactive elements (input, buttons) to prevent event bubbling
- **Drag-and-drop** uses React's onDrop and onDragOver events on the main container
- **Debounce delay of 500ms** balances performance vs data safety for auto-save
- lodash.debounce already in dependencies, no additional package needed
- **Tab List Dropdown** positioned with `bottom-full` and high z-index for proper layering above canvas
- Click-outside detection using useEffect with mousedown event listener
- **Import fix** added new store action `loadNativeExcalidraw` to handle native Excalidraw format
- Import logic now properly distinguishes between multi-tab format and single native format

---

## 🎨 UI Improvements - Nice to Have
**Date:** 2026-01-31

### 📋 Future UI Enhancement Ideas

#### Toolbar Polish
- **Toolbar spacing improvements** - Add proper left/right padding to prevent child elements from touching container edges
- **Toolbar positioning** - Consider sticky or fixed positioning so toolbar remains visible during scrolling
- **Tooltip improvements** - Add keyboard shortcuts hints to tooltips (e.g., "Export (Ctrl+E)")
- **Toolbar grouping** - Add visual separators between different tool groups for better organization
- **Icon animations** - Add subtle hover animations to toolbar icons for better feedback

#### Visual Enhancements
- **Smooth transitions** - Add smooth color/size transitions when switching between dark/light mode
- **Loading states** - Add skeleton screens or loading spinners during file import/export operations
- **Success notifications** - Add toast notifications for successful operations (export complete, file imported, etc.)
- **Error boundaries** - Add error boundary components to gracefully handle component failures
- **Focus indicators** - Improve keyboard navigation focus indicators across all interactive elements

#### Collaboration Panel
- **Participant avatars** - Show user avatars/initials for participants in the room
- **Online status indicators** - Show green dot for online users, gray for offline
- **Participant list** - Display list of all participants in the collaboration panel
- **Room settings** - Add room settings (password protection, max participants, etc.)
- **Quick share buttons** - Add share buttons for copy link, email, Slack, etc.

#### Sidebar Enhancements
- **Search functionality** - Add search/filter for finding tabs by name
- **Tab sorting** - Add options to sort tabs by name, date created, or last modified
- **Tab grouping** - Allow grouping tabs into folders or categories
- **Recent files** - Add "Recently Opened" section in sidebar
- **Favorites/Starred** - Allow starring frequently used tabs
- **Tab thumbnails** - Show small thumbnails of tab content in sidebar
- **Color coding** - Allow assigning colors to tabs for visual organization

#### TabBar Improvements
- **Drag to reorder** - Allow dragging tabs to reorder them
- **Tab pinning** - Pin important tabs so they can't be accidentally closed
- **Tab duplicates** - Add option to duplicate/copy existing tabs
- **Tab templates** - Save tabs as templates for reuse
- **Max width indicators** - Show ellipsis (...) when too many tabs are open

#### Import/Export UX
- **Export progress** - Show progress bar for large file exports
- **Batch operations** - Allow selecting and exporting multiple tabs at once
- **Auto-naming** - Smart default filenames based on content or date
- **Format presets** - Save export format preferences (PNG resolution, SVG options, etc.)
- **Cloud export** - Add options to export directly to cloud storage (Google Drive, Dropbox)

#### Accessibility (a11y)
- **Keyboard shortcuts** - Comprehensive keyboard shortcut support (Ctrl+N new tab, Ctrl+W close, etc.)
- **Screen reader support** - Add ARIA labels and announcements for screen readers
- **High contrast mode** - Option for high contrast theme for better visibility
- **Reduced motion** - Respect prefers-reduced-motion for users with motion sensitivity
- **Focus trap** - Proper focus management in modals and panels

#### Performance Indicators
- **Element count** - Show number of elements in current tab
- **Storage usage** - Display localStorage usage and warnings
- **Performance stats** - Show render time or FPS for debugging
- **Auto-save indicator** - Visual indicator when auto-save is in progress

#### Micro-interactions
- **Button ripple effects** - Add ripple effect on button clicks (Material Design style)
- **Hover previews** - Show tab content preview on hover
- **Celebration animations** - Subtle confetti or checkmark animation on successful operations
- **Empty state illustrations** - Friendly illustrations when no tabs or content exists

#### Responsive Design
- **Mobile toolbar** - Collapsible toolbar for smaller screens
- **Touch gestures** - Support pinch-to-zoom, two-finger pan on touch devices
- **Adaptive sidebar** - Sidebar becomes full-screen drawer on mobile
- **Responsive canvas** - Better handling of canvas on different screen sizes

### 🎯 Priority Categories
- **Quick Wins** - Can be implemented quickly with high UX impact (tooltips, notifications, spacing fixes)
- **Medium Effort** - Requires more development but significant value (drag-reorder, search, thumbnails)
- **Long-term** - Major features requiring substantial work (cloud integration, real-time sync, advanced collaboration)

---

## ✅ Phase 7: Real-Time Collaboration Stabilization (COMPLETE)
**Date:** 2026-03-30

### 🛠 Part 1 - Element Sync Fix

**Backend Changes:**
- **Extended `ElementPayload` struct** (`internal/websocket/types.go`):
  - Added all Excalidraw-required fields: `seed`, `version`, `versionNonce`, `strokeWidth`, `strokeStyle`, `roughness`, `opacity`, `isDeleted`, `groupIds`, `frameId`, `boundElements`, `updated`, `link`, `locked`
  - Renamed old fields to match Excalidraw naming: `stroke` → `strokeColor`, `background` → `backgroundColor`, `fill` → `fillStyle`
  - Added `BoundElementPayload` type for bound element relationships

- **Updated `room.Element` struct** (`internal/room/room.go`):
  - Added `BoundElement` type for bound element relationships
  - Extended element struct with all new fields to match frontend requirements

- **Updated conversion functions** (`internal/websocket/handler.go`):
  - `elementsToPayload()` - Now converts all fields including new properties
  - `payloadToElements()` - Reconstructs full room.Element from payload
  - Added `convertBoundElements()` and `convertPayloadBoundElements()` helpers

**Frontend Changes:**
- **Renamed `Element` type** (`src/types/websocket.ts`):
  - Renamed to `ExcalidrawElementPayload` to avoid conflict with DOM `Element` type
  - Kept `Element` as deprecated alias for backward compatibility
  - Extended with all new fields matching backend

- **Updated `convertBackendToExcalidraw()`** (`src/components/Whiteboard.tsx`):
  - Now generates defaults for all required Excalidraw properties
  - Generates `seed` (random), `version` (1), `versionNonce` (random), `updated` (timestamp) if missing
  - Properly handles all element types with complete property mapping
  - Removed unused `convertExcalidrawToBackend()` (handled by elementSyncService)

- **Updated `elementSyncService`** (`src/services/elementSyncService.ts`):
  - `convertToBackendElement()` - Now serializes all Excalidraw properties
  - `convertToExcalidrawElement()` - Reconstructs full element with all required fields
  - `elementsDiffer()` - Compares all relevant properties for change detection
  - Updated all type references from `Element` to `ExcalidrawElementPayload`

### 🛠 Part 2 - Cursor, Auto-Join & Conflict UI

**Remote Cursor Coordinate Transformation** (`src/components/Whiteboard.tsx`, `src/components/RemoteCursors.tsx`, `src/services/cursorService.ts`):
- Fixed coordinate transformation between screen and canvas space
- Mouse position tracking in screen coordinates
- Transform to canvas coordinates before sending to backend:
  ```
  canvasX = (screenX - offsetLeft - scrollX) / zoom
  canvasY = (screenY - offsetTop - scrollY) / zoom
  ```
- Transform back to screen coordinates for rendering remote cursors:
  ```
  screenX = canvasX * zoom + scrollX + offsetLeft
  screenY = canvasY * zoom + scrollY + offsetTop
  ```
- Cursors update when zoom/scroll changes to maintain correct positioning
- Added `excalidrawAPI` prop to `RemoteCursors` component for viewport state access

**Room Auto-Join with Invite Flow** (`src/components/RoomInviteDialog.tsx`, `src/utils/roomURL.ts`):
- New `RoomInviteDialog` component for confirmation modal
- Detects `?room={roomId}` in URL on app load
- Shows professional invite dialog with:
  - Room ID display
  - User avatar with color
  - Username confirmation
  - Join/Cancel buttons
  - Error handling
- Clears URL query param after join or cancel
- Loading state during connection

**Conflict Resolution UI Panel** (`src/components/ConflictResolutionPanel.tsx`, `src/components/Whiteboard.tsx`):
- New `ConflictResolutionPanel` component showing changes from other users
- Tracks conflicts with detailed info:
  - Username and color
  - Timestamp
  - Description ("added 2, updated 3, deleted 1")
  - Element count
- Collapsible panel design
- Individual dismiss or dismiss all
- Last-write-wins resolution strategy note
- Maximum 10 conflicts to prevent UI overflow
- Integrated with element sync service to record changes

### 🧠 Technical Decisions & Challenges

**Root Cause of Sync Issues:**
The original implementation only sent basic geometric properties but Excalidraw requires ~20 additional properties for proper rendering.

**Cursor Coordinate Challenge:**
The original cursor tracking used `scrollX`/`scrollY` which are canvas pan positions, not mouse positions. Fixed by:
1. Tracking actual mouse position in screen coordinates
2. Transforming to canvas coordinates before sending
3. Other users transform back to screen coordinates for rendering
4. Re-rendering when zoom/scroll changes to keep cursors accurate

**Type Safety Challenge:**
Discovered that the frontend type name `Element` conflicted with the DOM's global `Element` type. Solution was renaming to `ExcalidrawElementPayload`.

### ✅ Build Verification

- ✅ Frontend builds successfully (`npm run build`)
- ✅ Backend builds successfully (`go build ./cmd/server`)
- ✅ No TypeScript errors
- ✅ No Go compilation errors

### 📊 Files Modified

**Backend:**
- `excalidraw-be/internal/websocket/types.go` - Extended ElementPayload
- `excalidraw-be/internal/room/room.go` - Extended Element struct, added BoundElement
- `excalidraw-be/internal/websocket/handler.go` - Updated conversion functions

**Frontend:**
- `excalidraw-fe/src/types/websocket.ts` - Renamed Element type, extended properties
- `excalidraw-fe/src/components/Whiteboard.tsx` - Updated conversions, added mouse tracking, integrated conflict panel and invite dialog
- `excalidraw-fe/src/components/RemoteCursors.tsx` - Added coordinate transformation with excalidrawAPI prop
- `excalidraw-fe/src/components/RoomInviteDialog.tsx` - New invite confirmation dialog
- `excalidraw-fe/src/components/ConflictResolutionPanel.tsx` - New conflict tracking UI
- `excalidraw-fe/src/services/elementSyncService.ts` - Updated all conversions
- `excalidraw-fe/src/utils/roomURL.ts` - URL utilities (unchanged but utilized)

### ⏭️ Next Steps

- **Phase 9**: File Storage (S3/MinIO integration for images)
- **Phase 10**: File Encryption (AES-GCM)
- **Phase 11**: REST API Completion (CRUD endpoints)
