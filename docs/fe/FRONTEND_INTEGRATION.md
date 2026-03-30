# Frontend Integration Guide - Backend Phase 3-6

## Overview

Backend Phase 3 (Element Synchronization) and Phase 4 (User Awareness/Cursors) are fully implemented. This guide explains how to integrate the frontend with these features.

## WebSocket Events Reference

### Client → Server Messages

```typescript
// Join a room
{
  type: "join_room",
  payload: {
    roomId: string,      // 10-character room ID
    username: string     // User's display name
  }
}

// Leave current room
{
  type: "leave_room",
  payload: {
    roomId: string
  }
}

// Send element changes (delta updates)
{
  type: "update_elements",
  payload: {
    roomId: string,
    changes: {
      added?: Element[],      // New elements created
      updated?: Element[],    // Modified elements
      deleted?: string[]      // IDs of deleted elements
    }
  }
}

// Update cursor position
{
  type: "cursor_move",
  payload: {
    roomId: string,
    position: {
      x: number,
      y: number
    }
  }
}
```

### Server → Client Messages

```typescript
// Initial room state (sent when joining)
{
  type: "room_state",
  payload: {
    elements: Element[],
    participants: User[]
  }
}

// User joined notification
{
  type: "user_joined",
  payload: {
    userId: string,
    username: string,
    color: string      // Auto-assigned by backend
  }
}

// User left notification
{
  type: "user_left",
  payload: {
    userId: string
  }
}

// Elements updated by another user
{
  type: "elements_updated",
  payload: {
    userId: string,      // ID of user who made changes
    changes: {
      added?: Element[],
      updated?: Element[],
      deleted?: string[]
    }
  }
}

// Cursor position updated
{
  type: "cursor_updated",
  payload: {
    userId: string,
    username: string,
    color: string,
    position: {
      x: number,
      y: number
    }
  }
}

// Error message
{
  type: "error",
  payload: {
    message: string,    // Human-readable error
    code?: string       // Error code (room_full, rate_limit_exceeded, etc.)
  }
}
```

## Backend Limits & Validation

### Rate Limiting
- **Element updates**: 20 messages per second per connection
- **Window limit**: 100 messages per 10-second window per connection
- **Cursor updates**: 20 updates per second per connection

### Element Validation
- **Max elements per room**: 5,000
- **Max element size**: 10KB per element
- **Valid element types**: rectangle, ellipse, arrow, line, freedraw, text, image
- **Coordinate range**: 0 to 100,000 for X and Y
- **Text max length**: 1,000 characters

### Room Limits
- **Max participants**: 50 per room (configurable)
- **Inactive room cleanup**: 1 hour timeout (rooms with 0 participants)

### Cursor Limits
- **Inactive cursor timeout**: 5 seconds
- **Auto-cleanup**: Cursors removed after inactivity

## Frontend Integration

### 1. Element Synchronization Integration

The frontend's `elementSyncService.ts` is already created. Here's how to integrate it with Excalidraw:

