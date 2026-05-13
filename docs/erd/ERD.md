# Dellcalidraw - Entity Relationship Diagram

## Overview

This document describes the database schema and architecture for the Dellcalidraw project. The project uses PostgreSQL for backend persistence with React frontend that supports both local storage and cloud sync.

## Architecture Diagram

```
┌─────────────────────────────────────────────────────────────────────────────────────────────┐
│                                    USER MANAGEMENT                                           │
├─────────────────────────────────────────────────────────────────────────────────────────────┤
│                                                                                             │
│                              ┌─────────────────┐                                          │
│                              │      users       │                                          │
│                              ├─────────────────┤                                          │
│                              │ id (PK) [UUID]  │                                          │
│                              │ username        │                                          │
│                              │ email           │◄────────────┐                            │
│                              │ password (hash) │             │                            │
│                              │ avatar_url      │             │                            │
│                              │ created_at      │             │                            │
│                              │ updated_at      │             │                            │
│                              └─────────────────┘             │                            │
│                                      ▲                        │                            │
│                     ┌─────────────────┼─────────────────┐      │                            │
│                     │                 │                 │      │                            │
│              ┌──────┴───────┐ ┌──────┴───────┐  ┌─────┴────┐  │                            │
│              │user_files    │ │refresh_tokens│  │password_ │  │                            │
│              ├──────────────┤ ├──────────────┤  │reset_    │  │                            │
│              │ id (PK) [UUID]│ │ id (PK) [UUID]│ │tokens    │  │                            │
│              │ user_id (FK)──┼─►user_id      │  ├─────────┤  │                            │
│              │ name          │ │ token        │  │token     │  │                            │
│              │ tab_count     │ │ expires_at   │  │user_id───┼──┘                            │
│              │ created_at   │ │ revoked      │  │expires_at│                               │
│              │ updated_at   │ └──────────────┘  └──────────┘                               │
│              └──────────────┘                                                               │
│                     │                                                                       │
│                     │ 1:N                                                                  │
│                     ▼                                                                       │
│           ┌─────────────────┐                                                              │
│           │      rooms       │                                                              │
│           ├─────────────────┤                                                              │
│           │ id (PK) [UUID]  │                                                              │
│           │ key (unique)    │◄─── Used for room URL (roomId)                               │
│           │ name            │                                                              │
│           │ owner_id (FK)──►users        │                                               │
│           │ password_hash   │              │                                               │
│           │ is_public       │              │                                               │
│           │ allow_anonymous │              │                                               │
│           │ created_at      │              │                                               │
│           │ updated_at      │              │                                               │
│           └─────────────────┘              │                                               │
│              │         │                   │                                               │
│              │         │                   │                                               │
│       ┌─────┴───┐ ┌───┴─────┐              │                                               │
│       │         │ │         │              │                                               │
│ ┌─────┴───┐ ┌───┴─┴───┐    │              │                                               │
│ │         │ │         │    │              │                                               │
│ ▼         ▼ ▼         ▼    ▼              ▼                                               │
│ ┌─────────────────────────────┐  ┌───────────────────────┐  ┌───────────────────┐         │
│ │     room_elements           │  │    room_members       │  │room_invitations   │         │
│ ├─────────────────────────────┤  ├───────────────────────┤  ├───────────────────┤         │
│ │ id (PK) [SERIAL]            │  │ id (PK) [UUID]        │  │ id (PK) [UUID]     │         │
│ │ room_id (FK)───►rooms       │  │ room_id (FK)──►rooms  │  │ room_id (FK)──►rooms│         │
│ │ element_id                  │  │ user_id (FK)──►users  │  │ email              │         │
│ │ data (JSONB)                │  │ role (owner/editor/   │  │ role               │         │
│ │ version                     │  │         viewer)       │  │ token              │         │
│ │ updated_at                  │  │ invited_by──►users   │  │ invited_by──►users │         │
│ └─────────────────────────────┘  │ created_at            │  │ expires_at         │         │
│                                  │ updated_at            │  │ used_at            │         │
│                                  └───────────────────────┘  │ created_at          │         │
│                                                                 └───────────────────┘         │
│                                       │                                                │
│                                       │ 1:N                                            │
│                                       ▼                                                │
│                             ┌─────────────────┐                                        │
│                             │   room_files    │                                        │
│                             ├─────────────────┤                                        │
│                             │ id (PK) [SERIAL]│                                        │
│                             │ room_id (FK)───►rooms   │                                │
│                             │ file_id         │                                        │
│                             │ mime_type       │                                        │
│                             │ size            │                                        │
│                             │ storage_key     │                                        │
│                             │ created_at      │                                        │
│                             └─────────────────┘                                        │
│                                                                                         │
└───────────────────────────────────────────────────────────────────────────────────────────┘
```

