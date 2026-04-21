import { useEffect, useState } from "react";
import Whiteboard from "./components/Whiteboard";
import AuthModal from "./components/AuthModal";
import { getRoomIdFromURL } from "./utils/roomURL";
import { roomService } from "./services/roomService";
import { useAuthStore } from "./store/useAuthStore";

function App() {
  const { user, isAuthenticated, clearAuth } = useAuthStore();
  const [authModalOpen, setAuthModalOpen] = useState(false);

  const username = user?.username || (() => {
    const saved = localStorage.getItem("username");
    if (saved) return saved;
    const newUsername = `User_${Math.random().toString(36).substring(2, 8)}`;
    localStorage.setItem("username", newUsername);
    return newUsername;
  })();

  useEffect(() => {
    const roomId = getRoomIdFromURL();
    if (roomId && username) {
      const timer = setTimeout(() => {
        roomService
          .joinRoom(roomId, username)
          .then(() => console.log("Auto-joined room:", roomId))
          .catch((err) => console.error("Failed to auto-join room:", err));
      }, 100);

      return () => clearTimeout(timer);
    }
  }, [username]);

  const handleLogout = () => {
    clearAuth();
    localStorage.removeItem("username");
  };

  return (
    <>
      <Whiteboard
        username={username}
        isAuthenticated={isAuthenticated}
        onOpenAuth={() => setAuthModalOpen(true)}
        onLogout={handleLogout}
      />
      <AuthModal
        isOpen={authModalOpen}
        onClose={() => setAuthModalOpen(false)}
      />
    </>
  );
}

export default App;