```typescript
// In Whiteboard.tsx or your main component
import { elementSyncService } from './services/elementSyncService';

function Whiteboard() {
  const excalidrawAPI = useRef<ExcalidrawAPI>(null);

  // Send local element changes to backend
  const handleElementChange = useCallback((
    elements: readonly OrderedExcalidrawElement[],
    appState: AppState,
  ) => {
    if (!excalidrawAPI.current) return;

    // Get previous state to calculate delta
    const prevElements = excalidrawAPI.current.getSceneElements();

    // Calculate changes
    const changes = calculateElementChanges(prevElements, elements);

    // Send to backend (rate limited automatically)
    elementSyncService.sendElementChanges(changes);
  }, []);

  // Handle remote element updates
  useEffect(() => {
    const unsubscribe = elementSyncService.onElementsUpdated((payload) => {
      if (!excalidrawAPI.current) return;

      const { userId, changes } = payload;
      console.log('📨 Remote element update', { userId, changes });

      // Apply remote changes to canvas
      if (changes.added && changes.added.length > 0) {
        excalidrawAPI.current.updateScene({
          elements: changes.added as any[],
        });
      }

      if (changes.updated && changes.updated.length > 0) {
        excalidrawAPI.current.updateScene({
          elements: changes.updated as any[],
        });
      }

      if (changes.deleted && changes.deleted.length > 0) {
        const currentElements = excalidrawAPI.current.getSceneElements();
        const filtered = currentElements.filter(el =>
          !changes.deleted.includes(el.id)
        );
        excalidrawAPI.current.updateScene({
          elements: filtered,
        });
      }
    });

    return unsubscribe;
  }, []);

  return (
    <Excalidraw
      onChange={handleElementChange}
      excalidrawAPI={(api) => { excalidrawAPI.current = api; }}
    />
  );
}

// Helper to calculate delta changes
function calculateElementChanges(
  prev: readonly OrderedExcalidrawElement[],
  current: readonly OrderedExcalidrawElement[],
): ElementChanges {
  const prevIds = new Set(prev.map(el => el.id));
  const currentIds = new Set(current.map(el => el.id));

  const added: any[] = [];
  const updated: any[] = [];
  const deleted: string[] = [];

  // Find new elements
  for (const el of current) {
    if (!prevIds.has(el.id)) {
      added.push(el);
    }
  }

  // Find updated elements (compare by ID)
  for (const el of current) {
    const prevEl = prev.find(p => p.id === el.id);
    if (prevEl && !elementsEqual(prevEl, el)) {
      updated.push(el);
    }
  }

  // Find deleted elements
  for (const el of prev) {
    if (!currentIds.has(el.id)) {
      deleted.push(el.id);
    }
  }

  return { added, updated, deleted };
}

function elementsEqual(a: any, b: any): boolean {
  return JSON.stringify(a) === JSON.stringify(b);
}
```

### 2. Cursor Tracking Integration

The frontend's `cursorService.ts` is already created. Here's how to integrate it:

```typescript
// In Whiteboard.tsx or your main component
import { cursorService } from './services/cursorService';

function Whiteboard() {
  const excalidrawAPI = useRef<ExcalidrawAPI>(null);
  const [remoteCursors, setRemoteCursors] = useState<RemoteCursor[]>([]);

  // Start cursor tracking when component mounts
  useEffect(() => {
    if (!excalidrawAPI.current) return;

    cursorService.startTracking();

    return () => {
      cursorService.stopTracking();
    };
  }, []);

  // Listen for remote cursor updates
  useEffect(() => {
    const unsubscribe = cursorService.onCursorUpdated((payload) => {
      const { userId, username, color, position } = payload;

      setRemoteCursors(prev => {
        const existing = prev.find(c => c.userId === userId);
        if (existing) {
          // Update existing cursor
          return prev.map(c =>
            c.userId === userId
              ? { ...c, position, updated: Date.now() }
              : c
          );
        } else {
          // Add new cursor
          return [
            ...prev,
            { userId, username, color, position, updated: Date.now() },
          ];
        }
      });
    });

    // Clean up inactive cursors (5 seconds timeout matches backend)
    const cleanupInterval = setInterval(() => {
      setRemoteCursors(prev => {
        const now = Date.now();
        return prev.filter(c => now - c.updated < 5000);
      });
    }, 1000);

    return () => {
      unsubscribe();
      clearInterval(cleanupInterval);
    };
  }, []);

  return (
    <>
      <Excalidraw
        excalidrawAPI={(api) => { excalidrawAPI.current = api; }}
      />
      <RemoteCursors cursors={remoteCursors} />
    </>
  );
}
```

### 3. Remote Cursor Component

The `RemoteCursors.tsx` component is already created. Here's how it works:

```typescript
// src/components/RemoteCursors.tsx
import React from 'react';

interface RemoteCursor {
  userId: string;
  username: string;
  color: string;
  position: { x: number; y: number };
  updated: number;
}

interface RemoteCursorsProps {
  cursors: RemoteCursor[];
}

export default function RemoteCursors({ cursors }: RemoteCursorsProps) {
  return (
    <svg
      className="remote-cursors-overlay"
      style={{
        position: 'fixed',
        top: 0,
        left: 0,
        width: '100%',
        height: '100%',
        pointerEvents: 'none',
        zIndex: 1000,
      }}
    >
      {cursors.map(cursor => (
        <g
          key={cursor.userId}
          transform={`translate(${cursor.position.x}, ${cursor.position.y})`}
        >
          {/* Cursor arrow */}
          <path
            d="M0,0 L0,20 L15,10 Z"
            fill={cursor.color}
            opacity="0.8"
          />
          {/* Username label */}
          <text
            x={15}
            y={15}
            fill={cursor.color}
            fontSize="12"
            fontWeight="bold"
            opacity="0.9"
          >
            {cursor.username}
          </text>
        </g>
      ))}
    </svg>
  );
}
```