---

## Tables

### users

User accounts table.

| Column | Type | Constraints | Description |
|--------|------|------------|-------------|
| id | UUID | PRIMARY KEY, DEFAULT gen_random_uuid() | Unique user identifier |
| username | VARCHAR(255) | NOT NULL, UNIQUE | Display name |
| email | VARCHAR(255) | NOT NULL, UNIQUE | User email address |
| password | VARCHAR(255) | NOT NULL | Bcrypt hashed password |
| avatar_url | VARCHAR(255) | NULL | Profile picture URL |
| created_at | TIMESTAMP | DEFAULT NOW() | Account creation time |
| updated_at | TIMESTAMP | DEFAULT NOW() | Last update time |

**Indexes:**
- `email` (unique)
- `username` (unique)

---

### user_files

Stores metadata about user's files (actual canvas data stays in rooms table).

| Column | Type | Constraints | Description |
|--------|------|------------|-------------|
| id | UUID | PRIMARY KEY, DEFAULT gen_random_uuid() | Unique file identifier |
| user_id | UUID | NOT NULL, REFERENCES users(id) ON DELETE CASCADE | Owner user ID |
| name | VARCHAR(255) | NOT NULL, DEFAULT 'Untitled' | File name |
| tab_count | INTEGER | NOT NULL, DEFAULT 1 | Number of sheets/tabs |
| created_at | TIMESTAMP | DEFAULT NOW() | Creation time |
| updated_at | TIMESTAMP | DEFAULT NOW() | Last update time |

