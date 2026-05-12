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
  const [resetPasswordOpen, setResetPasswordOpen] = useState(false);
  const [resetToken, setResetToken] = useState<string>("");

  const username = user?.username || (() => {
    const saved = localStorage.getItem("username");
    if (saved) return saved;
    const newUsername = `User_${Math.random().toString(36).substring(2, 8)}`;
    localStorage.setItem("username", newUsername);
    return newUsername;
  })();

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

  // Check for password reset token in URL
  useEffect(() => {
    const urlParams = new URLSearchParams(window.location.search);
    const token = urlParams.get("reset-token") || urlParams.get("token");
    
    // Check if this is a password reset URL
    if (token && window.location.pathname.includes("reset-password")) {
      setResetToken(token);
      setResetPasswordOpen(true);
      // Clean up URL
      window.history.replaceState({}, document.title, window.location.pathname);
    }
  }, []);

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
