# Frontend Enhancements for Backend Integration

## Overview

This document outlines the frontend changes required to integrate with the new Go-based backend for real-time collaboration features.

**Backend Status**: Phase 1 & 2 Complete
**Last Updated**: 2026-01-31

---

## Required Frontend Changes

### 1. WebSocket Connection Management

#### Current State (if any):
- Check if frontend has existing WebSocket implementation
- Identify current real-time sync approach

#### Required Implementation:

**WebSocket Service** (`src/services/websocket.ts`):
```typescript
interface WSMessage {
  type: string;
  payload: any;
}

interface WSConfig {
  url: string;
  reconnectInterval?: number;
  maxReconnectAttempts?: number;
}

class WebSocketService {
  private ws: WebSocket | null = null;
  private reconnectAttempts = 0;
  private messageHandlers: Map<string, Function[]> = new Map();

  connect(config: WSConfig): Promise<void> {
    return new Promise((resolve, reject) => {
      try {
        this.ws = new WebSocket(config.url);

        this.ws.onopen = () => {
          console.log('WebSocket connected');
          this.reconnectAttempts = 0;
          resolve();
        };

        this.ws.onmessage = (event) => {
          const message: WSMessage = JSON.parse(event.data);
          this.handleMessage(message);
        };

        this.ws.onerror = (error) => {
          console.error('WebSocket error:', error);
          reject(error);
        };

        this.ws.onclose = () => {
          console.log('WebSocket disconnected');
          this.attemptReconnect(config);
        };
      } catch (error) {
        reject(error);
      }
    });
  }

  private handleMessage(message: WSMessage) {
    const handlers = this.messageHandlers.get(message.type) || [];
    handlers.forEach(handler => handler(message.payload));
  }

  on(eventType: string, handler: Function) {
    if (!this.messageHandlers.has(eventType)) {
      this.messageHandlers.set(eventType, []);
    }
    this.messageHandlers.get(eventType)!.push(handler);
  }

  send(type: string, payload: any) {
    if (this.ws && this.ws.readyState === WebSocket.OPEN) {
      this.ws.send(JSON.stringify({ type, payload }));
    }
  }

  disconnect() {
    if (this.ws) {
      this.ws.close();
      this.ws = null;
    }
  }

  private attemptReconnect(config: WSConfig) {
    if (this.reconnectAttempts < (config.maxReconnectAttempts || 5)) {
      this.reconnectAttempts++;
      setTimeout(() => {
        console.log(`Reconnecting... Attempt ${this.reconnectAttempts}`);
        this.connect(config);
      }, config.reconnectInterval || 3000);
    }
  }
}

export const wsService = new WebSocketService();
```

---

### 2. Room Management

#### Required Features:

**Join Room** (`src/services/room.ts`):
```typescript
interface JoinRoomPayload {
  roomId: string;
  username: string;
}

interface RoomState {
  elements: Element[];
  participants: Participant[];
}

class RoomService {
  private currentRoom: string | null = null;

  async joinRoom(roomId: string, username: string): Promise<void> {
    // Connect to WebSocket
    await wsService.connect({
      url: `ws://localhost:8080/ws`,
    });

    // Send join_room message
    wsService.send('join_room', {
      roomId,
      username,
    } as JoinRoomPayload);

    this.currentRoom = roomId;

    // Listen for room_state
    return new Promise((resolve) => {
      wsService.on('room_state', (state: RoomState) => {
        // Handle initial room state
        this.handleRoomState(state);
        resolve();
      });
    });
  }

  leaveRoom() {
    if (this.currentRoom) {
      wsService.send('leave_room', {
        roomId: this.currentRoom,
      });
      this.currentRoom = null;
    }
    wsService.disconnect();
  }

  private handleRoomState(state: RoomState) {
    // Update local state with elements
    // Update participants list
    // Render elements on canvas
  }
}

export const roomService = new RoomService();
```

---

### 3. Element Synchronization

#### Required Features:

**Element Sync** (`src/services/elementSync.ts`):
```typescript
interface ElementChanges {
  added?: Element[];
  updated?: Element[];
  deleted?: string[];
}

class ElementSyncService {
  // Send element changes to backend
  sendElementChanges(changes: ElementChanges) {
    wsService.send('update_elements', {
      roomId: roomService.getCurrentRoom(),
      changes,
    });
  }

  // Handle element updates from other users
  onElementsUpdated(callback: (changes: ElementChanges) => void) {
    wsService.on('elements_updated', (payload: any) => {
      callback(payload.changes);
    });
  }
}

export const elementSyncService = new ElementSyncService();
```

**Integration with Excalidraw**:
```typescript
// In your main Excalidraw component
import { elementSyncService } from './services/elementSync';

