import { useState, useCallback, useEffect, useRef } from "react";
import { Users, Copy, RefreshCw, Wifi, WifiOff, Loader2, ChevronDown, AlertCircle } from "lucide-react";
import { roomService } from "../services/roomService";
import { copyShareableLink } from "../utils/roomURL";
import { useThemeStore } from "../store/useThemeStore";

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
  const [connectionState, setConnectionState] = useState<'disconnected' | 'connecting' | 'connected' | 'reconnecting'>(() => 
    roomService.getConnectionState()
  );
  const [reconnectAttempts, setReconnectAttempts] = useState(0);
  const [connectionError, setConnectionError] = useState<string | null>(null);
  const [participants, setParticipants] = useState(() => roomService.getParticipants());
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

  // Monitor connection state with enhanced info
  useEffect(() => {
    const unsubConnect = roomService.onConnectionChange((connected) => {
      setConnectionState(connected ? 'connected' : roomService.getConnectionState());
      setReconnectAttempts(roomService.getReconnectAttempts());
    });

    // Subscribe to errors
    const unsubError = roomService.onError((error) => {
      setConnectionError(error.message);
      // Clear error after 5 seconds
      setTimeout(() => setConnectionError(null), 5000);
    });

    // Periodically update connection state and reconnect attempts
    const interval = setInterval(() => {
      setConnectionState(roomService.getConnectionState());
      setReconnectAttempts(roomService.getReconnectAttempts());
    }, 1000);

    return () => {
      unsubConnect();
      unsubError();
      clearInterval(interval);
    };
  }, []);

  // Monitor participants
  useEffect(() => {
    const unsubJoined = roomService.onUserJoined(() => {
      setParticipants(roomService.getParticipants());
    });

    const unsubLeft = roomService.onUserLeft(() => {
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
    if (connectionState === 'connected' || connectionState === 'connecting') return;

    setConnectionState('connecting');
    try {
      await roomService.joinRoom(roomId, username);
      setConnectionError(null);
      console.log('Joined room successfully');
    } catch (error) {
      console.error('Failed to join room:', error);
      setConnectionError(error instanceof Error ? error.message : 'Failed to connect');
    }
  }, [roomId, username, connectionState]);

  const handleReconnect = useCallback(async () => {
    try {
      setConnectionState('connecting');
      await roomService.reconnect();
      setConnectionError(null);
    } catch (error) {
      console.error('Reconnection failed:', error);
      setConnectionError('Reconnection failed');
    }
  }, []);

  const handleLeaveRoom = useCallback(() => {
    roomService.leaveRoom();
    setParticipants([]);
    setConnectionError(null);
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

  // Helper to get connection status UI
  const getConnectionStatusUI = () => {
    switch (connectionState) {
      case 'connected':
        return {
          icon: <Wifi size={16} className={theme === 'dark' ? 'text-green-400' : 'text-green-600'} />,
          text: 'Connected',
          className: theme === 'dark' ? 'bg-green-900/30 text-green-400' : 'bg-green-50 text-green-600'
        };
      case 'connecting':
        return {
          icon: <Loader2 size={16} className="animate-spin" />,
          text: 'Connecting...',
          className: theme === 'dark' ? 'bg-blue-900/30 text-blue-400' : 'bg-blue-50 text-blue-600'
        };
      case 'reconnecting':
        return {
          icon: <Loader2 size={16} className={`animate-spin ${theme === 'dark' ? 'text-yellow-400' : 'text-yellow-600'}`} />,
          text: `Reconnecting (attempt ${reconnectAttempts})`,
          className: theme === 'dark' ? 'bg-yellow-900/30 text-yellow-400' : 'bg-yellow-50 text-yellow-600'
        };
      default:
        return {
          icon: <WifiOff size={16} className={theme === 'dark' ? 'text-gray-500' : 'text-gray-400'} />,
          text: 'Disconnected',
          className: theme === 'dark' ? 'bg-gray-700/50 text-gray-400' : 'bg-gray-50 text-gray-500'
        };
    }
  };

  const statusUI = getConnectionStatusUI();
  const isConnected = connectionState === 'connected';

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
            : connectionState === 'reconnecting'
              ? theme === 'dark'
                ? 'bg-yellow-900/30 border border-yellow-700 hover:bg-yellow-900/50 text-yellow-400'
                : 'bg-yellow-50 border border-yellow-200 hover:bg-yellow-100'
              : theme === 'dark'
                ? 'hover:bg-gray-700 text-gray-300'
                : 'hover:bg-gray-50 text-gray-600'
        }`}
        title={`Open collaboration panel${isConnected ? ' (Connected)' : connectionState === 'reconnecting' ? ' (Reconnecting...)' : ''}`}
      >
        {statusUI.icon}
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
              <div className={`p-2 rounded-lg flex items-center gap-2 ${statusUI.className}`}>
                {statusUI.icon}
                <span className="text-xs font-medium">
                  {statusUI.text}
                </span>
              </div>

              {/* Error Message */}
              {connectionError && (
                <div className={`p-2 rounded-lg flex items-center gap-2 ${
                  theme === 'dark' ? 'bg-red-900/30 text-red-400' : 'bg-red-50 text-red-600'
                }`}>
                  <AlertCircle size={16} />
                  <span className="text-xs">{connectionError}</span>
                </div>
              )}

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

              {/* Join/Leave/Reconnect Buttons */}
              {!isConnected ? (
                connectionState === 'reconnecting' ? (
                  <button
                    onClick={handleReconnect}
                    className="w-full py-2 bg-yellow-600 text-white rounded-lg hover:bg-yellow-700 transition-colors text-sm font-medium"
                  >
                    Force Reconnect
                  </button>
                ) : (
                  <button
                    onClick={handleJoinRoom}
                    disabled={connectionState === 'connecting'}
                    className="w-full py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors text-sm font-medium"
                  >
                    {connectionState === 'connecting' ? 'Connecting...' : 'Join Room'}
                  </button>
                )
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
