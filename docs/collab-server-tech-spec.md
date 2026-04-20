# Technical Specification: Excalidraw Realtime Collaboration Clone

## Architecture Overview

```
+-------------------------------------------------------------+
|                        CLIENT (Browser)                     |
|  Excalidraw Canvas + Encryption (AES-GCM) + WebSocket      |
+-------------------------------------------------------------+
                            |
              WebSocket (ws:// or wss://)
                            |
                            v
+-------------------------------------------------------------+
|                   GO WEBSOCKET SERVER                       |
|  +--------------+  +--------------+  +-------------------+  |
|  | HTTP Routes  |  |   Room Hub   |  |  WS Handler       |  |
|  | - /ws/:roomId|  | - Join/Leave |  |  - Message parse  |  |
|  | - POST /rooms|  | - Broadcast  |  |  - Route handlers |  |
|  +--------------+  +--------------+  +-------------------+  |
+-------------------------------------------------------------+
            |                              |
    PostgreSQL (Scene)              S3/MinIO (Files)
```

---

## 1. PostgreSQL Schema

### Database Tables

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

---

## 2. Go Server - Core Message Types

```go
// Message Types (matching Excalidraw)
const (
    MsgTypeInit            = "INIT"
    MsgTypeUpdate          = "UPDATE"
    MsgTypeMouseLocation   = "MOUSE_LOCATION"
    MsgTypeIdleStatus      = "IDLE_STATUS"
    MsgTypeUserFollow      = "USER_FOLLOW"
    MsgTypeVisibleBounds   = "USER_VISIBLE_SCENE_BOUNDS"
)

// WS Message Envelope
type WSMessage struct {
    Type    string          `json:"type"`
    Payload json.RawMessage `json:"payload"`
    IV      string          `json:"iv,omitempty"`
}

// Mouse Location
type MouseLocationPayload struct {
    SocketId          string    `json:"socketId"`
    Pointer           CursorPos `json:"pointer"`
    Button            string    `json:"button"`
    Username          string    `json:"username"`
    SelectedElementIds []string `json:"selectedElementIds"`
}

type CursorPos struct {
    X float64 `json:"x"`
    Y float64 `json:"y"`
}

// Update
type UpdatePayload struct {
    Elements     []json.RawMessage `json:"elements"`
    SceneVersion int64             `json:"sceneVersion"`
}
```

---

## 3. Room Hub

```go
type Hub struct {
    rooms      map[string]*Room
    register   chan *Client
    unregister chan *Client
    broadcast  chan *Message
    mutex      sync.RWMutex
}

func (h *Hub) Run() {
    for {
        select {
        case client := <-h.register:
            h.mutex.Lock()
            if h.rooms[client.RoomID] == nil {
                h.rooms[client.RoomID] = NewRoom(client.RoomID)
                go h.rooms[client.RoomID].Run()
            }
            h.rooms[client.RoomID].Register <- client
            h.mutex.Unlock()

        case client := <-h.unregister:
            if r := h.rooms[client.RoomID]; r != nil {
                r.Unregister <- client
            }

        case msg := <-h.broadcast:
            if r := h.rooms[msg.RoomID]; r != nil {
                r.Broadcast <- msg
            }
        }
    }
}
```

---

## 4. Room & Client

```go
type Room struct {
    ID        string
    Clients   map[*Client]bool
    Register  chan *Client
    Unregister chan *Client
    Broadcast chan *Message
    mutex     sync.RWMutex
}

type Client struct {
    ID       string
    RoomID   string
    Username string
    Conn     *websocket.Conn
    Send     chan *Message
}

// ReadPump handles incoming messages
func (c *Client) ReadPump(hub *Hub) {
    for {
        _, data, err := c.Conn.ReadMessage()
        if err != nil {
            break
        }
        var msg WSMessage
        json.Unmarshal(data, &msg)

        hub.broadcast <- &Message{
            RoomID:  c.RoomID,
            Type:    msg.Type,
            Payload: msg.Payload,
            Exclude: c,
        }
    }
}

// WritePump handles outgoing messages
func (c *Client) WritePump() {
    for {
        msg, ok := <-c.Send
        if !ok {
            c.Conn.Close()
            break
        }
        c.Conn.WriteJSON(msg)
    }
}
```

---

## 5. WebSocket Handler

```go
func (h *WSHandler) HandleWS(w http.ResponseWriter, r *http.Request) {
    vars := mux.Vars(r)
    roomID := vars["roomId"]

    conn, _ := upgrader.Upgrade(w, r, nil)
    username := r.URL.Query().Get("username")

    client := NewClient(uuid.New().String(), roomID, username, conn)
    h.Hub.Register <- client

    go client.WritePump()
    client.ReadPump(h.Hub)
}
```

---

## 6. REST API Routes

