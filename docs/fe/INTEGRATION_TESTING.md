# Frontend Integration Testing Guide

## Prerequisites

1. **Backend running**: Ensure backend service is running on port 8080
   ```bash
   cd excalidraw-be && make run
   ```
   Verify with: `curl http://localhost:8080/health`

2. **Frontend running**: Ensure frontend dev server is running
   ```bash
   cd excalidraw-fe && npm run dev
   ```
   Open browser to: `http://localhost:5173`

## Test Scenarios

### 1. Single User - Basic Functionality ✅

**Purpose:** Verify all core features work without collaboration

- [x] **Canvas Drawing**
  - Draw rectangles
  - Draw ellipses
  - Draw arrows/lines
  - Add text
  - Use freehand drawing

- [x] **Multi-Tab Management**
  - Create new tabs
  - Switch between tabs
  - Rename tabs (double-click)
  - Delete tabs with confirmation

- [x] **Persistence**
  - Create drawings
  - Refresh page
  - Verify drawings are preserved

- [x] **Import/Export**
  - Export to PNG
  - Export to SVG
  - Export to JSON (.excalidraw)
  - Import from JSON file
  - Drag-and-drop import

- [x] **Theme Toggle**
  - Switch to dark mode
  - Switch to light mode
  - Verify canvas and UI sync

- [x] **Keyboard Shortcuts**
  - Ctrl+S (manual save)
  - Ctrl+T (new tab)
  - Ctrl+W (close tab)
  - Ctrl+D (toggle dark mode)
  - Ctrl+Tab (next tab)
  - Ctrl+1..9 (switch to specific tab)
  - Delete/Backspace (delete tab)
  - Escape (clear selection)

### 2. Two Users - Basic Collaboration ✅

**Purpose:** Verify real-time collaboration between two users

**Setup:**
- Open two browser windows (Incognito for clean session)
- Join same room in both windows
- Use different usernames

**Tests:**

- [x] **Join Room**
  - Window 1: Click "Collaborate" → "Join Room"
  - Window 2: Click "Collaborate" → "Join Room"
  - Verify both show "Connected" status
  - Verify participant count shows 2
  - Verify both users appear in participant list with different colors

- [x] **Element Synchronization**
  - Window 1: Draw a rectangle
  - Window 2: Verify rectangle appears immediately
  - Window 2: Draw an ellipse
  - Window 1: Verify ellipse appears immediately
  - Window 1: Modify rectangle (move, resize, change color)
  - Window 2: Verify changes appear immediately
  - Window 2: Delete ellipse
  - Window 1: Verify ellipse disappears immediately

- [x] **Cursor Tracking**
  - Window 1: Move mouse around canvas
  - Window 2: Verify cursor appears with correct position
  - Window 2: Verify cursor has correct color and username
  - Window 2: Move mouse around canvas
  - Window 1: Verify cursor appears
  - Stop moving mouse in Window 1
  - Window 2: Verify cursor disappears after 5 seconds

- [x] **Selection Awareness**
  - Window 1: Click on rectangle to select it
  - Window 2: Verify colored border appears around rectangle
  - Window 2: Verify username label shows next to selection
  - Window 2: Select ellipse
  - Window 1: Verify selection border appears
  - Window 1: Deselect rectangle
  - Window 2: Verify selection border disappears

- [x] **Conflict Warnings**
  - Window 1: Make changes to drawing
  - Window 2: Verify yellow notification appears
  - Verify notification shows correct username
  - Verify notification shows change count

- [x] **Room Link Sharing**
  - Window 1: Click copy link button
  - Verify "Link copied!" confirmation
  - Paste link in new browser window
  - Verify it opens with `?room={roomId}` in URL
  - Verify auto-joins to room (shows connected status)

- [x] **Leave Room**
  - Window 2: Click "Leave Room"
  - Window 1: Verify user disappears from participant list
  - Window 1: Verify "User left" notification
  - Window 1: Verify cursors disappear
  - Window 1: Verify selection borders disappear

### 3. Three Users - Advanced Collaboration ⏳

**Purpose:** Test with multiple simultaneous collaborators

**Setup:**
- Open three browser windows
- Join same room in all three windows
- Use different usernames (Alice, Bob, Charlie)

**Tests:**

- [ ] **Participant List**
  - Verify all three users appear in list
  - Verify each has different color
  - Verify "(You)" appears on your own entry

- [ ] **Concurrent Drawing**
  - Alice draws rectangle
  - Bob draws ellipse at same time
  - Charlie draws arrow at same time
  - Verify all elements appear in all windows
  - Verify no elements are lost

- [ ] **Multiple Cursors**
  - All three users move cursors
  - Verify all three cursors visible in each window
  - Verify each has correct color and username

- [ ] **Multiple Selections**
  - Alice selects rectangle
  - Bob selects ellipse
  - Charlie selects arrow
  - Verify all three selections visible with different colors
  - Verify all three username labels visible

- [ ] **User Leaves**
  - Charlie leaves room
  - Alice and Bob verify Charlie disappears
  - Verify Charlie's cursor disappears
  - Verify Charlie's selection borders disappear

