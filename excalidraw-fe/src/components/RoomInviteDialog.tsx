import { useState, useEffect } from "react";
import { getRoomIdFromURL, clearRoomIdFromURL } from "../utils/roomURL";
import { roomService } from "../services/roomService";
import { useThemeStore } from "../store/useThemeStore";

interface RoomInviteDialogProps {
  username: string;
  onJoined?: () => void;
}

export default function RoomInviteDialog({ username, onJoined }: RoomInviteDialogProps) {
  const { theme } = useThemeStore();
  const [isOpen, setIsOpen] = useState(false);
  const [roomId, setRoomId] = useState<string | null>(null);
  const [isJoining, setIsJoining] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Check for room in URL on mount
  useEffect(() => {
    const roomFromURL = getRoomIdFromURL();
    if (roomFromURL) {
      setRoomId(roomFromURL);
      setIsOpen(true);
    }
  }, []);

  const handleJoin = async () => {
    if (!roomId) return;

    setIsJoining(true);
    setError(null);

    try {
      await roomService.joinRoom(roomId, username);
      clearRoomIdFromURL();
      setIsOpen(false);
      onJoined?.();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to join room");
    } finally {
      setIsJoining(false);
    }
  };

  const handleCancel = () => {
    clearRoomIdFromURL();
    setIsOpen(false);
  };

  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 z-[100] flex items-center justify-center bg-black/50">
      <div
        className={`w-full max-w-md mx-4 p-6 rounded-lg shadow-xl ${
          theme === "dark" ? "bg-gray-800 text-white" : "bg-white text-gray-900"
        }`}
      >
        <div className="flex items-center gap-3 mb-4">
          <div className="w-10 h-10 rounded-full bg-blue-500 flex items-center justify-center">
            <svg
              className="w-6 h-6 text-white"
              fill="none"
              stroke="currentColor"
              viewBox="0 0 24 24"
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M18 9v3m0 0v3m0-3h3m-3 0h-3m-2-5a4 4 0 11-8 0 4 4 0 018 0zM3 20a6 6 0 0112 0v1H3v-1z"
              />
            </svg>
          </div>
          <div>
            <h2 className="text-xl font-semibold">Join Collaboration Room</h2>
            <p className={`text-sm ${theme === "dark" ? "text-gray-400" : "text-gray-600"}`}>
              You've been invited to join a whiteboard session
            </p>
          </div>
        </div>

        <div
          className={`p-4 rounded-lg mb-4 ${
            theme === "dark" ? "bg-gray-700" : "bg-gray-100"
          }`}
        >
          <p className={`text-sm mb-1 ${theme === "dark" ? "text-gray-400" : "text-gray-600"}`}>
            Room ID
          </p>
          <p className="font-mono text-lg font-medium">{roomId}</p>
        </div>

        <p className={`mb-6 ${theme === "dark" ? "text-gray-300" : "text-gray-700"}`}>
          Joining will connect you to other participants in this room. Your username will be:
          <span className="font-semibold ml-1">{username}</span>
        </p>

        {error && (
          <div className="mb-4 p-3 rounded-lg bg-red-500/10 border border-red-500/50 text-red-500">
            {error}
          </div>
        )}

        <div className="flex gap-3 justify-end">
          <button
            onClick={handleCancel}
            disabled={isJoining}
            className={`px-4 py-2 rounded-lg font-medium transition-colors ${
              theme === "dark"
                ? "bg-gray-700 hover:bg-gray-600 text-white"
                : "bg-gray-200 hover:bg-gray-300 text-gray-900"
            } disabled:opacity-50`}
          >
            Cancel
          </button>
          <button
            onClick={handleJoin}
            disabled={isJoining}
            className={`px-4 py-2 rounded-lg font-medium transition-colors ${
              theme === "dark"
                ? "bg-blue-600 hover:bg-blue-500 text-white"
                : "bg-blue-500 hover:bg-blue-600 text-white"
            } disabled:opacity-50 flex items-center gap-2`}
          >
            {isJoining ? (
              <>
                <svg className="animate-spin h-4 w-4" viewBox="0 0 24 24">
                  <circle
                    className="opacity-25"
                    cx="12"
                    cy="12"
                    r="10"
                    stroke="currentColor"
                    strokeWidth="4"
                    fill="none"
                  />
                  <path
                    className="opacity-75"
                    fill="currentColor"
                    d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
                  />
                </svg>
                Joining...
              </>
            ) : (
              "Join Room"
            )}
          </button>
        </div>
      </div>
    </div>
  );
}