```go
router.HandleFunc("/ws/{roomId}", wsHandler.HandleWS)
router.HandleFunc("/api/rooms", roomHandler.CreateRoom).Methods("POST")
router.HandleFunc("/api/rooms/{roomId}", roomHandler.GetRoom).Methods("GET")
router.HandleFunc("/api/rooms/{roomId}/elements", roomHandler.GetElements).Methods("GET")
router.HandleFunc("/api/files/upload", fileHandler.Upload).Methods("POST")
router.HandleFunc("/api/files/{roomId}/{fileId}", fileHandler.Download).Methods("GET")
```

---

## 7. Persistence Layer

```go
func (p *PostgresClient) BatchSaveElements(roomID string, updates []ElementUpdate) error {
    tx, _ := p.db.Begin()
    stmt, _ := tx.Prepare(`
        INSERT INTO room_elements (room_id, element_id, data, version, updated_at)
        VALUES ($1, $2, $3, $4, NOW())
        ON CONFLICT (room_id, element_id)
        DO UPDATE SET data = $3, version = $4
    `)

    for _, u := range updates {
        stmt.Exec(roomID, u.ElementID, u.Data, u.Version)
    }
    return tx.Commit()
}

// Throttled persistence (every 3 seconds)
func (r *Room) StartPersistence(db *storage.Postgres, interval time.Duration) {
    ticker := time.NewTicker(interval)
    pending := make([]ElementUpdate, 0)

    go func() {
        for {
            select {
            case update := <-r.elementCh:
                pending = append(pending, update)
            case <-ticker.C:
                if len(pending) > 0 {
                    db.BatchSaveElements(r.ID, pending)
                    pending = pending[:0]
                }
            }
        }
    }()
}
```

---

## 8. Client-Side WebSocket (JavaScript)

```javascript
class CollabClient {
    constructor(roomId, roomKey, onMessage) {
        this.roomId = roomId;
        this.roomKey = roomKey;
        this.onMessage = onMessage;
    }

    connect() {
        const wsUrl = `ws://${host}/ws/${this.roomId}?username=${this.username}`;
        this.socket = new WebSocket(wsUrl);

        this.socket.onmessage = (event) => {
            const msg = JSON.parse(event.data);
            this.handleMessage(msg);
        };
    }

    // Throttled cursor (50ms)
    sendMouseMove(x, y, button, selectedIds) {
        this.throttle(50, () => {
            this.send({
                type: 'MOUSE_LOCATION',
                payload: { pointer: {x, y}, button, selectedElementIds: selectedIds }
            });
        })();
    }

    // Element sync
    sendElementUpdate(elements, sceneVersion) {
        this.send({
            type: 'UPDATE',
            payload: { elements, sceneVersion }
        });
    }

    // Send initial scene (when joining a room)
    sendInitialScene(elements) {
        this.send({
            type: 'INIT',
            payload: { elements }
        });
    }

    // Send idle status
    sendIdleStatus(status) {
        this.send({
            type: 'IDLE_STATUS',
            payload: { userState: status, username: this.username }
        });
    }

    // Send visible scene bounds (for user following)
    sendVisibleSceneBounds(bounds) {
        this.send({
            type: 'USER_VISIBLE_SCENE_BOUNDS',
            payload: { sceneBounds: bounds }
        });
    }

    disconnect() {
        this.socket?.close();
    }
}
```

---

## 9. Message Encryption (AES-GCM)

All WebSocket messages are encrypted using AES-GCM to ensure privacy.

### Encryption (Client -> Server)

```javascript
import { encryptData } from "@excalidraw/excalidraw/data/encryption";

async function encryptMessage(payload, encryptionKey) {
    const json = JSON.stringify(payload);
    const encoded = new TextEncoder().encode(json);
    const { encryptedBuffer, iv } = await encryptData(encryptionKey, encoded);
    return { encryptedBuffer, iv };
}

// Usage
const { encryptedBuffer, iv } = await encryptMessage(data, roomKey);
socket.emit('server-broadcast', roomId, encryptedBuffer, iv);
```

### Decryption (Server -> Client)

```javascript
import { decryptData } from "@excalidraw/excalidraw/data/encryption";

async function decryptMessage(iv, encryptedData, decryptionKey) {
    const decrypted = await decryptData(iv, encryptedData, decryptionKey);
    const decodedData = new TextDecoder("utf-8").decode(new Uint8Array(decrypted));
    return JSON.parse(decodedData);
}

