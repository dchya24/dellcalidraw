# Quick Start - Real-Time Collaboration Testing

## 1. Start Services

### Backend (Terminal 1)
```bash
cd excalidraw-be
make run
```

Expected output:
```
{"level":"info","msg":"Starting server","port":"8080"}
```

### Frontend (Terminal 2)
```bash
cd excalidraw-fe
npm run dev
```

Expected output:
```
  VITE v7.3.1  ready in 300 ms

  ➜  Local:   http://localhost:5173/
```

## 2. Test Real-Time Collaboration

### Step 1: Open Two Browser Windows
1. Open browser to `http://localhost:5173`
2. Open **Incognito/Private window** to `http://localhost:5173`
   - This ensures separate sessions with different usernames

### Step 2: Join the Same Room
1. In both windows, click the **"Collaborate"** button in the toolbar (users icon)
2. Click **"Join Room"** in both windows
3. Verify both show **"Connected"** status with green WiFi icon
4. Verify participant count shows **2**
5. Verify both users appear in participant list with different colors

### Step 3: Test Element Sync
1. **Window 1**: Draw a rectangle
2. **Window 2**: Verify rectangle appears instantly
3. **Window 2**: Draw an ellipse
4. **Window 1**: Verify ellipse appears instantly
5. **Window 1**: Move the rectangle
6. **Window 2**: Verify rectangle moves in real-time
7. **Window 2**: Delete the ellipse
8. **Window 1**: Verify ellipse disappears instantly

### Step 4: Test Cursor Tracking
1. **Window 1**: Move mouse around the canvas
2. **Window 2**: Verify cursor appears with:
   - Correct position
   - User's color
   - Username label
3. **Window 2**: Move mouse around
4. **Window 1**: Verify cursor appears

### Step 5: Test Selection Awareness
1. **Window 1**: Click on rectangle to select it
2. **Window 2**: Verify colored border appears around rectangle
3. **Window 2**: Verify username label shows
4. **Window 2**: Select ellipse
5. **Window 1**: Verify selection border appears with username

### Step 6: Test Room Link Sharing
1. **Window 1**: Click the **copy icon** (next to Room ID)
2. Verify **"Link copied!"** message appears
3. Open a **third browser window** (or Incognito)
4. Paste the link and press Enter
5. Verify it opens with `?room={roomId}` in URL
6. Verify auto-joins to room automatically
7. Verify participant count shows **3**

### Step 7: Test Leave Room
1. **Window 3**: Click **"Leave Room"**
2. **Window 1 & 2**: Verify user disappears from participant list
3. **Window 1 & 2**: Verify cursor disappears
4. **Window 1 & 2**: Verify selection borders disappear

## Common Issues & Solutions

### "WebSocket not connected" error
**Solution:**
- Verify backend is running: `curl http://localhost:8080/health`
- Refresh the browser page
- Check browser console for errors

### Elements not syncing
**Solution:**
- Verify both windows are in the same room (check Room IDs match)
- Verify both show "Connected" status
- Make sure you've joined the room in both windows

### Cursors not appearing
**Solution:**
- Verify both users are connected to room
- Move mouse to trigger cursor update
- Check browser console for errors

## What to Look For

✅ **Success Indicators:**
- Elements appear instantly when created by other user
- Cursors follow mouse movements smoothly
- Selection borders show when other user selects elements
- Participant list updates when users join/leave
- "Connected" status shows green WiFi icon

⚠️ **Warning Indicators:**
- Elements take more than 1 second to sync
- Cursor movement is delayed/laggy
- Selection borders don't appear
- Participant list doesn't update

❌ **Error Indicators:**
- "WebSocket not connected" error
- "Failed to join room" error
- Elements not appearing at all
- Application crashes or freezes

## Quick Reference

| Feature | How to Test | Expected Result |
|---------|--------------|------------------|
| Element sync | Draw in Window 1, check Window 2 | Element appears instantly |
| Cursor tracking | Move mouse in Window 1, check Window 2 | Cursor follows in real-time |
| Selection awareness | Select in Window 1, check Window 2 | Colored border appears |
| Room sharing | Copy link, open in new window | Auto-joins to room |
| Leave room | Click "Leave Room" | User disappears from others' view |

## Next Steps

After completing basic tests:
1. Try with **3+ users** simultaneously
2. Test with **large drawings** (100+ elements)
3. Test **rapid updates** (draw continuously)
4. Test **network interruption** (disconnect internet, reconnect)
5. Review full testing guide: `docs/fe/INTEGRATION_TESTING.md`

## Need Help?

- **Full documentation**: See `docs/fe/INTEGRATION_COMPLETE.md`
- **Detailed testing**: See `docs/fe/INTEGRATION_TESTING.md`
- **Architecture**: See `docs/fe/PHASE_SUMMARY.md`

---

**Happy Testing! 🎉**
