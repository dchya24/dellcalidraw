# Backend Service Requirements (MVP Scope)

## Overview

This document outlines the backend services required to support **real-time collaboration** features for the whiteboard application. The MVP scope focuses on enabling users to collaborate on whiteboards in real-time with minimal infrastructure.

**MVP Scope Decisions:**
- **User Access**: Guest access with temporary identity (username chosen by user)
- **Room Storage**: In-memory only (data lost on server restart)
- **User Profiles**: Temporary per session (foundation for future login system)
- **File Export**: Client-side only (no server-side file storage)

**Future Expansion:** Foundation will allow adding:
- Persistent user authentication (login system)
- Database-backed room persistence
- User profiles with saved preferences
- Server-side file storage

---

## 1. Real-time Collaboration Services

### 1.1 WebSocket Connection Management
- **Bidirectional communication channel** between clients and server
- **Connection lifecycle management**: handle connect, disconnect, reconnect events
- **Connection health monitoring**: heartbeat/ping-pong to detect stale connections
- **Scalable connection handling**: support multiple concurrent users per room
- **Automatic reconnection**: clients should reconnect seamlessly on network interruption

### 1.2 Room Management (In-Memory)
- **Room creation**: generate unique room identifiers (currently using 10-character nanoid)
- **Room lookup**: retrieve room state by room ID from in-memory store
- **Room lifecycle**: rooms exist in memory only, created when first user joins
- **Room capacity limits**: enforce maximum participants per room (configurable)
- **Room activity tracking**: track last activity timestamp for cleanup of inactive rooms
- **Auto-cleanup**: remove inactive rooms from memory after timeout period (e.g., 1 hour)
- **Multi-room support**: users can participate in multiple rooms simultaneously

### 1.3 Real-time State Synchronization
- **Element broadcasting**: propagate drawing changes (new elements, updates, deletions) to all room participants
- **Optimized updates**: send only incremental changes rather than full state
- **Conflict resolution**: handle concurrent edits using either:
  - Operational Transformation (OT)
  - Conflict-free Replicated Data Types (CRDT)
- **State reconciliation**: ensure all clients eventually reach consistent state
- **Change ordering**: maintain causal ordering of operations

### 1.4 Presence Tracking
- **User join/leave events**: notify all participants when users enter/exit rooms
- **Online status**: track which users are currently active in each room
- **Idle detection**: identify inactive users (no mouse/keyboard activity for timeout period)
- **User count**: display number of active participants in room

---

## 2. User Awareness Features

### 2.1 Cursor Position Broadcasting
- **Real-time cursor sync**: broadcast mouse position coordinates to all room participants
- **Cursor throttling**: limit cursor update frequency (e.g., 10-20 times per second) to reduce bandwidth
- **Cursor rendering**: display remote user cursors with distinct colors/labels
- **Cursor ownership**: associate each cursor with a user identity

### 2.2 User Identification (Temporary)
- **User identity management**: assign unique user IDs to each participant (session-based)
- **Display names**: show user-chosen names (prompt on join) or auto-generated random names
- **Visual distinction**: assign unique colors to each user for easy identification
- **Temporary profiles**: user data exists only for current session (not persisted)
- **Username prompt**: ask user to enter name when joining (optional, can auto-generate)

### 2.3 Selection & Interaction Awareness
- **Selected element sync**: broadcast which elements are selected by which users
- **Viewport sync**: optionally show viewport position of other users
- **Interaction indicators**: visual feedback when multiple users edit same element

---

## 3. User Management (Guest Access)

### 3.1 Guest Access (MVP)
- **Anonymous access**: users can join rooms without authentication
- **Username selection**: prompt users to enter display name when joining (or auto-generate)
- **Session-based identity**: assign temporary user ID valid for current session
- **No persistence**: user data not stored (username exists only in memory during session)

### 3.2 Future: Authentication Foundation
**Note: Not implemented in MVP, but architecture should support future addition**
- **Optional authentication**: support for persistent user accounts
- **Authentication methods**: Email/password, OAuth (Google, GitHub), Magic links
- **Session tokens**: JWT tokens for authenticated sessions
- **User profiles**: persistent user data (avatar, preferences)

