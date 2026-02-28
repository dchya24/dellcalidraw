import { WSMessage } from '../types/websocket';

export interface WSConfig {
  url: string;
  reconnectInterval?: number;
  maxReconnectAttempts?: number;
}

type MessageHandler = (payload: unknown) => void;
type ConnectionChangeHandler = (connected: boolean) => void;

class WebSocketService {
  private ws: WebSocket | null = null;
  private reconnectAttempts = 0;
  private messageHandlers: Map<string, Set<MessageHandler>> = new Map();
  private connectionChangeHandlers: Set<ConnectionChangeHandler> = new Set();
  private isManualDisconnect = false;
  private config: WSConfig | null = null;

  connect(config: WSConfig): Promise<void> {
    return new Promise((resolve, reject) => {
      this.config = config;
      this.isManualDisconnect = false;

      console.log('🔌 Attempting WebSocket connection to:', config.url);

      try {
        this.ws = new WebSocket(config.url);

        this.ws.onopen = () => {
          console.log('✅ WebSocket connected', { url: config.url, readyState: this.ws?.readyState });
          this.reconnectAttempts = 0;
          this.notifyConnectionChange(true);
          resolve();
        };

        this.ws.onmessage = (event) => {
          console.log('📥 Received WebSocket message:', event.data);
          try {
            const message: WSMessage = JSON.parse(event.data);
            this.handleMessage(message);
          } catch (error) {
            console.error('Failed to parse WebSocket message:', error);
          }
        };

        this.ws.onerror = (error) => {
          console.error('❌ WebSocket error:', error, { readyState: this.ws?.readyState });
          this.notifyConnectionChange(false);
          reject(error);
        };

        this.ws.onclose = (event) => {
          console.log('❌ WebSocket disconnected', { code: event.code, reason: event.reason, wasClean: event.wasClean });
          this.notifyConnectionChange(false);

          if (!this.isManualDisconnect) {
            this.attemptReconnect();
          }
        };
      } catch (error) {
        console.error('❌ Failed to create WebSocket:', error);
        reject(error);
      }
    });
  }

  private handleMessage(message: WSMessage) {
    const { type, payload } = message;
    const handlers = this.messageHandlers.get(type);

    if (handlers) {
      handlers.forEach(handler => {
        try {
          handler(payload);
        } catch (error) {
          console.error(`Error handling message type "${type}":`, error);
        }
      });
    } else {
      console.log(`No handlers for message type: ${type}`);
    }
  }

  on(eventType: string, handler: MessageHandler): () => void {
    if (!this.messageHandlers.has(eventType)) {
      this.messageHandlers.set(eventType, new Set());
    }
    this.messageHandlers.get(eventType)!.add(handler);

    // Return unsubscribe function
    return () => {
      const handlers = this.messageHandlers.get(eventType);
      if (handlers) {
        handlers.delete(handler);
        if (handlers.size === 0) {
          this.messageHandlers.delete(eventType);
        }
      }
    };
  }

  send(type: string, payload: unknown): boolean {
    if (!this.ws || this.ws.readyState !== WebSocket.OPEN) {
      console.warn('Cannot send message: WebSocket not connected', { type, readyState: this.ws?.readyState });
      return false;
    }

    try {
      const message = JSON.stringify({ type, payload });
      console.log('📤 Sending WebSocket message:', { type, payload, readyState: this.ws.readyState });
      this.ws.send(message);
      console.log('✅ Message sent successfully');
      return true;
    } catch (error) {
      console.error('❌ Failed to send WebSocket message:', error);
      return false;
    }
  }

  disconnect() {
    this.isManualDisconnect = true;
    if (this.ws) {
      this.ws.close();
      this.ws = null;
    }
    this.reconnectAttempts = 0;
  }

  onConnectionChange(handler: ConnectionChangeHandler): () => void {
    this.connectionChangeHandlers.add(handler);
    return () => {
      this.connectionChangeHandlers.delete(handler);
    };
  }

  private notifyConnectionChange(connected: boolean) {
    this.connectionChangeHandlers.forEach(handler => {
      try {
        handler(connected);
      } catch (error) {
        console.error('Error in connection change handler:', error);
      }
    });
  }

  private attemptReconnect() {
    if (!this.config) return;

    const maxAttempts = this.config.maxReconnectAttempts || 5;
    if (this.reconnectAttempts >= maxAttempts) {
      console.error('Max reconnection attempts reached');
      return;
    }

    this.reconnectAttempts++;
    const interval = this.config.reconnectInterval || 3000;

    console.log(`Reconnecting... Attempt ${this.reconnectAttempts}/${maxAttempts}`);

    setTimeout(() => {
      if (!this.isManualDisconnect) {
        this.connect(this.config!).catch((error) => {
          console.error('Reconnection failed:', error);
        });
      }
    }, interval);
  }

  isConnected(): boolean {
    return this.ws !== null && this.ws.readyState === WebSocket.OPEN;
  }
}

// Singleton instance
export const wsService = new WebSocketService();
