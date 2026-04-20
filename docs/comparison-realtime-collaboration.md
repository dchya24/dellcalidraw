# Real-Time Collaboration Comparison

## 📡 Overview

Comparison between current project implementation and technical specification for real-time collaboration features including WebSocket, message handling, encryption, and user awareness.

---

## ✅ Fully Implemented Features

| Spec Requirement | Spec Details | Current Implementation | Status |
| ---------------- | ------------ | ---------------------- | ------- |
| **WebSocket Protocol** | ws:// or wss:// | ✅ `ws://host:port/ws?room={roomId}` | **100%** ✅ |
| **Room Hub Architecture** | `map[string]*Room` with channels | ✅ Hub pattern with Room management | **100%** ✅ |
| **Client Register/Unregister** | Register/unregister channels | ✅ Join/leave handlers with mutex protection | **100%** ✅ |
| **Message Broadcasting** | Broadcast to all except sender | ✅ Broadcast with exclude sender logic | **100%** ✅ |
| **INIT Message** | Send initial scene to new users | ✅ `room_state` event with all elements | **100%** ✅ |
| **UPDATE Message** | Element delta sync | ✅ `element_change` event (added/updated/deleted) | **100%** ✅ |
| **MOUSE_LOCATION Message** | Cursor position sync | ✅ `cursor_position` event | **100%** ✅ |
| **Message Throttling** | Cursor: 50ms, Scene: 3s | ✅ Cursor: 50ms (20/sec), Elements: debounced | **100%** ✅ |
| **Rate Limiting** | Per-second and per-window limits | ✅ 20 msg/sec, 100 msg/10sec for elements | **100%** ✅ |
| **Heartbeat/Ping-Pong** | Connection health check | ✅ 10s interval, 10s timeout | **100%** ✅ |
| **Exponential Backoff** | Reconnection delay strategy | ✅ 1s→2s→4s→8s...→30s with jitter | **100%** ✅ |
| **Connection States** | Track connection status | ✅ 4 states: disconnected/connecting/connected/reconnecting | **100%** ✅ |
| **Reconnection Attempts** | Max retry count | ✅ 10 attempts (with jitter) | **100%** ✅ |
| **Offline Message Queue** | Queue messages during disconnect | ✅ Max 100 messages, auto-flush on reconnect | **100%** ✅ |
| **Acknowledgment System** | Request-response pattern | ✅ Promise-based ack with 10s timeout | **100%** ✅ |
| **User Presence** | Join/leave notifications | ✅ `user_joined`, `user_left` events | **100%** ✅ |
| **Cursor Rendering** | Show remote cursors with labels | ✅ `RemoteCursors` component with coordinate transform | **100%** ✅ |
| **Selection Sync** | Selection awareness | ✅ `selection_change` event with `SelectionOverlay` | **100%** ✅ |
| **Auto Re-join** | Reconnect and re-join room automatically | ✅ Auto re-join on WebSocket reconnect | **100%** ✅ |

---

## ⚠️ Partially Implemented Features

| Spec Requirement | Spec Details | Current Implementation | Gap |
| ---------------- | ------------ | ---------------------- | ---- |
| **Message Encryption** | AES-GCM encryption for all WS messages | ❌ No encryption (plaintext only) | **Critical Gap** - No privacy protection |
| **Encryption Key Management** | Random room key shared via URL | ❌ No key generation/management | Missing: Room key generation and sharing via URL hash |
| **IV (Initialization Vector)** | Random IV per message | ❌ No IV generation | Missing: Random IV for each encrypted message |
| **Message Envelope** | `{type, payload, iv}` format | ⚠️ `{type, payload}` (missing `iv` field) | Missing: IV field in message structure |
| **Client-side Encryption** | `encryptData()` from @excalidraw/encryption | ❌ No encryption function | Missing: Encrypt before sending to server |
| **Server-side Relay** | Relay encrypted messages without decryption | ❌ Messages processed (would need to relay blindly) | Would need to change to relay encrypted payloads only |
| **Client-side Decryption** | `decryptData()` from @excalidraw/encryption | ❌ No decryption function | Missing: Decrypt received messages |
| **IDLE_STATUS Message** | Broadcast idle state (30s threshold) | ❌ Not implemented | Missing: Idle detection and broadcasting |
| **Idle Detection** | 30s idle, 5s active thresholds | ❌ No idle detection | Missing: Pointer movement monitoring and timeout handling |
| **USER_FOLLOW Message** | Allow users to follow other users' viewport | ❌ Not implemented | Missing: Follow initiation and handling |
| **USER_VISIBLE_SCENE_BOUNDS** | Broadcast viewport bounds to followers | ❌ Not implemented | Missing: Viewport sync and zoom-to-follow logic |
| **User Following API** | `onUserFollow` callback | ❌ No follow API integration | Missing: Follow button in UI and backend support |
| **Viewport Change Sync** | Broadcast on scroll/zoom | ❌ No viewport change broadcasting | Missing: Scroll/zoom event tracking |

---

## 📊 Message Types Comparison

| Message Type | Spec Implementation | Current Implementation | Status |
| ------------ | ------------------ | ---------------------- | ------- |
| **INIT** | Send all elements when new user joins | ✅ `room_state` event sends all elements | **Implemented** |
| **UPDATE** | Delta updates (added/updated/deleted) | ✅ `element_change` event | **Implemented** |
| **MOUSE_LOCATION** | Cursor position with button state | ✅ `cursor_position` event | **Implemented** |
| **IDLE_STATUS** | Broadcast idle/active state | ❌ Not implemented | **Missing** |
| **USER_FOLLOW** | Initiate follow other user | ❌ Not implemented | **Missing** |
| **USER_VISIBLE_SCENE_BOUNDS** | Broadcast viewport bounds | ❌ Not implemented | **Missing** |