// Socket message handler
socket.on('client-broadcast', async (encryptedData, iv) => {
    const decrypted = await decryptMessage(iv, encryptedData, roomKey);
    handleMessage(decrypted);
});
```

---

## 10. File Upload & Download Handling

### File Upload Workflow

```javascript
class FileManager {
    async saveFiles({ addedFiles }) {
        // Encode files for upload with encryption
        const encoded = await encodeFilesForUpload({
            files: addedFiles,
            encryptionKey: roomKey,
            maxBytes: FILE_UPLOAD_MAX_BYTES,
        });

        // Upload to Firebase Storage
        const { savedFiles, erroredFiles } = await saveFilesToFirebase({
            prefix: `${FIREBASE_STORAGE_PREFIXES.collabFiles}/${roomId}`,
            files: encoded,
        });

        return { savedFiles, erroredFiles };
    }
}
```

### File Download Workflow

```javascript
async function fetchImageFilesFromFirebase(opts) {
    const unfetchedImages = opts.elements
        .filter(element =>
            isInitializedImageElement(element) &&
            !fileManager.isFileTracked(element.fileId) &&
            element.status === "saved"
        )
        .map(element => element.fileId);

    return await fileManager.getFiles(unfetchedImages);
}
```

---

## 11. Room Lifecycle Management

### Creating a New Room

```javascript
async function startCollaboration(existingRoomLinkData = null) {
    let roomId, roomKey;

    if (existingRoomLinkData) {
        ({ roomId, roomKey } = existingRoomLinkData);
    } else {
        ({ roomId, roomKey } = await generateCollaborationLinkData());
        window.history.pushState(
            {},
            APP_NAME,
            getCollaborationLink({ roomId, roomKey })
        );
    }

    // Initialize Socket.IO connection
    const socket = socketIOClient(WS_SERVER_URL, {
        transports: ["websocket", "polling"],
    });

    // Open connection via Portal
    portal.open(socket, roomId, roomKey);
}
```

### Joining an Existing Room

```javascript
// When joining existing room, fetch initial scene
socket.on("first-in-room", async () => {
    const sceneData = await loadFromFirebase(roomId, roomKey, socket);
    if (sceneData) {
        initializeScene(sceneData);
    }
});

// When new user joins room
socket.on("new-user", async () => {
    // Broadcast current scene to new user
    portal.broadcastScene(
        WS_SUBTYPES.INIT,
        getSceneElementsIncludingDeleted(),
        true // syncAll
    );
});
```

### Stopping Collaboration

```javascript
function stopCollaboration(keepRemoteState = true) {
    // Cancel pending operations
    queueBroadcastAllElements.cancel();
    queueSaveToFirebase.cancel();

    // Save current state to Firebase
    saveCollabRoomToFirebase(getSyncableElements(elements));

    // Close socket connection
    portal.close();

    // Reset state
    if (!keepRemoteState) {
        resetBrowserStateVersions();
        window.history.pushState({}, APP_NAME, window.location.origin);
    }
}
```

---

## 12. Error Handling & Reconnection

### Socket Connection Errors

```javascript
socket.once("connect_error", (error) => {
    console.error("WebSocket connection failed:", error);

    // Fallback: initialize room via Firebase
    initializeRoom({
        fetchScene: true,
        roomLinkData: existingRoomLinkData
    }).then(scene => {
        scenePromise.resolve(scene);
    });
});
```

### Decryption Errors

```javascript
async function decryptPayload(iv, encryptedData, decryptionKey) {
    try {
        const decrypted = await decryptData(iv, encryptedData, decryptionKey);
        const decodedData = new TextDecoder("utf-8").decode(new Uint8Array(decrypted));
        return JSON.parse(decodedData);
    } catch (error) {
        window.alert(t("alerts.decryptFailed"));
        console.error(error);
        return { type: WS_SUBTYPES.INVALID_RESPONSE };
    }
}
```

### Firebase Save Errors

```javascript
async function saveCollabRoomToFirebase(syncableElements) {
    try {
        await saveToFirebase(portal, syncableElements, appState);
    } catch (error) {
        const errorMessage = /is longer than.*?bytes/.test(error.message)
            ? t("errors.collabSaveFailed_sizeExceeded")
            : t("errors.collabSaveFailed");

        setErrorIndicator(errorMessage);
        setErrorDialog(errorMessage);
    }
}
```

---

## 13. Performance Optimizations

### Throttled Cursor Updates (50ms)

```javascript
onPointerUpdate = throttle((payload) => {
    payload.pointersMap.size < 2 &&
        socket &&
        broadcastMouseLocation(payload);
}, CURSOR_SYNC_TIMEOUT); // 50ms
```

### Throttled Scene Sync (3s)

```javascript
queueBroadcastAllElements = throttle(() => {
    portal.broadcastScene(
        WS_SUBTYPES.UPDATE,
        getSceneElementsIncludingDeleted(),
        true
    );
}, SYNC_FULL_SCENE_INTERVAL_MS); // 3000ms
```

### Throttled Firebase Save (3s)

```javascript
queueSaveToFirebase = throttle(() => {
    if (portal.socketInitialized) {
        saveCollabRoomToFirebase(getSyncableElements(elements));
    }
}, SYNC_FULL_SCENE_INTERVAL_MS, { leading: false });
```

### Selective Element Broadcasting

```javascript
function broadcastScene(updateType, elements, syncAll) {
    // Only broadcast changed elements
    const syncableElements = elements.reduce((acc, element) => {
        if (
            syncAll ||
            !broadcastedElementVersions.has(element.id) ||
            element.version > broadcastedElementVersions.get(element.id)
        ) {
            acc.push(element);
        }
        return acc;
    }, []);

    // Track broadcasted versions
    for (const element of syncableElements) {
        broadcastedElementVersions.set(element.id, element.version);
    }

    await _broadcastSocketData({ type: updateType, payload: { elements: syncableElements } });
}
```

### RAF-Based Throttling for Viewport Updates

```javascript
const throttledRelayUserViewportBounds = throttleRAF(
    relayVisibleSceneBounds
);

