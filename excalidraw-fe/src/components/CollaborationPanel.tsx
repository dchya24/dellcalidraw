import { useState, useCallback, useEffect, useRef } from "react";
import { Users, Copy, RefreshCw, Wifi, WifiOff, Loader2, ChevronDown } from "lucide-react";
import { roomService } from "../services/roomService";
import { copyShareableLink } from "../utils/roomURL";
import { useThemeStore } from "../store/useThemeStore";
import type { Participant } from "../types/websocket";

interface CollaborationPanelProps {
  roomId: string;
  username: string;
  onRegenerateRoomId: () => void;
}

export default function CollaborationPanel({
  roomId,
  username,
  onRegenerateRoomId,
}: CollaborationPanelProps) {
  const [copied, setCopied] = useState(false);
  const [showDropdown, setShowDropdown] = useState(false);
  const [isConnected, setIsConnected] = useState(false);
  const [isConnecting, setIsConnecting] = useState(false);
  const [participants, setParticipants] = useState<Participant[]>([]);
  const dropdownRef = useRef<HTMLDivElement>(null);
  const { theme } = useThemeStore();

  // Close dropdown when clicking outside
  useEffect(() => {
    const handleClickOutside = (event: MouseEvent) => {
      if (dropdownRef.current && !dropdownRef.current.contains(event.target as Node)) {
        setShowDropdown(false);
      }
    };

    if (showDropdown) {
      document.addEventListener("mousedown", handleClickOutside);
    }

    return () => {
      document.removeEventListener("mousedown", handleClickOutside);
    };
  }, [showDropdown]);

  // Monitor connection state
  useEffect(() => {
    const unsubConnect = roomService.onConnectionChange((connected) => {
      setIsConnected(connected);
      setIsConnecting(false);
    });

    // Check initial state
    setIsConnected(roomService.isConnectedToRoom());
    setParticipants(roomService.getParticipants());

    return unsubConnect;
  }, []);

  // Monitor participants
  useEffect(() => {
    const unsubJoined = roomService.onUserJoined((payload) => {
      setParticipants(roomService.getParticipants());
    });

    const unsubLeft = roomService.onUserLeft((payload) => {
      setParticipants(roomService.getParticipants());
    });

    const unsubState = roomService.onRoomState((payload) => {
      setParticipants(payload.participants || []);
    });

    return () => {
      unsubJoined();
      unsubLeft();
      unsubState();
    };
  }, []);

  const handleJoinRoom = useCallback(async () => {
    if (isConnected || isConnecting) return;

    setIsConnecting(true);
    try {
      await roomService.joinRoom(roomId, username);
      console.log('Joined room successfully');
    } catch (error) {
      console.error('Failed to join room:', error);
      setIsConnecting(false);
    }
  }, [roomId, username, isConnected, isConnecting]);

  const handleLeaveRoom = useCallback(() => {
    roomService.leaveRoom();
    setParticipants([]);
  }, []);

  const handleCopyRoomId = useCallback(async () => {
    const success = await copyShareableLink(roomId);
    if (success) {
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    }
  }, [roomId]);

  const handleRegenerate = useCallback(() => {
    onRegenerateRoomId();
  }, [onRegenerateRoomId]);

  return (
    <div className="relative" ref={dropdownRef}>
      {/* Toggle Button */}
      <button
        onClick={() => setShowDropdown(!showDropdown)}
        className={`flex items-center gap-2 px-3 py-2 transition-colors border-l border-r border-gray-200 ${
          isConnected
            ? theme === 'dark'
              ? 'bg-green-900/30 border border-green-700 hover:bg-green-900/50 text-green-400'
              : 'bg-green-50 border border-green-200 hover:bg-green-100'
            : theme === 'dark'
              ? 'hover:bg-gray-700 text-gray-300'
              : 'hover:bg-gray-50 text-gray-600'
        }`}
        title={`Open collaboration panel${isConnected ? ' (Connected)' : ''}`}
      >
        {isConnecting ? (
          <Loader2 size={16} className="animate-spin" />
        ) : isConnected ? (
          <Wifi size={16} className={theme === 'dark' ? 'text-green-400' : 'text-green-600'} />
        ) : (
          <WifiOff size={16} className={theme === 'dark' ? 'text-gray-500' : 'text-gray-400'} />
        )}
        <Users size={16} />
        <span className="text-sm">Collaborate</span>
        {participants.length > 0 && (
          <span className={`ml-auto text-xs px-2 py-0.5 rounded-full ${
            theme === 'dark' ? 'bg-gray-700' : 'bg-gray-200'
          }`}>
            {participants.length}
          </span>
        )}
        <ChevronDown
          size={14}
          className={`transition-transform ${showDropdown ? '' : 'rotate-180'}`}
        />
      </button>

      {/* Dropdown Panel */}
      {showDropdown && (
        <div className={`absolute bottom-full left-0 mt-2 z-50 w-80 rounded-lg shadow-xl border overflow-hidden ${
          theme === 'dark'
            ? 'bg-gray-800 border-gray-700'
            : 'bg-white border-gray-200'
        }`}>
          <div className="p-4">
            <div className="flex items-center justify-between mb-3">
              <h3 className={`font-semibold text-sm flex items-center gap-2 ${
                theme === 'dark' ? 'text-gray-200' : 'text-gray-900'
              }`}>
                <Users size={16} />
                Collaboration
              </h3>
            </div>

            <div className="space-y-3">
              {/* Connection Status */}
              <div className={`p-2 rounded-lg flex items-center gap-2 ${
                isConnected
                  ? theme === 'dark'
                    ? 'bg-green-900/30'
                    : 'bg-green-50'
                  : theme === 'dark'
                    ? 'bg-gray-700/50'
                    : 'bg-gray-50'
              }`}>
                {isConnecting ? (
                  <Loader2 size={16} className={`animate-spin ${theme === 'dark' ? 'text-gray-400' : 'text-gray-500'}`} />
                ) : isConnected ? (
                  <Wifi size={16} className={theme === 'dark' ? 'text-green-400' : 'text-green-600'} />
                ) : (
                  <WifiOff size={16} className={theme === 'dark' ? 'text-gray-500' : 'text-gray-400'} />
                )}
                <span className={`text-xs font-medium ${
                  theme === 'dark' ? 'text-gray-300' : 'text-gray-700'
                }`}>
                  {isConnecting ? 'Connecting...' : isConnected ? 'Connected' : 'Disconnected'}
                </span>
              </div>

              {/* Room ID */}
              <div>
                <label className={`text-xs block mb-1 ${
                  theme === 'dark' ? 'text-gray-400' : 'text-gray-500'
                }`}>Room ID</label>
                <div className="flex items-center gap-2">
                  <code className={`flex-1 px-2 py-1.5 rounded text-xs font-mono truncate ${
                    theme === 'dark' ? 'bg-gray-700 text-gray-300' : 'bg-gray-100 text-gray-800'
                  }`}>
                    {roomId}
                  </code>
                  <button
                    onClick={handleCopyRoomId}
                    className={`p-1.5 rounded transition-colors ${
                      theme === 'dark' ? 'hover:bg-gray-700 text-gray-300' : 'hover:bg-gray-100 text-gray-600'
                    }`}
                    title="Copy room link"
                  >
                    <Copy size={14} />
                  </button>
                  <button
                    onClick={handleRegenerate}
                    className={`p-1.5 rounded transition-colors ${
                      theme === 'dark' ? 'hover:bg-gray-700 text-gray-300' : 'hover:bg-gray-100 text-gray-600'
                    }`}
                    title="Generate new room ID"
                  >
                    <RefreshCw size={14} />
                  </button>
                </div>
                {copied && (
                  <p className={`text-xs mt-1 ${theme === 'dark' ? 'text-green-400' : 'text-green-600'}`}>
                    ✓ Link copied!
                  </p>
                )}
              </div>

              {/* Join/Leave Buttons */}
              {!isConnected ? (
                <button
                  onClick={handleJoinRoom}
                  disabled={isConnecting}
                  className="w-full py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors text-sm font-medium"
                >
                  {isConnecting ? 'Connecting...' : 'Join Room'}
                </button>
              ) : (
                <button
                  onClick={handleLeaveRoom}
                  className="w-full py-2 bg-red-600 text-white rounded-lg hover:bg-red-700 transition-colors text-sm font-medium"
                >
                  Leave Room
                </button>
              )}

              {/* Participants List */}
              {participants.length > 0 && (
                <div className={`pt-2 border-t ${theme === 'dark' ? 'border-gray-700' : 'border-gray-200'}`}>
                  <label className={`text-xs block mb-2 ${
                    theme === 'dark' ? 'text-gray-400' : 'text-gray-500'
                  }`}>
                    Participants ({participants.length})
                  </label>
                  <div className="space-y-1 max-h-32 overflow-y-auto">
                    {participants.map((participant) => (
                      <div
                        key={participant.id}
                        className={`flex items-center gap-2 px-2 py-1.5 rounded ${
                          theme === 'dark' ? 'bg-gray-700/50' : 'bg-gray-50'
                        }`}
                      >
                        <div
                          className="w-3 h-3 rounded-full"
                          style={{ backgroundColor: participant.color }}
                        />
                        <span className={`text-xs flex-1 truncate ${
                          theme === 'dark' ? 'text-gray-300' : 'text-gray-700'
                        }`}>
                          {participant.username}
                          {participant.id === roomService.getUsername() && ' (You)'}
                        </span>
                      </div>
                    ))}
                  </div>
                </div>
              )}

              <div className={`pt-2 border-t ${theme === 'dark' ? 'border-gray-700' : 'border-gray-200'}`}>
                <p className={`text-xs ${theme === 'dark' ? 'text-gray-400' : 'text-gray-500'}`}>
                  Share the room link with others to collaborate in real-time.
                </p>
              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