---

## 🔐 Encryption Implementation Gap

### Current (No Encryption):
```javascript
// Current: Plaintext message
socket.send(JSON.stringify({
    type: 'element_change',
    payload: elements
}));
```

### Required (AES-GCM):
```javascript
// Required: Encrypted message
import { encryptData } from "@excalidraw/excalidraw/data/encryption";

async function sendMessage(type, payload, encryptionKey) {
    const json = JSON.stringify({ type, payload });
    const encoded = new TextEncoder().encode(json);
    const { encryptedBuffer, iv } = await encryptData(encryptionKey, encoded);

    socket.send(JSON.stringify({
        type,
        payload: Array.from(encryptedBuffer), // Convert to array for JSON
        iv: Array.from(iv)
    }));
}
```

### Server-Side Relay Required:
```go
// Required: Relay encrypted messages without decryption
func (h *WSHandler) HandleEncryptedMessage(msg WSMessage) error {
    // DO NOT decrypt - just relay to all clients
    room.Broadcast <- &Message{
        Type:    msg.Type,
        Payload: msg.Payload, // Keep encrypted
        IV:      msg.IV,
        Exclude: msg.Sender,
    }
    return nil
}
```

---

## 🎯 Idle Detection Gap

### Spec Requirements:
```javascript
const IDLE_THRESHOLD = 30000; // 30 seconds
const ACTIVE_THRESHOLD = 5000;  // 5 seconds

function initializeIdleDetector() {
    document.addEventListener(EVENT.POINTER_MOVE, onPointerMove);
    document.addEventListener(EVENT.VISIBILITY_CHANGE, onVisibilityChange);
}

function onPointerMove() {
    if (idleTimeoutId) {
        clearTimeout(idleTimeoutId);
    }
    idleTimeoutId = setTimeout(reportIdle, IDLE_THRESHOLD);

    if (!activeIntervalId) {
        activeIntervalId = setInterval(reportActive, ACTIVE_THRESHOLD);
    }
}

function reportIdle() {
    socket.emit('IDLE_STATUS', { userState: 'IDLE', username });
}

function reportActive() {
    socket.emit('IDLE_STATUS', { userState: 'ACTIVE', username });
}
```

### Current Implementation:
```javascript
// ❌ Not implemented - No idle detection
```

---

## 👥 User Following Gap

### Spec Requirements:
```javascript
// Initiate follow
excalidrawAPI.onUserFollow((payload) => {
    socket.emit('user-follow', payload);
});

// Receive follow updates
socket.on('user-follow-room-change', (followedBy) => {
    excalidrawAPI.updateScene({
        appState: { followedBy: new Set(followedBy) }
    });
    relayVisibleSceneBounds({ force: true });
});

// Handle remote viewport bounds
socket.on('client-broadcast', async (encryptedData, iv) => {
    const decrypted = await decryptMessage(iv, encryptedData, roomKey);

    if (decrypted.type === 'USER_VISIBLE_SCENE_BOUNDS') {
        const { sceneBounds, socketId } = decrypted.payload;

        // Only accept bounds from user we're following
        if (appState.userToFollow?.socketId === socketId) {
            excalidrawAPI.updateScene({
                appState: zoomToFitBounds({
                    appState,
                    bounds: sceneBounds,
                    fitToViewport: true,
                    viewportZoomFactor: 1,
                }).appState
            });
        }
    }
});
```

### Current Implementation:
```javascript
// ❌ Not implemented - No user following
```

---

## 📈 Real-Time Collaboration Completion

| Category | Progress |
| --------- | -------- |
| **Core WebSocket Infrastructure** | 100% ✅ |
| **Message Handling** | 80% ⚠️ |
| **Connection Management** | 100% ✅ |
| **User Presence** | 100% ✅ |
| **Element Sync** | 100% ✅ |
| **Cursor Sync** | 100% ✅ |
| **Selection Sync** | 100% ✅ |
| **Encryption** | 0% ❌ |
| **Idle Detection** | 0% ❌ |
| **User Following** | 0% ❌ |

**Overall Real-Time Collaboration**: **~75% Complete** 📡

---

## 🚀 Implementation Roadmap

### Phase 8: Encryption (High Priority 🔴)
1. **Install dependencies**: `@excalidraw/excalidraw` package
2. **Generate room keys**: Random key generation on room creation
3. **Client-side encryption**: Implement `encryptMessage()` function
4. **Client-side decryption**: Implement `decryptMessage()` function
5. **Message envelope update**: Add `iv` field to WS message structure
6. **Server-side relay**: Modify to relay encrypted messages without decryption
7. **URL key sharing**: Share room key via URL hash

### Phase 9: Idle Detection (Medium Priority 🟡)
1. **Idle detector**: Implement pointer movement monitoring
2. **Threshold timers**: Add 30s idle and 5s active timers
3. **Idle status broadcast**: Send `IDLE_STATUS` message
4. **Visibility handling**: Handle tab visibility changes
5. **UI indicators**: Show active/idle status in participant list

### Phase 10: User Following (Medium Priority 🟡)
1. **Follow button**: Add follow button to participant list
2. **Follow initiation**: Send `USER_FOLLOW` message
3. **Follow tracking**: Track followed users in room state
4. **Viewport broadcast**: Send `USER_VISIBLE_SCENE_BOUNDS` on scroll/zoom
5. **Viewport receive**: Handle incoming viewport bounds
6. **Zoom-to-fit**: Implement viewport bounds sync for followers

---

## 📝 Notes

- **Encryption is critical** for production deployment to ensure privacy
- **Idle detection** improves UX by showing user status
- **User following** enhances collaborative experience
- All three gaps are optional for MVP but required for full spec compliance