// Listen for element changes
useEffect(() => {
  const unsubscribe = elementSyncService.onElementsUpdated((changes) => {
    // Apply changes to Excalidraw state
    if (changes.added) {
      // Add new elements
    }
    if (changes.updated) {
      // Update existing elements
    }
    if (changes.deleted) {
      // Delete elements
    }
  });

  return unsubscribe;
}, []);

// Send local changes
const handleElementChange = (elements: Element[]) => {
  // Calculate delta (what changed)
  const changes = calculateDelta(elements);
  elementSyncService.sendElementChanges(changes);
};
```

---

### 4. User Awareness (Cursors & Presence)

#### Required Features:

**Cursor Tracking** (`src/services/cursor.ts`):
```typescript
interface CursorPosition {
  x: number;
  y: number;
}

interface CursorUpdate {
  userId: string;
  username: string;
  color: string;
  position: CursorPosition;
}

class CursorService {
  private cursorUpdateInterval: NodeJS.Timeout | null = null;
  private lastPosition: CursorPosition = { x: 0, y: 0 };

  startTracking() {
    // Send cursor position every 100ms (throttled)
    this.cursorUpdateInterval = setInterval(() => {
      const currentPos = this.getCurrentCursorPosition();
      if (this.hasPositionChanged(currentPos)) {
        wsService.send('cursor_move', {
          roomId: roomService.getCurrentRoom(),
          position: currentPos,
        });
        this.lastPosition = currentPos;
      }
    }, 100);
  }

  stopTracking() {
    if (this.cursorUpdateInterval) {
      clearInterval(this.cursorUpdateInterval);
      this.cursorUpdateInterval = null;
    }
  }

  onCursorUpdated(callback: (update: CursorUpdate) => void) {
    wsService.on('cursor_updated', (payload: CursorUpdate) => {
      callback(payload);
    });
  }

  private getCurrentCursorPosition(): CursorPosition {
    // Get cursor position from Excalidraw
    // This depends on Excalidraw's API
    return { x: 0, y: 0 };
  }

  private hasPositionChanged(newPos: CursorPosition): boolean {
    return newPos.x !== this.lastPosition.x || newPos.y !== this.lastPosition.y;
  }
}

export const cursorService = new CursorService();
```

**Participant Display** (`src/components/Participants.tsx`):
```typescript
interface Participant {
  id: string;
  username: string;
  color: string;
}

function ParticipantsList({ participants }: { participants: Participant[] }) {
  return (
    <div className="participants-list">
      <h3>Participants ({participants.length})</h3>
      <ul>
        {participants.map(p => (
          <li key={p.id}>
            <span className="color-dot" style={{ backgroundColor: p.color }} />
            {p.username}
          </li>
        ))}
      </ul>
    </div>
  );
}
```

**Remote Cursor Rendering** (`src/components/RemoteCursors.tsx`):
```typescript
interface RemoteCursor {
  userId: string;
  username: string;
  color: string;
  position: { x: number; y: number };
}

function RemoteCursors({ cursors }: { cursors: RemoteCursor[] }) {
  return (
    <svg className="remote-cursors">
      {cursors.map(cursor => (
        <g key={cursor.userId} transform={`translate(${cursor.position.x}, ${cursor.position.y})`}>
          <path
            d="M0,0 L0,20 L15,10 Z"
            fill={cursor.color}
            opacity="0.7"
          />
          <text x="15" y="15" fill={cursor.color} fontSize="12">
            {cursor.username}
          </text>
        </g>
      ))}
    </svg>
  );
}
```

---

### 5. URL Routing & Room Links

#### Required Features:

**URL Parameter Handling** (`src/utils/roomURL.ts`):
```typescript
// Parse room ID from URL
export function getRoomIdFromURL(): string | null {
  const params = new URLSearchParams(window.location.search);
  return params.get('room');
}

// Update URL with room ID
export function setRoomIdInURL(roomId: string) {
  const url = new URL(window.location.href);
  url.searchParams.set('room', roomId);
  window.history.pushState({}, '', url.toString());
}

// Clear room ID from URL
export function clearRoomIdFromURL() {
  const url = new URL(window.location.href);
  url.searchParams.delete('room');
  window.history.pushState({}, '', url.toString());
}

// Generate shareable link
export function generateShareableLink(roomId: string): string {
  const url = new URL(window.location.href);
  url.searchParams.set('room', roomId);
  return url.toString();
}
```

**Auto-Join on Page Load** (`src/App.tsx`):
```typescript
import { getRoomIdFromURL } from './utils/roomURL';
import { roomService } from './services/room';