**Indexes:**
- `user_id` (for listing user's files)
- `updated_at DESC` (for sorting by recent)

**Relationships:**
- `user_id` → `users.id` (many-to-one)

---

### rooms

Canvas rooms (each sheet/tab in a file is a room).

| Column | Type | Constraints | Description |
|--------|------|------------|-------------|
| id | UUID | PRIMARY KEY, DEFAULT gen_random_uuid() | Unique room identifier |
| key | VARCHAR(64) | NOT NULL, UNIQUE | Shareable room ID (URL) |
| name | VARCHAR(255) | NULL | Room name |
| owner_id | UUID | REFERENCES users(id) ON DELETE SET NULL | Room owner |
| password_hash | VARCHAR(255) | NULL | Optional room password |
| is_public | BOOLEAN | DEFAULT TRUE | Public access flag |
| allow_anonymous | BOOLEAN | DEFAULT TRUE | Allow anonymous users |
| created_at | TIMESTAMP | DEFAULT NOW() | Creation time |
| updated_at | TIMESTAMP | DEFAULT NOW() | Last activity time |

**Indexes:**
- `key` (unique, for fast room lookup)
- `updated_at` (for cleanup queries)

**Relationships:**
- `owner_id` → `users.id` (many-to-one)

---

### room_elements

Stores canvas drawing elements as JSON.

| Column | Type | Constraints | Description |
|--------|------|------------|-------------|
| id | SERIAL | PRIMARY KEY | Auto-increment ID |
| room_id | UUID | NOT NULL, REFERENCES rooms(id) ON DELETE CASCADE | Parent room |
| element_id | VARCHAR(255) | NOT NULL | Excalidraw element ID |
| data | JSONB | NOT NULL | Element JSON data |
| version | INTEGER | DEFAULT 1 | Element version |
| updated_at | TIMESTAMP | DEFAULT NOW() | Last update time |

**Indexes:**
- `(room_id, element_id)` UNIQUE (for upsert)
- `room_id` (for room queries)

**Relationships:**
- `room_id` → `rooms.id` (many-to-one)

---

### room_members

Persistent access assignments for rooms.

| Column | Type | Constraints | Description |
|--------|------|------------|-------------|
| id | UUID | PRIMARY KEY, DEFAULT gen_random_uuid() | Unique member ID |
| room_id | UUID | NOT NULL, REFERENCES rooms(id) ON DELETE CASCADE | Room |
| user_id | UUID | NOT NULL, REFERENCES users(id) ON DELETE CASCADE | User |
| role | VARCHAR(20) | NOT NULL, DEFAULT 'editor' | Role: owner, editor, viewer |
| invited_by | UUID | REFERENCES users(id) ON DELETE SET NULL | Who invited |
| created_at | TIMESTAMP | DEFAULT NOW() | When added |
| updated_at | TIMESTAMP | DEFAULT NOW() | Last change |

**Indexes:**
- `(room_id, user_id)` UNIQUE (prevent duplicates)
- `room_id` (for room member queries)
- `user_id` (for user's room list)

**Roles:**
- `owner` - Full control, can delete room
- `editor` - Can edit canvas and invite others
- `viewer` - Read-only access

---

### room_invitations

Pending room invitations.

| Column | Type | Constraints | Description |
|--------|------|------------|-------------|
| id | UUID | PRIMARY KEY, DEFAULT gen_random_uuid() | Invitation ID |
| room_id | UUID | NOT NULL, REFERENCES rooms(id) ON DELETE CASCADE | Target room |
| email | VARCHAR(255) | NULL | Invite by email |
| role | VARCHAR(20) | NOT NULL, DEFAULT 'editor' | Assigned role |
| token | VARCHAR(255) | NOT NULL, UNIQUE | Invite token |
| invited_by | UUID | REFERENCES users(id) ON DELETE SET NULL | Who invited |
| expires_at | TIMESTAMP | NOT NULL | Expiration time |
| used_at | TIMESTAMP | NULL | When accepted |
| created_at | TIMESTAMP | DEFAULT NOW() | Creation time |

**Indexes:**
- `token` UNIQUE (for invite lookup)
- `email` (for pending invites by email)
- `room_id` (for room's invitations)

---

### room_files

Metadata for uploaded files (images, etc.) in rooms.

| Column | Type | Constraints | Description |
|--------|------|------------|-------------|
| id | SERIAL | PRIMARY KEY | Auto-increment ID |
| room_id | UUID | NOT NULL, REFERENCES rooms(id) ON DELETE CASCADE | Parent room |
| file_id | VARCHAR(255) | NOT NULL | Unique file identifier |
| mime_type | VARCHAR(100) | NULL | File MIME type |
| size | BIGINT | NULL | File size in bytes |
| storage_key | VARCHAR(255) | NULL | Object storage key |
| created_at | TIMESTAMP | DEFAULT NOW() | Upload time |

**Indexes:**
- `(room_id, file_id)` UNIQUE (for upsert)
- `room_id` (for room file listing)

**Relationships:**
- `room_id` → `rooms.id` (many-to-one)

---

### refresh_tokens

JWT refresh tokens for authenticated sessions.

| Column | Type | Constraints | Description |
|--------|------|------------|-------------|
| id | UUID | PRIMARY KEY, DEFAULT gen_random_uuid() | Token ID |
| user_id | UUID | NOT NULL, REFERENCES users(id) ON DELETE CASCADE | Owner user |
| token | VARCHAR(255) | NOT NULL | Refresh token string |
| expires_at | TIMESTAMP | NOT NULL | Expiration time |
| created_at | TIMESTAMP | DEFAULT NOW() | Issue time |
| revoked | BOOLEAN | DEFAULT FALSE | Revocation flag |

**Indexes:**
- `token` UNIQUE (for token validation)
- `user_id` (for user's token list)

---

### password_reset_tokens

Password reset tokens for account recovery.

| Column | Type | Constraints | Description |
|--------|------|------------|-------------|
| id | UUID | PRIMARY KEY, DEFAULT gen_random_uuid() | Token ID |
| user_id | UUID | NOT NULL, REFERENCES users(id) ON DELETE CASCADE | Target user |
| token | VARCHAR(255) | NOT NULL, UNIQUE | Reset token |
| expires_at | TIMESTAMP | NOT NULL | Expiration time |
| created_at | TIMESTAMP | DEFAULT NOW() | Issue time |

**Indexes:**
- `token` UNIQUE (for token lookup)

---

## Relationships Summary

```
users
 ├─── user_files (1:N) ──────────── File metadata per user
 ├─── refresh_tokens (1:N) ───────── Login sessions
 ├─── password_reset_tokens (1:N) ── Password recovery
 └─── room_members (1:N) ──┬─── rooms (1:N) ──── room_elements (1:N)
                          │                    └─── room_files (1:N)
                          └─── room_invitations (1:N)
```

---

## Data Model

### User → File → Tab → Room

```
┌─────────┐         ┌───────────┐         ┌──────┐         ┌────────┐
│  User   │ 1:N     │ UserFile  │ 1:N     │ Room │ 1:N    │ Room   │
│         │────────►│ (meta)    │────────►│(tab) │────────►│Element │
└─────────┘         └───────────┘         └──────┘         │(canvas)│
                                                          └────────┘
```

- **User**: Has many files (user_files table)
- **UserFile**: Contains metadata (name, tab count) and references tabs
- **Room**: Each tab maps to a room (rooms table with unique key)
- **RoomElement**: Actual canvas drawing data stored per room

---

## API Endpoints

### Authentication

| Method | Endpoint | Auth | Description |
|--------|----------|------|-------------|
| POST | `/api/auth/register` | ❌ | Register new user |
| POST | `/api/auth/login` | ❌ | Login user |
| POST | `/api/auth/refresh` | ❌ | Refresh access token |
| POST | `/api/auth/logout` | ❌ | Logout user |
| POST | `/api/auth/forgot-password` | ❌ | Request password reset |
| POST | `/api/auth/validate-reset-token` | ❌ | Validate reset token |
| POST | `/api/auth/reset-password` | ❌ | Reset password |

### User Management

| Method | Endpoint | Auth | Description |
|--------|----------|------|-------------|
| GET | `/api/users/me` | ✅ | Get current user profile |
| PUT | `/api/users/me` | ✅ | Update current user profile |
| GET | `/api/users/:id` | ✅ | Get user by ID |

### File Management

| Method | Endpoint | Auth | Description |
|--------|----------|------|-------------|
| GET | `/api/files` | ✅ | List all user files |
| POST | `/api/files` | ✅ | Create new file |
| GET | `/api/files/:fileId` | ✅ | Get file by ID |
| PUT | `/api/files/:fileId` | ✅ | Update file metadata |
| PATCH | `/api/files/:fileId/rename` | ✅ | Rename file |
| DELETE | `/api/files/:fileId` | ✅ | Delete file |

### Canvas Operations

| Method | Endpoint | Auth | Description |
|--------|----------|------|-------------|
| POST | `/api/rooms/:roomId/canvas/save` | ❌ | Save canvas state |
| GET | `/api/rooms/:roomId/canvas/load` | ❌ | Load canvas state |
| POST | `/api/rooms/:roomId/canvas/restore` | ❌ | Restore canvas |
| DELETE | `/api/rooms/:roomId/canvas` | ❌ | Clear canvas |

### File Upload

| Method | Endpoint | Auth | Description |
|--------|----------|------|-------------|
| POST | `/api/rooms/:roomId/files` | ❌ | Upload file |
| GET | `/api/rooms/:roomId/files/:fileId` | ❌ | Download file |
| DELETE | `/api/rooms/:roomId/files/:fileId` | ❌ | Delete file |
| GET | `/api/rooms/:roomId/files` | ❌ | List room files |

### WebSocket

| Endpoint | Description |
|----------|-------------|
| `/ws` | Real-time collaboration WebSocket |

---

## Frontend Architecture

### File Storage Strategy

```
┌─────────────────────────────────────────────────────────────┐
│                        Frontend                              │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  ┌─────────────────────┐    ┌─────────────────────┐        │
│  │   useAuthStore      │    │    useFileStore      │        │
│  ├─────────────────────┤    ├─────────────────────┤        │
│  │ user: User          │    │ localFiles: File[]   │        │
│  │ isAuthenticated     │    │ activeFileId: string │        │
│  │ accessToken         │    │ syncStatus           │        │
│  │ refreshToken        │    │                      │        │
│  └─────────────────────┘    └──────────┬──────────┘        │
│                                          │                   │
│                     ┌────────────────────┼────────────────────┐
│                     │                    │                    │
│                     ▼                    ▼                    ▼
│           ┌─────────────────┐  ┌─────────────────┐  ┌────────────────┐
│           │   NOT LOGGED IN │  │   LOGGED IN      │  │   REAL-TIME     │
│           │                 │  │                 │  │                 │
│           │ localStorage    │  │ API Sync        │  │   WebSocket     │
│           │ (persist)       │  │ (fileService)   │  │   (roomService) │
│           └─────────────────┘  └─────────────────┘  └────────────────┘
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

### Sync Flow

1. **Not Authenticated**:
   - Files stored in `localStorage` via Zustand persist
   - All operations are local
   - Data persists across sessions

2. **Authenticated (Login)**:
   - Auto-sync from cloud via `/api/files`
   - Cloud files replace local files
   - Changes sync to cloud

3. **Logout**:
   - Cloud files remain in localStorage
   - New files stored locally
   - Can log back in to sync again

---

## Migration History

| # | Migration | Description |
|---|-----------|-------------|
| 000001 | init_schema | Basic rooms and room_elements tables |
| 000002 | file_storage | room_files table for uploaded images |
| 000003 | user_auth | users, refresh_tokens tables |
| 000004 | room_permissions | room_members, room_invitations, room settings |
| 000005 | password_reset | password_reset_tokens table |
| 000006 | user_files | user_files table for file metadata |

---

## Project Structure

```
dellcalidraw/
├── docs/
│   ├── erd/
│   │   └── ERD.md              ← This file
│   ├── be/
│   │   ├── DEVELOPENT_PHASES.md
│   │   └── BACKEND_REQUIREMENTS.md
│   └── fe/
│       └── FRONTEND_INTEGRATION.md
│
├── excalidraw-be/              ← Go Backend
│   ├── cmd/server/
│   │   ├── main.go
│   │   ├── auth_handlers.go
│   │   ├── file_management_handlers.go
│   │   ├── canvas_handlers.go
│   │   └── file_handlers.go
│   └── internal/
│       ├── auth/
│       │   └── auth.go, middleware.go
│       ├── database/
│       │   ├── users.go
│       │   ├── user_files.go
│       │   ├── repository.go
│       │   └── migrations/
│       ├── room/
│       └── websocket/
│
└── excalidraw-fe/              ← React Frontend
    └── src/
        ├── App.tsx
        ├── components/
        │   ├── Sidebar.tsx
        │   ├── Whiteboard.tsx
        │   └── AuthModal.tsx
        ├── store/
        │   ├── useAuthStore.ts
        │   ├── useFileStore.ts     ← New: File management
        │   └── useWhiteboardStore.ts
        └── services/
            ├── api.ts
            ├── fileService.ts      ← New: File API client
            └── roomService.ts
```

---

## Glossary

| Term | Description |
|------|-------------|
| **File** | Top-level container with name and multiple tabs |
| **Tab/Sheet** | Individual canvas, maps to a Room |
| **Room** | Real-time collaboration space with unique URL |
| **Room Element** | Drawing data (shapes, text, etc.) |
| **Local Storage** | Browser localStorage for offline mode |
| **Cloud Sync** | Sync with backend API when authenticated |