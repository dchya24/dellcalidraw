import { useEffect, useState } from "react";
import Whiteboard from "./components/Whiteboard";
import AuthModal from "./components/AuthModal";
import ForgotPasswordModal from "./components/ForgotPasswordModal";
import ResetPasswordModal from "./components/ResetPasswordModal";
import { getRoomIdFromURL } from "./utils/roomURL";
import { roomService } from "./services/roomService";
import { useAuthStore } from "./store/useAuthStore";
import { apiService } from "./services/api";
import { tokenRefreshService } from "./services/tokenRefreshService";

function App() {
  const { user, isAuthenticated, refreshToken, clearAuth } = useAuthStore();
  const [authModalOpen, setAuthModalOpen] = useState(false);
  const [forgotPasswordOpen, setForgotPasswordOpen] = useState(false);

  // 1. Helper to get the token (optional, keeps code clean)
  const getInitialToken = () => {
    if (typeof window === "undefined") return null;
    const urlParams = new URLSearchParams(window.location.search);
    return urlParams.get("reset-token") || urlParams.get("token");
  };


  const [resetToken, setResetToken] = useState(() => getInitialToken());
  const [resetPasswordOpen, setResetPasswordOpen] = useState(() => {
    const token = getInitialToken();
    return !!(token && window.location.pathname.includes("reset-password"));
  });

  const [username] = useState(() => {
    // 1. Check the 'user' object from props/context first
    if (user?.username) return user.username;

    // 2. Check localStorage
    const saved = localStorage.getItem("username");
    if (saved) return saved;

    // 3. Generate new (Only happens once on mount)
    const newUsername = `User_${Math.random().toString(36).substring(2, 8)}`;
    localStorage.setItem("username", newUsername);
    return newUsername;
  });

  // Start/stop token refresh service based on auth state
  useEffect(() => {
    if (isAuthenticated) {
      tokenRefreshService.start();
    } else {
      tokenRefreshService.stop();
    }

    return () => {
      tokenRefreshService.stop();
    };
  }, [isAuthenticated]);

  useEffect(() => {
    if (resetToken) {
      window.history.replaceState({}, document.title, window.location.pathname);
    }
  }, [resetToken]);

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

  const handleLogout = async () => {
    const currentRefreshToken = refreshToken;
    clearAuth();
    tokenRefreshService.stop();
    localStorage.removeItem("username");
    if (currentRefreshToken) {
      try {
        await apiService.logout(currentRefreshToken);
      } catch (err) {
        console.error("Logout API error:", err);
      }
    }
  };

  const handleForgotPassword = () => {
    setAuthModalOpen(false);
    setForgotPasswordOpen(true);
  };

  const handleBackToLogin = () => {
    setForgotPasswordOpen(false);
    setResetPasswordOpen(false);
    setAuthModalOpen(true);
  };

  const handleResetSuccess = () => {
    setResetPasswordOpen(false);
    setResetToken("");
    setAuthModalOpen(true);
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
        onForgotPassword={handleForgotPassword}
      />
      <ForgotPasswordModal
        isOpen={forgotPasswordOpen}
        onClose={() => setForgotPasswordOpen(false)}
        onBackToLogin={handleBackToLogin}
      />
      <ResetPasswordModal
        isOpen={resetPasswordOpen}
        token={resetToken}
        onClose={() => {
          setResetPasswordOpen(false);
          setResetToken("");
        }}
        onSuccess={handleResetSuccess}
      />
    </>
  );
}

export default App;
