import { wsService } from './websocket';
import type { JoinRoomPayload, RoomStatePayload, UserJoinedPayload, UserLeftPayload, Participant, ErrorPayload } from '../types/websocket';

interface RoomServiceConfig {
  wsUrl: string;
  username?: string;
  enableHeartbeat?: boolean;
}

class RoomService {
  private currentRoomId: string | null = null;
  private currentUsername: string = '';
  private isConnected: boolean = false;
  private participants: Participant[] = [];
  private roomStateListeners: Set<(state: RoomStatePayload) => void> = new Set();
  private userJoinedListeners: Set<(payload: UserJoinedPayload) => void> = new Set();
  private userLeftListeners: Set<(payload: UserLeftPayload) => void> = new Set();
  private connectionChangeListeners: Set<(connected: boolean) => void> = new Set();
  private errorListeners: Set<(error: Error) => void> = new Set();
  private unloadHandler: ((this: Window, ev: Event) => unknown) | undefined;
  private wsErrorUnsubscribe: (() => void) | null = null;

  async joinRoom(roomId: string, username: string, config?: RoomServiceConfig): Promise<void> {
    if (this.currentRoomId === roomId && this.isConnected) {
      console.log('[RoomService] Already joined room:', roomId);
      return;
    }

    // Leave current room if any
    if (this.currentRoomId) {
      this.leaveRoom();
    }

    const wsUrl = config?.wsUrl || this.getDefaultWsUrl();
    console.log('[RoomService] Connecting to WebSocket:', wsUrl);
    this.currentUsername = username;

    try {
      // Set up message handlers BEFORE connecting
      this.setupMessageHandlers();

      // Connect to WebSocket with enhanced features
      await wsService.connect({
        url: wsUrl,
        reconnectInterval: 1000, // Start with 1s, will increase exponentially
        maxReconnectAttempts: 10,
        enableHeartbeat: config?.enableHeartbeat ?? true,
        heartbeatInterval: 10000, // 10 seconds
        heartbeatTimeout: 10000,  // 10 seconds
      });

      // Send join_room message with acknowledgment support
      const payload: JoinRoomPayload = { roomId, username };
      console.log('[RoomService] Sending join_room:', payload);

      // Use regular send (join_room doesn't need ack)
      const sent = wsService.send('join_room', payload);

      if (!sent) {
        throw new Error('Failed to send join_room message - WebSocket not connected');
      }

      this.currentRoomId = roomId;
      this.isConnected = true;
      this.notifyConnectionChange(true);

      // Set up page unload handler to leave room properly
      this.setupUnloadHandler();

      // Set up WebSocket error handler
      this.wsErrorUnsubscribe = wsService.onError((error) => {
        console.error('[RoomService] WebSocket error:', error);
        this.notifyError(error);
      });

      console.log('✅ [RoomService] Joined room:', roomId);
    } catch (error) {
      console.error('❌ [RoomService] Failed to join room:', error);
      this.isConnected = false;
      this.notifyConnectionChange(false);
      throw error;
    }
  }

  leaveRoom(): void {
    if (!this.currentRoomId) return;

    // Send leave_room message
    if (this.isConnected) {
      wsService.send('leave_room', { roomId: this.currentRoomId });
    }

    // Remove unload handler
    this.removeUnloadHandler();

    // Unsubscribe from WebSocket errors
    if (this.wsErrorUnsubscribe) {
      this.wsErrorUnsubscribe();
      this.wsErrorUnsubscribe = null;
    }

    // Disconnect WebSocket
    wsService.disconnect();

    // Clear state
    this.currentRoomId = null;
    this.isConnected = false;
    this.participants = [];
    this.notifyConnectionChange(false);

    console.log('[RoomService] 👋 Left room');
  }

  /**
   * Reconnect to current room (useful for manual retry)
   */
  async reconnect(): Promise<void> {
    if (!this.currentRoomId || !this.currentUsername) {
      throw new Error('No room to reconnect to');
    }

    console.log('[RoomService] Attempting reconnect to room:', this.currentRoomId);
    this.isConnected = false;

    await wsService.reconnect();

    // Re-join room after reconnection
    const payload: JoinRoomPayload = {
      roomId: this.currentRoomId,
      username: this.currentUsername
    };
    wsService.send('join_room', payload);

    this.isConnected = true;
    this.notifyConnectionChange(true);
  }

  /**
   * Get current connection state
   */
  getConnectionState(): 'disconnected' | 'connecting' | 'connected' | 'reconnecting' {
    return wsService.getConnectionState();
  }

  getCurrentRoom(): string | null {
    return this.currentRoomId;
  }

  isConnectedToRoom(): boolean {
    return this.isConnected && this.currentRoomId !== null;
  }

  getParticipants(): Participant[] {
    return [...this.participants];
  }

  getUsername(): string {
    return this.currentUsername;
  }

  /**
   * Get reconnect attempt count
   */
  getReconnectAttempts(): number {
    return wsService.getReconnectAttempts();
  }

  // Event listeners
  onRoomState(callback: (state: RoomStatePayload) => void): () => void {
    this.roomStateListeners.add(callback);
    return () => this.roomStateListeners.delete(callback);
  }