## Error Handling

### Rate Limit Exceeded
```typescript
// In elementSyncService.ts
wsService.on('error', (payload: { message: string; code: string }) => {
  if (payload.code === 'rate_limit_exceeded') {
    alert('Please slow down. You\'re sending too many updates.');
  } else if (payload.code === 'room_full') {
    alert('This room is full. Try another room.');
  } else if (payload.code === 'element_limit_exceeded') {
    alert('This room has reached the maximum element limit (5000).');
  } else {
    console.error('WebSocket error:', payload.message);
  }
});
```

### Connection Errors
```typescript
// In roomService.ts
wsService.on('close', () => {
  showNotification('Disconnected. Reconnecting...');

  // Attempt to reconnect after 3 seconds
  setTimeout(() => {
    roomService.joinRoom(currentRoomId, username);
  }, 3000);
});
```

## Performance Optimization

### 1. Debounce Element Updates

Element updates are automatically debounced in the backend. On the frontend, consider batching:

```typescript
// Use a small delay before sending updates
const debouncedSend = useMemo(
  () => debounce((changes: ElementChanges) => {
    elementSyncService.sendElementChanges(changes);
  }, 100), // 100ms delay
  [],
);
```

### 2. Throttle Cursor Updates

Cursor updates are throttled to 20/sec in the backend. The frontend's `cursorService.ts` already handles this:

```typescript
// From cursorService.ts
startTracking() {
  this.cursorUpdateInterval = setInterval(() => {
    const currentPos = this.getCurrentCursorPosition();
    if (this.hasPositionChanged(currentPos)) {
      wsService.send('cursor_move', {
        roomId: roomService.getCurrentRoom(),
        position: currentPos,
      });
      this.lastPosition = currentPos;
    }
  }, 50); // 50ms = 20 updates per second
}
```

### 3. Optimize Re-renders

Use React.memo for cursor components:

```typescript
const RemoteCursor = React.memo(({ cursor }: { cursor: RemoteCursor }) => {
  // Render cursor
});
```

## Testing Checklist

### Manual Testing
- [ ] Connect to WebSocket server
- [ ] Join a room and receive room_state
- [ ] Create elements in one window and see them sync to another
- [ ] Update elements and see changes sync
- [ ] Delete elements and see removal sync
- [ ] Move cursor in one window and see it in another
- [ ] Leave room and see notification in other windows
- [ ] Test rate limiting (rapid element updates)
- [ ] Test room capacity (join with 50+ users)
- [ ] Test element limit (add 5000+ elements)

### Multi-User Testing
- [ ] Open 3+ browser windows
- [ ] Join same room from all windows
- [ ] Draw from one window, verify sync to all
- [ ] Move cursors, verify visibility to all
- [ ] Leave from one window, verify notification
- [ ] Test concurrent edits (same element modified by multiple users)

## Known Issues

### Conflict Resolution
Currently, the backend uses **last-write-wins** for element updates. If two users edit the same element simultaneously:
- The last update received by the server wins
- No conflict resolution UI exists yet
- Future phases will add collaboration features (selection awareness, locking)

### Element Loss Prevention
- Delta updates minimize data loss
- But concurrent edits to same element can still cause data loss
- Consider implementing operational transformation (OT) or CRDTs for production

## Next Steps

1. **Integrate Excalidraw onChange**: Hook elementSyncService to Excalidraw's onChange handler
2. **Test multi-user scenarios**: Verify synchronization works with 3+ users
3. **Add conflict UI**: Visual feedback when multiple users edit same element (Phase 6)
4. **Implement element selection awareness**: Track what users have selected (Phase 6)
5. **Optimize performance**: Test with 100+ elements and multiple cursors

## Backend API Endpoints

### Health Check
```bash
GET /health
Response: {
  status: "healthy",
  version: "1.0.0",
  timestamp: "2026-03-12T11:00:00Z"
}
```