### 3.3 Room Access Control (MVP - Simplified)
- **Public rooms only**: anyone with room link can join
- **No permissions system**: all participants can edit (equal access)
- **No password protection**: room ID is the only access control mechanism

**Future Expansion Points:**
- Private rooms with passwords
- Room permissions (owner, editor, viewer)
- Room creation restrictions

---

## 4. Data Persistence (MVP - In-Memory Only)

### 4.1 Room State Storage (In-Memory)
- **Runtime state only**: room data exists in server memory during runtime
- **No persistence**: room data is lost when server restarts
- **State synchronization**: maintain current state in memory for all connected clients
- **State retrieval**: provide current state to new users when they join
- **Auto-cleanup**: remove rooms from memory after inactivity timeout

### 4.2 No Database (MVP)
**Note: MVP does not require database for room/user data**
- Room state: In-memory only
- User data: Temporary session-based (no persistence)
- Element data: Stored in room state (in-memory)

### 4.3 Client-Side File Operations
**Note: File export/import handled entirely on client-side**
- **Export to client**: PNG, SVG, JSON exported to user's device (not stored on server)
- **Import from client**: Users load files from their device
- **No server storage**: No file upload/download to server

### 4.4 Future: Database Persistence
**Not in MVP, but architecture should support future addition**
- **Room persistence**: Save room state to database for recovery
- **User database**: Store user accounts and profiles
- **Element storage**: Persistent storage of whiteboard elements
- **Version history**: Track changes over time

---

## 5. Room Link Management

### 5.1 URL Routing
- **Room link generation**: create shareable URLs with room ID (`?room={roomId}`)
- **Query parameter parsing**: extract room ID from URL on page load
- **Auto-join**: automatically connect to room when room ID is present in URL
- **URL cleaning**: remove room ID from URL after joining (optional, for cleaner URLs)

### 5.2 Invitation System
- **Shareable links**: generate unique, shareable URLs for each room
- **Invitation metadata**: optional custom messages or permissions in links
- **Link expiration**: optionally expire invitation links after time or number of uses
- **QR code generation**: generate QR codes for easy mobile sharing

### 5.3 Room Discovery (Optional - Not in MVP)
**Note: Room discovery features not needed for MVP**
- **No public directory**: Rooms are accessed via direct links only
- **No search**: No room search functionality
- **No suggestions**: No room recommendation system

**Future Expansion:**
- Public room directory for discoverable rooms
- Search by room name or metadata
- Room suggestions based on activity

---

## 6. API Endpoints

### 6.1 WebSocket Events
**Client → Server:**
- `join_room` - User joins a room
- `leave_room` - User leaves a room
- `update_elements` - Send element changes
- `cursor_move` - Broadcast cursor position
- `selection_change` - Broadcast selected elements
- `heartbeat` - Maintain connection alive

**Server → Client:**
- `user_joined` - New user joined
- `user_left` - User left
- `elements_updated` - Receive element changes from others
- `cursor_updated` - Receive cursor position of others
- `selection_updated` - Receive selection changes
- `room_state` - Initial room state on join
- `error` - Error messages

### 6.2 HTTP REST API (MVP - Minimal)
**Note: MVP requires minimal HTTP API since most communication is via WebSocket**

**Room Management:**
- `GET /api/rooms/:id` - Get room metadata (optional, mostly for debugging)
- `GET /health` - Health check endpoint

**Removed from MVP (future expansion):**
- `POST /api/rooms` - Create new room (rooms created automatically via WebSocket)
- `DELETE /api/rooms/:id` - Delete room (not needed, rooms auto-cleanup)
- `GET /api/rooms/:id/state` - Get room state (via WebSocket join_room)
- `PUT /api/rooms/:id/state` - Save room state (via WebSocket update_elements)
- `GET /api/rooms/:id/participants` - Get participants (via WebSocket presence)
- `POST /api/auth/login` - User login (guest access only)
- `POST /api/auth/logout` - User logout (no auth in MVP)
- `GET /api/users/me` - Get user profile (no persistent profiles)

---

## 7. Performance & Scalability (MVP - Simplified)

### 7.1 Performance Optimization (MVP Focus)
- **Message throttling**: throttle cursor updates (10-20 times per second) to reduce bandwidth
- **Delta updates**: send only changed elements, not full state
- **Connection pooling**: efficient handling of multiple WebSocket connections

