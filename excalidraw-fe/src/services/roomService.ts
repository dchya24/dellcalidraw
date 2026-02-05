import { wsService } from './websocket';
import type { JoinRoomPayload, RoomStatePayload, UserJoinedPayload, UserLeftPayload, Participant } from '../types/websocket';

interface RoomServiceConfig {
  wsUrl: string;
  username?: string;
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
  private unloadHandler: ((this: Window, ev: Event) => unknown) | undefined;

  async joinRoom(roomId: string, username: string, config?: RoomServiceConfig): Promise<void> {
    if (this.currentRoomId === roomId && this.isConnected) {
      console.log('Already joined room:', roomId);
      return;
    }

    // Leave current room if any
    if (this.currentRoomId) {
      this.leaveRoom();
    }

    const wsUrl = config?.wsUrl || this.getDefaultWsUrl();
    console.log('Connecting to WebSocket:', wsUrl);
    this.currentUsername = username;

    try {
      // Set up message handlers BEFORE connecting
      this.setupMessageHandlers();

      // Connect to WebSocket
      await wsService.connect({
        url: wsUrl,
        reconnectInterval: 3000,
        maxReconnectAttempts: 5,
      });

      // Send join_room message
      const payload: JoinRoomPayload = { roomId, username };
      console.log('Sending join_room message:', payload);
      const sent = wsService.send('join_room', payload);

      if (!sent) {
        throw new Error('Failed to send join_room message');
      }

      this.currentRoomId = roomId;
      this.isConnected = true;
      this.notifyConnectionChange(true);

      // Set up page unload handler to leave room properly
      this.setupUnloadHandler();

      console.log('✅ Joined room:', roomId);
    } catch (error) {
      console.error('❌ Failed to join room:', error);
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

    // Disconnect WebSocket
    wsService.disconnect();

    // Clear state
    this.currentRoomId = null;
    this.isConnected = false;
    this.participants = [];
    this.notifyConnectionChange(false);

    console.log('👋 Left room');
  }

  getCurrentRoom(): string | null {
    return this.currentRoomId;
  }

  isConnectedToRoom(): boolean {
    return this.isConnected && this.currentRoomId !== null;
  }

  getParticipants(): Participant[] {
    console.log('Participants:', this.participants);
    return [...this.participants];
  }

  getUsername(): string {
    return this.currentUsername;
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
    return () => this.connectionChangeListeners.delete(callback);
  }

  private setupMessageHandlers(): void {
    // Handle room_state
    wsService.on('room_state', (payload: RoomStatePayload) => {
      console.log('📦 Received room state:', payload);
      this.participants = payload.participants || [];
      this.roomStateListeners.forEach(listener => {
        try {
          listener(payload);
        } catch (error) {
          console.error('Error in room_state handler:', error);
        }
      });
    });

    // Handle user_joined
    wsService.on('user_joined', (payload: UserJoinedPayload) => {
      console.log('👤 User joined:', payload);
      // Add the new user to participants list
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
          console.error('Error in user_joined handler:', error);
        }
      });
    });

    // Handle user_left
    wsService.on('user_left', (payload: UserLeftPayload) => {
      console.log('👤 User left:', payload);
      this.participants = this.participants.filter(p => p.id !== payload.userId);
      this.userLeftListeners.forEach(listener => {
        try {
          listener(payload);
        } catch (error) {
          console.error('Error in user_left handler:', error);
        }
      });
    });

    // Handle connection state changes
    wsService.onConnectionChange((connected) => {
      this.isConnected = connected;
      this.notifyConnectionChange(connected);
    });
  }

  private notifyConnectionChange(connected: boolean): void {
    this.connectionChangeListeners.forEach(listener => {
      try {
        listener(connected);
      } catch (error) {
        console.error('Error in connection change handler:', error);
      }
    });
  }

  private getDefaultWsUrl(): string {
    // Check for environment variable first
    const envWsUrl = import.meta.env.VITE_WS_URL;
    if (envWsUrl) {
      return envWsUrl;
    }

    // Fallback to dynamic URL based on current location
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const host = window.location.hostname;
    return `${protocol}//${host}:8080/ws`;
  }

  private setupUnloadHandler(): void {
    // Handle visibility change (more reliable than beforeunload)
    const handleVisibilityChange = () => {
      if (document.visibilityState === 'hidden' && this.isConnected) {
        console.log('👋 Tab hidden, sending leave_room');
        // Send leave_room immediately when tab is hidden
        wsService.send('leave_room', { roomId: this.currentRoomId });
        // Mark as disconnected to prevent reconnection
        this.isConnected = false;
      }
    };

    // Handle page unload (best effort)
    const handleBeforeUnload = () => {
      if (this.currentRoomId && this.isConnected) {
        console.log('👋 Page unloading, sending leave_room');
        // Use sendBeacon for more reliable delivery during unload
        const data = JSON.stringify({
          type: 'leave_room',
          payload: { roomId: this.currentRoomId }
        });

        // Try WebSocket first
        wsService.send('leave_room', { roomId: this.currentRoomId });

        // Also try sendBeacon as fallback
        if (navigator.sendBeacon) {
          const blob = new Blob([data], { type: 'application/json' });
          navigator.sendBeacon(this.getDefaultWsUrl(), blob);
        }
      }
    };

    // Listen for visibility change (most reliable)
    document.addEventListener('visibilitychange', handleVisibilityChange);

    // Also listen for beforeunload as backup
    window.addEventListener('beforeunload', handleBeforeUnload);

    // Store handler reference for cleanup
    this.unloadHandler = handleVisibilityChange;
  }

  private removeUnloadHandler(): void {
    if (this.unloadHandler) {
      document.removeEventListener('visibilitychange', this.unloadHandler);
      window.removeEventListener('beforeunload', this.unloadHandler);
      this.unloadHandler = undefined;
    }
  }
}

// Singleton instance
export const roomService = new RoomService();