  onUserJoined(callback: (payload: UserJoinedPayload) => void): () => void {
    this.userJoinedListeners.add(callback);
    return () => this.userJoinedListeners.delete(callback);
  }

  onUserLeft(callback: (payload: UserLeftPayload) => void): () => void {
    this.userLeftListeners.add(callback);
    return () => this.userLeftListeners.delete(callback);
  }

  onConnectionChange(callback: (connected: boolean) => void): () => void {
    this.connectionChangeListeners.add(callback);
    // Immediately notify current state
    callback(this.isConnectedToRoom());
    return () => this.connectionChangeListeners.delete(callback);
  }

  onError(callback: (error: Error) => void): () => void {
    this.errorListeners.add(callback);
    return () => this.errorListeners.delete(callback);
  }

  private setupMessageHandlers(): void {
    // Handle room_state
    wsService.on('room_state', (payload: RoomStatePayload) => {
      console.log('[RoomService] 📦 Received room state:', payload);
      this.participants = payload.participants || [];
      this.roomStateListeners.forEach(listener => {
        try {
          listener(payload);
        } catch (error) {
          console.error('[RoomService] Error in room_state handler:', error);
        }
      });
    });

    // Handle user_joined
    wsService.on('user_joined', (payload: UserJoinedPayload) => {
      console.log('[RoomService] 👤 User joined:', payload);
      const newParticipant: Participant = {
        id: payload.userId,
        username: payload.username,
        color: payload.color,
      };
      // Avoid duplicates
      if (!this.participants.find(p => p.id === payload.userId)) {
        this.participants = [...this.participants, newParticipant];
      }
      this.userJoinedListeners.forEach(listener => {
        try {
          listener(payload);
        } catch (error) {
          console.error('[RoomService] Error in user_joined handler:', error);
        }
      });
    });

    // Handle user_left
    wsService.on('user_left', (payload: UserLeftPayload) => {
      console.log('[RoomService] 👤 User left:', payload);
      this.participants = this.participants.filter(p => p.id !== payload.userId);
      this.userLeftListeners.forEach(listener => {
        try {
          listener(payload);
        } catch (error) {
          console.error('[RoomService] Error in user_left handler:', error);
        }
      });
    });

    // Handle connection state changes from WebSocket service
    wsService.onConnectionChange((connected) => {
      // Only update if we're in a room
      if (this.currentRoomId) {
        this.isConnected = connected;
        this.notifyConnectionChange(connected);

        // If reconnected, re-join room
        if (connected && wsService.getReconnectAttempts() > 0) {
          console.log('[RoomService] Reconnected, re-joining room:', this.currentRoomId);
          const payload: JoinRoomPayload = {
            roomId: this.currentRoomId,
            username: this.currentUsername
          };
          wsService.send('join_room', payload);
        }
      }
    });

    // Handle server errors
    wsService.on('error', (payload: ErrorPayload) => {
      console.error('[RoomService] ❌ Server error:', payload.message, payload.code);
      this.notifyError(new Error(`${payload.code}: ${payload.message}`));
    });
  }

  private notifyConnectionChange(connected: boolean): void {
    this.connectionChangeListeners.forEach(listener => {
      try {
        listener(connected);
      } catch (error) {
        console.error('[RoomService] Error in connection change handler:', error);
      }
    });
  }

  private notifyError(error: Error): void {
    this.errorListeners.forEach(listener => {
      try {
        listener(error);
      } catch (e) {
        console.error('[RoomService] Error in error handler:', e);
      }
    });
  }

  private getDefaultWsUrl(): string {
    const envWsUrl = import.meta.env.VITE_WS_URL;
    let baseUrl: string;

    if (envWsUrl) {
      baseUrl = envWsUrl;
    } else {
      const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
      const host = window.location.hostname;
      baseUrl = `${protocol}//${host}:8080/ws`;
    }

    const token = this.getStoredAccessToken();
    if (token) {
      const separator = baseUrl.includes('?') ? '&' : '?';
      return `${baseUrl}${separator}token=${encodeURIComponent(token)}`;
    }

    return baseUrl;
  }

  private getStoredAccessToken(): string | null {
    try {
      const raw = localStorage.getItem('auth-storage');
      if (!raw) return null;
      const parsed = JSON.parse(raw);
      return parsed?.state?.accessToken || null;
    } catch {
      return null;
    }
  }

  private setupUnloadHandler(): void {
    const handleBeforeUnload = () => {
      if (this.currentRoomId && this.isConnected) {
        console.log('[RoomService] 👋 Page unloading, sending leave_room');
        const data = JSON.stringify({
          type: 'leave_room',
          payload: { roomId: this.currentRoomId }
        });

        // Try WebSocket first
        wsService.send('leave_room', { roomId: this.currentRoomId });

        // Also try sendBeacon as fallback
        if (navigator.sendBeacon) {
          const blob = new Blob([data], { type: 'application/json' });
          navigator.sendBeacon('/ws', blob);
        }
      }
    };

    window.addEventListener('beforeunload', handleBeforeUnload);
    this.unloadHandler = handleBeforeUnload;
  }

  private removeUnloadHandler(): void {
    if (this.unloadHandler) {
      window.removeEventListener('beforeunload', this.unloadHandler);
      this.unloadHandler = undefined;
    }
  }
}

// Singleton instance
export const roomService = new RoomService();