**Future Expansion:**
- Message batching for high-frequency updates
- Load balancing across multiple server instances
- Geographic distribution with edge servers

### 7.2 Scalability Requirements (MVP - Single Server)
- **Target capacity**: Support 10-50 concurrent users per room (reasonable for MVP)
- **Room limit**: Support 100-500 concurrent rooms (in-memory)
- **Message throughput**: Handle hundreds of messages per second
- **Memory management**: Auto-cleanup inactive rooms to prevent memory leaks

**Note:** MVP runs on single server. Multi-server scaling is future expansion.

---

## 8. Security Considerations

### 8.1 Input Validation
- **Validate all incoming data**: sanitize element data to prevent injection attacks
- **Rate limiting**: prevent spam/abuse by limiting message frequency
- **Size limits**: enforce maximum element count or data size per room

### 8.2 Data Privacy
- **Encryption**: use TLS/SSL for all communications
- **Data isolation**: ensure users can only access rooms they're authorized for
- **PII protection**: properly handle any personal information
- **GDPR compliance**: provide data export/deletion capabilities

### 8.3 Abuse Prevention
- **Spam prevention**: detect and block malicious behavior
- **Room hijacking prevention**: verify room ownership
- **DoS protection**: implement rate limiting and connection throttling

---

## 9. Monitoring & Observability (MVP - Basic)

### 9.1 Metrics to Track (MVP - Essential)
- **Active connections**: number of concurrent WebSocket connections
- **Room activity**: active rooms, participants per room
- **Error rates**: connection errors, failed operations
- **Resource usage**: CPU, memory (no storage in MVP)

**Future Expansion:**
- Message throughput metrics
- Latency tracking
- Detailed performance profiling

### 9.2 Logging (MVP - Minimal)
- **Error logs**: all errors with stack traces (essential)
- **Connection logs**: user connect/disconnect events (basic)
- **Performance logs**: slow operations, timeouts (optional)

**Note:** Keep logging minimal to reduce overhead

### 9.3 Health Checks (MVP)
- **Endpoint health**: `/health` endpoint for monitoring
- **WebSocket server status**: verify WebSocket server is running

**Note:** No database connectivity check (no database in MVP)

---

## 10. Deployment & Infrastructure (MVP - Simple)

### 10.1 Server Requirements (MVP - Single Server)
- **WebSocket server**: handles real-time bidirectional communication (e.g., Socket.io, ws)
- **Simple HTTP server**: serves frontend and health endpoint (can use existing dev server)
- **No database**: In-memory storage only (MVP)
- **No file storage**: Client-side file operations only

### 10.2 Infrastructure Needs (MVP - Minimal)
- **Single server deployment**: No load balancer needed for MVP
- **No message queue**: Direct WebSocket communication
- **No caching layer**: In-memory room state is sufficient
- **Static hosting**: Frontend can be served from same server or CDN

**Future Expansion:**
- Load balancer for multi-server deployment
- Redis for pub/sub across multiple servers
- CDN for static assets

### 10.3 Environment Configuration
- **Development**: Local development with hot reload
- **Production**: Single server (e.g., VPS, cloud instance)

**Recommended Platforms for MVP:**
- Railway, Render, Fly.io, or similar simple PaaS
- Or VPS (DigitalOcean, Linode) with Node.js

---

## Summary: MVP Backend Architecture

### Core Services (MVP)
1. **WebSocket Server** - Real-time bidirectional communication for collaboration
2. **In-Memory Room Management** - Temporary room state storage
3. **Guest User Management** - Session-based temporary identity
4. **Minimal HTTP API** - Health checks and optional room metadata

### What MVP Enables
✅ **Real-time collaboration sync** - Multiple users can draw together
✅ **User awareness (cursors, presence)** - See other users' cursors and who's online
✅ **Room link auto-join** - Share links to invite collaborators

### What's NOT in MVP (Future Expansion)
❌ Database persistence (room/user data)
❌ User authentication (login system)
❌ Server-side file storage
❌ Multi-server scaling
❌ Room discovery/search
❌ Advanced permissions

### MVP Technical Focus
- **Simplicity**: Single server, in-memory storage
- **Speed**: Fast to implement and deploy
- **Foundation**: Architecture supports future features
- **Cost-effective**: Minimal infrastructure requirements