excalidrawAPI.onScrollChange(() => throttledRelayUserViewportBounds());
```

---

## 14. Idle Detection

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
    onIdleStateChange(UserIdleState.IDLE);
    if (activeIntervalId) {
        clearInterval(activeIntervalId);
    }
}

function reportActive() {
    onIdleStateChange(UserIdleState.ACTIVE);
}
```

---

## 15. User Following Feature

### Initiate Follow

```javascript
excalidrawAPI.onUserFollow((payload) => {
    socket && socket.emit("user-follow", payload);
});
```

### Receive Follow Updates

```javascript
socket.on("user-follow-room-change", (followedBy: SocketId[]) => {
    excalidrawAPI.updateScene({
        appState: { followedBy: new Set(followedBy) }
    });

    // Broadcast viewport bounds to followers
    relayVisibleSceneBounds({ force: true });
});
```

### Handle Remote Viewport Bounds

```javascript
socket.on("client-broadcast", async (encryptedData, iv) => {
    const decrypted = await decryptMessage(iv, encryptedData, roomKey);

    if (decrypted.type === WS_SUBTYPES.USER_VISIBLE_SCENE_BOUNDS) {
        const { sceneBounds, socketId } = decrypted.payload;

        // Only accept bounds from the user we're following
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

---

## 16. Constants & Configuration

```javascript
// WebSocket timeouts
const CURSOR_SYNC_TIMEOUT = 50;
const INITIAL_SCENE_UPDATE_TIMEOUT = 10000;
const LOAD_IMAGES_TIMEOUT = 500;
const SYNC_FULL_SCENE_INTERVAL_MS = 3000;

// File upload limits
const FILE_UPLOAD_MAX_BYTES = 2 * 1024 * 1024; // 2MB per file

// Idle detection
const IDLE_THRESHOLD = 30000;  // 30 seconds
const ACTIVE_THRESHOLD = 5000;   // 5 seconds

// Firebase prefixes
const FIREBASE_STORAGE_PREFIXES = {
    collabFiles: "files/rooms",
    shareLinkFiles: "files/shareLinks"
};
```

---

## 17. Integration with Excalidraw App

### Collab API Interface

```javascript
export interface CollabAPI {
    isCollaborating: () => boolean;
    onPointerUpdate: (payload) => void;
    startCollaboration: (roomLinkData) => Promise<SceneData>;
    stopCollaboration: (keepRemoteState?) => void;
    syncElements: (elements) => void;
    fetchImageFilesFromFirebase: (opts) => Promise<FilesResult>;
    setUsername: (username) => void;
    getUsername: () => string;
    getActiveRoomLink: () => string | null;
    setCollabError: (errorMessage) => void;
}
```

### Usage in App.tsx

```javascript
const [collabAPI] = useAtom(collabAPIAtom);
const [isCollaborating] = useAtom(isCollaboratingAtom);

// Start collaboration
const roomLinkData = getCollaborationLinkData(window.location.href);
if (roomLinkData) {
    const scene = await collabAPI.startCollaboration(roomLinkData);
    excalidrawAPI.updateScene(scene);
}

// Sync element changes
const onChange = (elements, appState, files) => {
    if (collabAPI?.isCollaborating()) {
        collabAPI.syncElements(elements);
    }
};

// Pointer updates for cursor sync
<Excalidraw
    onPointerUpdate={collabAPI?.onPointerUpdate}
    isCollaborating={isCollaborating}
/>
```

---

## 18. Security Considerations

### End-to-End Encryption

- All WebSocket messages encrypted using AES-GCM
- Room key generated randomly and shared via URL hash
- Server only relays encrypted messages without decryption

### Access Control

- Room ID and key required for connection
- Only participants with valid key can decrypt messages
- Deleted elements not exposed in initial scene

### Data Privacy

- User state (cursor, selection) broadcast as volatile messages
- Scene data persisted only in Firebase (not in server memory)
- Username stored locally and shared with collaborators
