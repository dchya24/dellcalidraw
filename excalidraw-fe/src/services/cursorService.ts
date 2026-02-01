import { wsService } from './websocket';
import { roomService } from './roomService';
import type { CursorMovePayload, CursorUpdatedPayload, CursorPosition, RemoteCursor } from '../types/websocket';

class CursorService {
  private isTracking: boolean = false;
  private currentPosition: CursorPosition = { x: 0, y: 0 };
  private lastSentPosition: CursorPosition = { x: -1, y: -1 };
  private remoteCursors: Map<string, RemoteCursor> = new Map();
  private updateInterval: NodeJS.Timeout | null = null;
  private cursorUpdateListeners: Set<(cursor: RemoteCursor) => void> = new Set();
  private cursorRemoveListeners: Set<(userId: string) => void> = new Set();

  // Position threshold for sending updates (in pixels)
  private readonly POSITION_THRESHOLD = 5;
  // Update interval in milliseconds
  private readonly UPDATE_INTERVAL = 100;

  startTracking(getPosition: () => CursorPosition): void {
    if (this.isTracking) {
      console.log('Cursor tracking already started');
      return;
    }

    this.isTracking = true;
    this.setupMessageHandlers();

    // Start periodic updates
    this.updateInterval = setInterval(() => {
      const pos = getPosition();
      this.sendUpdate(pos);
    }, this.UPDATE_INTERVAL);

    console.log('✅ Cursor tracking started');
  }

  stopTracking(): void {
    if (!this.isTracking) return;

    this.isTracking = false;

    if (this.updateInterval) {
      clearInterval(this.updateInterval);
      this.updateInterval = null;
    }

    console.log('❌ Cursor tracking stopped');
  }

  isTrackingActive(): boolean {
    return this.isTracking && roomService.isConnectedToRoom();
  }

  // Event listeners
  onCursorUpdated(callback: (cursor: RemoteCursor) => void): () => void {
    this.cursorUpdateListeners.add(callback);
    return () => this.cursorUpdateListeners.delete(callback);
  }

  onCursorRemoved(callback: (userId: string) => void): () => void {
    this.cursorRemoveListeners.add(callback);
    return () => this.cursorRemoveListeners.delete(callback);
  }

  getRemoteCursors(): RemoteCursor[] {
    return Array.from(this.remoteCursors.values());
  }

  private sendUpdate(position: CursorPosition): void {
    if (!this.isTrackingActive()) {
      return;
    }

    // Check if position changed significantly
    if (
      Math.abs(position.x - this.lastSentPosition.x) < this.POSITION_THRESHOLD &&
      Math.abs(position.y - this.lastSentPosition.y) < this.POSITION_THRESHOLD
    ) {
      return;
    }

    const roomId = roomService.getCurrentRoom();
    if (!roomId) {
      return;
    }

    const payload: CursorMovePayload = {
      roomId,
      position,
    };

    const sent = wsService.send('cursor_move', payload);
    if (sent) {
      this.lastSentPosition = { ...position };
    }
  }

  private setupMessageHandlers(): void {
    wsService.on('cursor_updated', (payload: CursorUpdatedPayload) => {
      const cursor: RemoteCursor = {
        userId: payload.userId,
        username: payload.username,
        color: payload.color,
        position: payload.position,
        timestamp: Date.now(),
      };

      this.remoteCursors.set(payload.userId, cursor);

      // Remove cursor after 5 seconds of inactivity
      setTimeout(() => {
        const existing = this.remoteCursors.get(payload.userId);
        if (existing && existing.timestamp === cursor.timestamp) {
          this.remoteCursors.delete(payload.userId);
          this.cursorRemoveListeners.forEach(listener => {
            try {
              listener(payload.userId);
            } catch (error) {
              console.error('Error in cursor remove handler:', error);
            }
          });
        }
      }, 5000);

      this.cursorUpdateListeners.forEach(listener => {
        try {
          listener(cursor);
        } catch (error) {
          console.error('Error in cursor_updated handler:', error);
        }
      });
    });
  }
}

// Singleton instance
export const cursorService = new CursorService();