### 4. Edge Cases & Error Handling ⏳

**Purpose:** Test system behavior in unusual situations

**Tests:**

- [ ] **Rapid Element Updates**
  - Draw and modify elements rapidly
  - Verify no rate limit errors (within 20/sec limit)
  - Verify all changes sync correctly

- [ ] **Large Drawing**
  - Create 100+ elements
  - Verify sync performance remains acceptable
  - Verify no elements are lost

- [ ] **Network Interruption**
  - Disconnect from internet
  - Verify error notification appears
  - Reconnect to internet
  - Verify auto-reconnection works
  - Verify reconnection doesn't duplicate elements

- [ ] **Element Validation**
  - Try to create invalid element (if possible via API)
  - Verify backend rejects invalid elements
  - Verify error message appears

- [ ] **Room Full (50 users)**
  - Simulate 50 users joining
  - Verify 51st user gets "room_full" error
  - Verify error notification is user-friendly

- [ ] **Tab Switching During Collaboration**
  - User in Room A with collaborators
  - Switch to Tab B (different room)
  - Verify disconnects from Room A
  - Verify cursors/selections disappear
  - Switch back to Tab A
  - Verify auto-reconnects to Room A

### 5. Multi-File Collaboration ⏳

**Purpose:** Test collaboration with multiple files/tabs

**Setup:**
- Create 3 files with multiple tabs each
- Join different rooms in different tabs
- Have multiple users collaborate across files

**Tests:**

- [ ] **Isolated Rooms**
  - Tab 1 in File 1: Room A
  - Tab 2 in File 2: Room B
  - Verify elements from Room A don't appear in Room B
  - Verify cursors from Room A don't appear in Room B

- [ ] **Simultaneous Collaboration**
  - User 1: Collaborating in File 1, Tab 1
  - User 2: Collaborating in File 2, Tab 1
  - Verify no cross-pollination of data
  - Verify participant lists are separate

### 6. Performance Tests ⏳

**Purpose:** Verify system performance under load

**Tests:**

- [ ] **Large Canvas Sync**
  - Create 500 elements
  - Verify sync completes in < 2 seconds
  - Verify browser remains responsive

- [ ] **Rapid Cursor Updates**
  - Move mouse rapidly across canvas
  - Verify smooth cursor tracking in other windows
  - Verify no lag or stuttering

- [ ] **Multiple Selections**
  - Select 50 elements at once
  - Verify selection borders render smoothly
  - Verify no performance degradation

## Success Criteria

### Minimum Viable Product (MVP) ✅
- [x] Two users can collaborate in real-time
- [x] Elements sync between users
- [x] Cursors are visible to other users
- [x] Selections are visible to other users
- [x] Room link sharing works
- [x] Auto-join from shared link works
- [x] Participant list updates correctly
- [x] Connection status is accurate

### Production Ready ⏳
- [ ] All tests pass with 3+ users
- [ ] Performance acceptable under load
- [ ] Error handling is graceful
- [ ] No data loss in any scenario
- [ ] Reconnection works reliably

## Common Issues & Troubleshooting

### Issue: "WebSocket not connected" in console

**Solution:**
1. Check if backend is running: `curl http://localhost:8080/health`
2. Check browser console for WebSocket errors
3. Verify no firewall blocking port 8080
4. Try refreshing the page

### Issue: Elements not syncing between windows

**Solution:**
1. Verify both windows are in same room
2. Check both windows show "Connected" status
3. Look for error messages in console
4. Verify both users have joined the room

### Issue: Cursors not appearing

**Solution:**
1. Verify you're connected to room
2. Move mouse to trigger cursor update
3. Check browser console for errors
4. Ensure RemoteCursors component is rendered

### Issue: Selection borders not showing

**Solution:**
1. Verify you've selected an element
2. Check SelectionOverlay is rendered
3. Verify other user is still connected
4. Check console for errors

### Issue: "Room full" error

**Solution:**
1. Current limit is 50 users per room
2. Wait for users to leave
3. Or create a new room with different ID

### Issue: Elements lost after refresh

**Solution:**
1. Verify you were connected to room
2. Check browser console for errors
3. Verify backend is still running
4. Rejoin the room manually

## Performance Benchmarks

### Acceptable Performance Targets
- **Element sync latency**: < 500ms
- **Cursor position latency**: < 200ms
- **Selection update latency**: < 300ms
- **Large canvas (100 elements)**: Sync in < 2 seconds
- **Join room**: < 1 second (after handshake)

### Measured Performance
- [ ] Element sync: ____ ms
- [ ] Cursor latency: ____ ms
- [ ] Selection latency: ____ ms
- [ ] Large canvas: ____ seconds
- [ ] Room join: ____ seconds

## Next Steps After Testing

1. **Document Issues**: Record any bugs or issues found
2. **Prioritize Fixes**: Rank issues by severity
3. **Plan Next Features**: Decide on Phase 8 features
4. **Performance Optimization**: Address any performance bottlenecks
5. **User Testing**: Get feedback from real users