function App() {
  useEffect(() => {
    const roomId = getRoomIdFromURL();
    if (roomId) {
      // Prompt for username or use saved username
      const username = localStorage.getItem('username') || 'Guest';
      roomService.joinRoom(roomId, username);
    }
  }, []);

  // ... rest of app
}
```

---

### 6. UI Components

#### Required UI Elements:

**Join Room Dialog** (`src/components/JoinRoomDialog.tsx`):
```typescript
function JoinRoomDialog({ onJoin }: { onJoin: (roomId: string, username: string) => void }) {
  const [roomId, setRoomId] = useState('');
  const [username, setUsername] = useState('');

  const handleJoin = () => {
    if (roomId && username) {
      localStorage.setItem('username', username);
      onJoin(roomId, username);
    }
  };

  return (
    <div className="join-room-dialog">
      <h2>Join a Room</h2>
      <input
        type="text"
        placeholder="Room ID"
        value={roomId}
        onChange={(e) => setRoomId(e.target.value)}
      />
      <input
        type="text"
        placeholder="Your Name"
        value={username}
        onChange={(e) => setUsername(e.target.value)}
      />
      <button onClick={handleJoin}>Join</button>
    </div>
  );
}
```

**Share Link Button** (`src/components/ShareButton.tsx`):
```typescript
function ShareButton({ roomId }: { roomId: string }) {
  const handleShare = async () => {
    const link = generateShareableLink(roomId);
    if (navigator.share) {
      await navigator.share({
        title: 'Join my whiteboard',
        url: link,
      });
    } else {
      navigator.clipboard.writeText(link);
      alert('Link copied to clipboard!');
    }
  };

  return (
    <button onClick={handleShare}>
      Share Room
    </button>
  );
}
```

---

## Integration Checklist

### Phase 1 & 2 Integration:
- [ ] Create WebSocket service
- [ ] Create room service
- [ ] Implement join/leave room functionality
- [ ] Display participants list
- [ ] Handle user join/leave notifications
- [ ] Test WebSocket connection

### Phase 3 Integration (Element Sync):
- [ ] Implement element delta calculation
- [ ] Send element changes to backend
- [ ] Handle incoming element updates
- [ ] Apply remote changes to canvas

### Phase 4 Integration (User Awareness):
- [ ] Implement cursor tracking
- [ ] Render remote cursors
- [ ] Implement throttling for cursor updates
- [ ] Add idle detection

### Phase 5 Integration (Room Links):
- [ ] Implement URL parameter parsing
- [ ] Auto-join on page load
- [ ] Add share button
- [ ] Test link sharing

---

## Testing Checklist

### Manual Testing:
- [ ] Connect to WebSocket server
- [ ] Join a room
- [ ] See other participants
- [ ] Draw elements and see them sync
- [ ] Move cursor and see it on other clients
- [ ] Leave room and see notification
- [ ] Share room link
- [ ] Open shared link and auto-join

### Multi-User Testing:
- [ ] Open 3+ browser windows
- [ ] Join same room from all windows
- [ ] Draw from one window, verify sync
- [ ] Move cursors, verify visibility
- [ ] Leave from one window, verify notification

---

## Environment Configuration

### Development:
```typescript
const config = {
  wsUrl: 'ws://localhost:8080/ws',
  apiUrl: 'http://localhost:8080/api',
};
```

### Production:
```typescript
const config = {
  wsUrl: 'wss://your-domain.com/ws',
  apiUrl: 'https://your-domain.com/api',
};
```

---

## Error Handling

**WebSocket Errors**:
```typescript
wsService.on('error', (payload: { message: string; code?: string }) => {
  if (payload.code === 'room_full') {
    alert('Room is full. Please try another room.');
  } else if (payload.code === 'room_not_found') {
    alert('Room not found.');
  } else {
    console.error('WebSocket error:', payload.message);
  }
});
```

**Reconnection Logic**:
```typescript
wsService.on('close', () => {
  // Show reconnecting indicator
  showNotification('Disconnected. Reconnecting...');

  // Attempt to reconnect
  setTimeout(() => {
    roomService.joinRoom(currentRoom, currentUsername);
  }, 3000);
});
```

---

## Performance Considerations

1. **Throttle Cursor Updates**: Send at most 10-20 updates per second
2. **Batch Element Updates**: Combine rapid changes into single messages
3. **Optimize Renders**: Use React.memo for cursor components
4. **Lazy Loading**: Load participant avatars/images on demand

---

## Security Notes

1. **Room ID Validation**: Validate room ID format before joining
2. **Username Sanitization**: Sanitize usernames to prevent XSS
3. **CORS Configuration**: Ensure backend allows frontend origin
4. **Rate Limiting**: Implement client-side rate limiting for messages

---

## Next Steps

1. **Create WebSocket Service**: Set up base WebSocket connection
2. **Implement Room Join/Leave**: Basic room functionality
3. **Add Participant List**: Show online users
4. **Test with Backend**: Connect to running Go backend
5. **Implement Element Sync**: Real-time drawing collaboration
6. **Add Cursor Tracking**: Show other users' cursors
7. **Polish UI**: Add loading states, error messages, etc.
