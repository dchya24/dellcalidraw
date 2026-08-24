import { useEffect, useRef, useState } from "react";
import Whiteboard from "./components/Whiteboard";
import AuthModal from "./components/AuthModal";
import ForgotPasswordModal from "./components/ForgotPasswordModal";
import ResetPasswordModal from "./components/ResetPasswordModal";
import MigrationDialog from "./components/MigrationDialog";
import { getRoomIdFromURL, clearRoomIdFromURL } from "./utils/roomURL";
import { roomService } from "./services/roomService";
import { useAuthStore } from "./store/useAuthStore";
import { useWhiteboardStore } from "./store/useWhiteboardStore";
import { useAIChatStore } from "./store/useAIChatStore";
import { apiService } from "./services/api";
import { tokenRefreshService } from "./services/tokenRefreshService";

function App() {
  const { user, isAuthenticated, refreshToken, clearAuth } = useAuthStore();
  const pendingMigration = useWhiteboardStore((s) => s.pendingMigration);
  const confirmMigration = useWhiteboardStore((s) => s.confirmMigration);
  const discardMigration = useWhiteboardStore((s) => s.discardMigration);
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

  const [username, setUsername] = useState(() => {
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

  // Keep `username` in sync with auth state.
  //   - logged in           -> mirror `user.username`
  //   - logged-in -> guest  -> wipe stored guest name and generate a fresh one
  //   - guest (steady state) -> reuse the saved guest name
  //
  // We track the previously-synced auth state via `useState` (not
  // `useRef`) and adjust `username` during render — this is the
  // pattern recommended by React when state needs to react to prop
  // changes. See:
  // https://react.dev/learn/you-might-not-need-an-effect#adjusting-some-state-when-a-prop-changes
  //
  // The `Math.random()` call for the fresh guest name is unavoidable
  // (the value must differ across logouts), so the purity rule is
  // disabled for that one expression.
  const [lastSyncedUserId, setLastSyncedUserId] = useState<string | undefined>(
    user?.id,
  );
  const [lastSyncedIsAuth, setLastSyncedIsAuth] = useState(isAuthenticated);
  if (lastSyncedUserId !== user?.id || lastSyncedIsAuth !== isAuthenticated) {
    const wasAuth = lastSyncedIsAuth;
    setLastSyncedUserId(user?.id);
    setLastSyncedIsAuth(isAuthenticated);

    if (user?.username) {
      if (username !== user.username) setUsername(user.username);
    } else if (wasAuth && !isAuthenticated) {
      // Auto-logout (or manual logout) just happened. Drop the previous
      // identity and start a clean guest session.
      // eslint-disable-next-line react-hooks/purity
      const fresh = `User_${Math.random().toString(36).substring(2, 8)}`;
      setUsername(fresh);
    }
    // Steady-state guest: keep current username
  }

  // Persist username to localStorage whenever it changes.
  useEffect(() => {
    localStorage.setItem("username", username);
  }, [username]);

  // Start/stop token refresh service based on auth state
  // Proactively refresh token on page load if expired
  useEffect(() => {
    if (isAuthenticated) {
      tokenRefreshService.start().catch((err) => {
        console.error("[App] Failed to start token refresh service:", err);
      });
    } else {
      tokenRefreshService.stop();
    }

    return () => {
      tokenRefreshService.stop();
    };
  }, [isAuthenticated]);

  // Centralized cleanup that runs on every authenticated -> guest
  // transition, whether triggered by manual signout or by an auto-logout
  // (refresh token invalid / expired). Keeping this in one place ensures
  // both paths leave behind the same clean slate.
  const wasAuthRef = useRef(isAuthenticated);
  useEffect(() => {
    const wasAuthenticated = wasAuthRef.current;
    wasAuthRef.current = isAuthenticated;

    if (wasAuthenticated && !isAuthenticated) {
      // 1. Disconnect any live collaboration room so the server stops
      //    treating us as the authenticated user.
      roomService.leaveRoom();

      // 2. Drop ?room= from the URL so the auto-join effect doesn't
      //    immediately re-attach the new guest to the same room.
      if (getRoomIdFromURL()) {
        clearRoomIdFromURL();
      }

      // 3. Clear AI conversations: they are keyed by tab IDs that get
      //    regenerated whenever cloud files are reloaded.
      useAIChatStore.getState().clearAllConversations();
    }
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

    // Side-effect cleanup (room/URL/AI/whiteboard/username) is handled
    // reactively by the effects + store subscribers that watch the
    // isAuthenticated transition. Keeping this function focused on the
    // auth-state change itself means manual signout and auto-logout
    // (token expired) follow the exact same cleanup path.
    clearAuth();
    tokenRefreshService.stop();

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
      <MigrationDialog
        isOpen={!!pendingMigration && pendingMigration.length > 0}
        pendingFiles={pendingMigration ?? []}
        onMerge={confirmMigration}
        onDiscard={discardMigration}
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