### Stats Endpoint
```bash
GET /api/stats
Response: {
  total_rooms: 5,
  total_participants: 12,
  timestamp: "2026-03-12T11:00:00Z"
}
```

### WebSocket Endpoint
```bash
WS /ws
Protocol: JSON messages with "type" and "payload"
```

## Phase 5: Room Link Management (✅ Backend Complete)

### Backend Implementation
- ✅ **HTTP Endpoint**: `GET /api/rooms/:id/link` - Returns shareable room URL
- ✅ **WebSocket Event**: `get_room_link` - Returns share URL + optional QR code

### WebSocket Events
```typescript
// Client → Server
{
  type: "get_room_link",
  payload: { roomId: string }
}

// Server → Client
{
  type: "room_link",
  payload: {
    shareUrl: string,    // Format: {origin}?room={roomId}
    qrCode?: string       // Optional: Base64 encoded QR code
  }
}
```

### Frontend Integration
```typescript
// Get room link from backend
const getRoomLink = async (roomId: string): Promise<string> => {
  const response = await fetch(`http://localhost:8080/api/rooms/${roomId}/link`);
  const data = await response.json();
  return data.shareUrl;
};

// Copy room link to clipboard
const copyRoomLink = async (shareUrl: string): Promise<void> => {
  try {
    await navigator.clipboard.writeText(shareUrl);
    alert('Room link copied to clipboard!');
  } catch (err) {
    console.error('Failed to copy:', err);
  }
};
```

### Success Criteria
- [x] Users can share room links via HTTP API
- [ ] QR code generation (optional)
- [ ] Frontend integration completed

---

## Phase 6: Selection & Interaction Awareness (✅ Backend Complete)

### Backend Implementation
- ✅ **Room Structure**: `SelectedIDs` map per user to track selections
- ✅ **Room Methods**: `UpdateSelectedIDs()`, `GetSelectedIDs()`, `ClearSelectedIDs()`
- ✅ **WebSocket Event**: `selection_change` - Broadcasts selection changes to room

### WebSocket Events
```typescript
// Client → Server
{
  type: "selection_change",
  payload: {
    roomId: string,
    selectedIds: string[]  // Array of element IDs
  }
}

// Server → Client
{
  type: "selection_updated",
  payload: {
    userId: string,
    username: string,
    color: string,
    selectedIds: string[]
  }
}
```

### Frontend Integration
```typescript
// Send selection changes to backend
const sendSelectionChange = (selectedIds: string[]): void => {
  wsService.send('selection_change', {
    roomId: roomService.getCurrentRoom(),
    selectedIds,
  });
};

// Listen for remote selection updates
wsService.on('selection_updated', (payload) => {
  const { userId, username, color, selectedIds } = payload;
  console.log('👆 Remote selection update', { userId, selectedIds });
  
  // Visual feedback: highlight selected elements with border
  highlightSelectedElements(selectedIds, userId, color);
});
});

// Highlight selected elements with user color
const highlightSelectedElements = (selectedIds: string[], userId: string, color: string): void => {
  excalidrawAPI.current.updateScene({
    elements: excalidrawAPI.current.getSceneElements().map(el => {
      // Add colored border overlay for elements selected by others
      if (selectedIds.includes(el.id) && userId !== getCurrentUserId()) {
        return {
          ...el,
          customData: {
            ...el.customData,
            selectionColor: color,
          },
        };
      }
      return el;
    }),
  });
};

// Track local selection state
const [localSelection, setLocalSelection] = useState<string[]>([]);
```

### Conflict Visual Feedback
When multiple users select the same element:
- Backend broadcasts to all users (including sender)
- Frontend renders colored borders around element
- Each user sees their own selection color + others' selections
- No automatic conflict resolution (manual resolution required)

### Success Criteria
- [x] Users see what others have selected
- [ ] Visual feedback for multiple users editing same element
- [ ] Frontend integration completed
```

---

## Documentation References

- Backend phases: `docs/be/DEVELOPMENT_PHASES.md`
- Backend integration: `docs/fe/BACKEND_INTEGRATION.md`
- Phase summary: `docs/fe/PHASE_SUMMARY.md`
- Backend AGENTS.md: `excalidraw-be/AGENTS.md`
