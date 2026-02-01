import { useEffect, useState } from "react";
import Whiteboard from "./components/Whiteboard";
import { getRoomIdFromURL } from "./utils/roomURL";
import { roomService } from "./services/roomService";

function App() {
  const [username] = useState(() => {
    // Get or generate username
    const saved = localStorage.getItem('username');
    if (saved) return saved;

    const newUsername = `User_${Math.random().toString(36).substring(2, 8)}`;
    localStorage.setItem('username', newUsername);
    return newUsername;
  });

  useEffect(() => {
    document.title = "Dell-Draw";
  }, []);

  // Auto-join room if URL contains room parameter
  useEffect(() => {
    const roomId = getRoomIdFromURL();
    console.log('Room ID:', roomId, username);
    if (roomId && username) {
      // Add small delay to ensure everything is ready
      const timer = setTimeout(() => {
        roomService.joinRoom(roomId, username)
          .then(() => console.log('✅ Auto-joined room:', roomId))
          .catch(err => console.error('❌ Failed to auto-join room:', err));
      }, 100);

      return () => clearTimeout(timer);
    }
  }, [username]);

  return <Whiteboard username={username} />;
}

export default App;
