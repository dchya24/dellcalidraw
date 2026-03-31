import { WSMessage } from '../types/websocket';

export interface WSConfig {
  url: string;
  reconnectInterval?: number;
  maxReconnectAttempts?: number;
  enableHeartbeat?: boolean;
  heartbeatInterval?: number;
  heartbeatTimeout?: number;
}

export interface PendingMessage {
  id: string;
  type: string;
  payload: unknown;
  timestamp: number;
  retryCount: number;
}

export interface AckCallback {
  resolve: (value: unknown) => void;
  reject: (reason: Error) => void;
  timeout: ReturnType<typeof setTimeout>;
}

type MessageHandler = (payload: unknown) => void;
type ConnectionChangeHandler = (connected: boolean) => void;
type ErrorHandler = (error: Error) => void;

class WebSocketService {
  private ws: WebSocket | null = null;
  private reconnectAttempts = 0;
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null;
  private messageHandlers: Map<string, Set<MessageHandler>> = new Map();
  private connectionChangeHandlers: Set<ConnectionChangeHandler> = new Set();
  private errorHandlers: Set<ErrorHandler> = new Set();
  private isManualDisconnect = false;
  private config: WSConfig | null = null;

  // Message queue for offline/reconnect scenarios
  private messageQueue: PendingMessage[] = [];
  private maxQueueSize = 100;

  // Heartbeat system
  private heartbeatTimer: ReturnType<typeof setInterval> | null = null;
  private heartbeatTimeoutTimer: ReturnType<typeof setTimeout> | null = null;
  private lastPongReceived = 0;

  // Acknowledgment system for request-response pattern
  private pendingAcks: Map<string, AckCallback> = new Map();
  private ackTimeout = 10000; // 10 seconds
  private ackIdCounter = 0;

  // Connection state
  private connectionState: 'disconnected' | 'connecting' | 'connected' | 'reconnecting' = 'disconnected';

  /**
   * Connect to WebSocket server with enhanced features
   */
  connect(config: WSConfig): Promise<void> {
    return new Promise((resolve, reject) => {
      this.config = {
        enableHeartbeat: true,
        heartbeatInterval: 30000, // 30 seconds
        heartbeatTimeout: 10000,  // 10 seconds timeout for pong
        ...config
      };

      this.isManualDisconnect = false;
      this.connectionState = 'connecting';

      console.log('🔌 [WebSocket] Attempting connection to:', config.url);

      try {
        this.ws = new WebSocket(config.url);

        this.ws.onopen = () => {
          console.log('✅ [WebSocket] Connected successfully', {
            url: config.url,
            readyState: this.ws?.readyState,
            reconnectAttempts: this.reconnectAttempts
          });

          this.connectionState = 'connected';
          this.reconnectAttempts = 0;
          this.lastPongReceived = Date.now();

          // Start heartbeat
          this.startHeartbeat();

          // Flush any queued messages
          this.flushMessageQueue();

          this.notifyConnectionChange(true);
          resolve();
        };

        this.ws.onmessage = (event) => {
          try {
            const message: WSMessage = JSON.parse(event.data);
            this.handleMessage(message);
          } catch (error) {
            console.error('[WebSocket] Failed to parse message:', error);
            this.notifyError(new Error('Failed to parse WebSocket message'));
          }
        };

        this.ws.onerror = (error) => {
          console.error('❌ [WebSocket] Error:', error);
          this.connectionState = 'disconnected';
          this.notifyConnectionChange(false);
          this.notifyError(new Error('WebSocket connection error'));
          reject(error);
        };

        this.ws.onclose = (event) => {
          console.log('❌ [WebSocket] Disconnected', {
            code: event.code,
            reason: event.reason,
            wasClean: event.wasClean,
            state: this.connectionState
          });

          this.stopHeartbeat();
          this.connectionState = 'disconnected';
          this.notifyConnectionChange(false);

          // Auto-reconnect if not manually disconnected
          if (!this.isManualDisconnect) {
            this.scheduleReconnect();
          }
        };
      } catch (error) {
        console.error('❌ [WebSocket] Failed to create connection:', error);
        this.connectionState = 'disconnected';
        reject(error);
      }
    });
  }

  /**
   * Handle incoming messages including ack responses and heartbeats
   */
  private handleMessage(message: WSMessage) {
    const { type, payload } = message;

    // Handle heartbeat pong
    if (type === 'pong') {
      this.lastPongReceived = Date.now();
      this.clearHeartbeatTimeout();
      return;
    }

    // Handle heartbeat ping from server
    if (type === 'ping') {
      this.send('pong', {});
      return;
    }

    // Handle acknowledgment responses
    if (type === 'ack' && payload && typeof payload === 'object') {
      const ackPayload = payload as { ackId: string; data: unknown };
      const ackCallback = this.pendingAcks.get(ackPayload.ackId);
      if (ackCallback) {
        clearTimeout(ackCallback.timeout);
        ackCallback.resolve(ackPayload.data);
        this.pendingAcks.delete(ackPayload.ackId);
        return;
      }
    }

    // Handle ack errors
    if (type === 'ack_error' && payload && typeof payload === 'object') {
      const errorPayload = payload as { ackId: string; error: string };
      const ackCallback = this.pendingAcks.get(errorPayload.ackId);
      if (ackCallback) {
        clearTimeout(ackCallback.timeout);
        ackCallback.reject(new Error(errorPayload.error));
        this.pendingAcks.delete(errorPayload.ackId);
        return;
      }
    }

    // Normal message handling
    const handlers = this.messageHandlers.get(type);
    if (handlers) {
      handlers.forEach(handler => {
        try {
          handler(payload);
        } catch (error) {
          console.error(`[WebSocket] Error handling message type "${type}":`, error);
        }
      });
    }
  }

  /**
   * Send message with acknowledgment (Socket.io-style emit with callback)
   */
  emit(type: string, payload: Record<string, unknown>, timeout?: number): Promise<unknown> {
    return new Promise((resolve, reject) => {
      if (!this.isConnected()) {
        reject(new Error('WebSocket not connected'));
        return;
      }

      const ackId = `${++this.ackIdCounter}-${Date.now()}`;
      const messagePayload = {
        ...payload,
        _ackId: ackId
      };

      // Set up timeout for acknowledgment
      const timeoutMs = timeout || this.ackTimeout;
      const timeoutTimer = setTimeout(() => {
        this.pendingAcks.delete(ackId);
        reject(new Error(`Acknowledgment timeout for message type "${type}"`));
      }, timeoutMs);

      // Store callback
      this.pendingAcks.set(ackId, {
        resolve,
        reject,
        timeout: timeoutTimer
      });

      // Send message
      const sent = this.send(type, messagePayload);
      if (!sent) {
        clearTimeout(timeoutTimer);
        this.pendingAcks.delete(ackId);
        reject(new Error('Failed to send message'));
      }
    });
  }

  /**
   * Subscribe to event (Socket.io-style on)
   */
  on(eventType: string, handler: MessageHandler): () => void {
    if (!this.messageHandlers.has(eventType)) {
      this.messageHandlers.set(eventType, new Set());
    }
    this.messageHandlers.get(eventType)!.add(handler);

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

  /**
   * Subscribe to event once (Socket.io-style once)
   */
  once(eventType: string, handler: MessageHandler): void {
    const unsubscribe = this.on(eventType, (payload) => {
      unsubscribe();
      handler(payload);
    });
  }

  /**
   * Send message - queues if offline
   */
  send(type: string, payload: unknown): boolean {
    if (!this.isConnected()) {
      // Queue message for later if offline
      this.queueMessage(type, payload);
      return false;
    }

    try {
      const message = JSON.stringify({ type, payload });
      this.ws!.send(message);
      return true;
    } catch (error) {
      console.error('[WebSocket] Failed to send message:', error);
      // Queue for retry
      this.queueMessage(type, payload);
      return false;
    }
  }

  /**
   * Queue message for later delivery
   */
  private queueMessage(type: string, payload: unknown) {
    if (this.messageQueue.length >= this.maxQueueSize) {
      // Remove oldest message
      this.messageQueue.shift();
    }

    const pendingMsg: PendingMessage = {
      id: `${Date.now()}-${Math.random().toString(36).substr(2, 9)}`,
      type,
      payload,
      timestamp: Date.now(),
      retryCount: 0
    };

    this.messageQueue.push(pendingMsg);
    console.log(`[WebSocket] Message queued (queue size: ${this.messageQueue.length})`);
  }

  /**
   * Flush queued messages after reconnection
   */
  private flushMessageQueue() {
    if (this.messageQueue.length === 0) return;

    console.log(`[WebSocket] Flushing ${this.messageQueue.length} queued messages`);

    // Copy queue and clear
    const queue = [...this.messageQueue];
    this.messageQueue = [];

    // Send all queued messages
    queue.forEach(msg => {
      const sent = this.send(msg.type, msg.payload);
      if (!sent) {
        // Re-queue if still not connected
        this.queueMessage(msg.type, msg.payload);
      }
    });
  }

  /**
   * Start heartbeat/ping system
   */
  private startHeartbeat() {
    if (!this.config?.enableHeartbeat) return;

    const interval = this.config.heartbeatInterval || 30000;

    this.heartbeatTimer = setInterval(() => {
      if (this.isConnected()) {
        // Send ping
        this.send('ping', {});

        // Set timeout for pong response
        const timeout = this.config?.heartbeatTimeout || 10000;
        this.heartbeatTimeoutTimer = setTimeout(() => {
          const timeSinceLastPong = Date.now() - this.lastPongReceived;
          if (timeSinceLastPong > timeout + interval) {
            console.warn('[WebSocket] Heartbeat timeout - connection may be dead');
            // Force reconnect
            this.ws?.close();
          }
        }, timeout);
      }
    }, interval);

    console.log('[WebSocket] Heartbeat started');
  }

  /**
   * Stop heartbeat system
   */
  private stopHeartbeat() {
    if (this.heartbeatTimer) {
      clearInterval(this.heartbeatTimer);
      this.heartbeatTimer = null;
    }
    this.clearHeartbeatTimeout();
  }

  /**
   * Clear heartbeat timeout
   */
  private clearHeartbeatTimeout() {
    if (this.heartbeatTimeoutTimer) {
      clearTimeout(this.heartbeatTimeoutTimer);
      this.heartbeatTimeoutTimer = null;
    }
  }

  /**
   * Schedule reconnection with exponential backoff
   */
  private scheduleReconnect() {
    if (!this.config || this.isManualDisconnect) return;

    const maxAttempts = this.config.maxReconnectAttempts || 5;

    if (this.reconnectAttempts >= maxAttempts) {
      console.error('[WebSocket] Max reconnection attempts reached');
      this.notifyError(new Error('Max reconnection attempts reached'));
      return;
    }

    this.connectionState = 'reconnecting';
    this.reconnectAttempts++;

    // Exponential backoff with jitter
    const baseDelay = this.config.reconnectInterval || 1000;
    const exponentialDelay = Math.min(baseDelay * Math.pow(2, this.reconnectAttempts - 1), 30000);
    const jitter = Math.random() * 1000; // Add 0-1000ms random jitter
    const delay = exponentialDelay + jitter;

    console.log(`[WebSocket] Reconnecting... Attempt ${this.reconnectAttempts}/${maxAttempts} in ${Math.round(delay)}ms`);

    this.reconnectTimer = setTimeout(() => {
      if (!this.isManualDisconnect && this.connectionState === 'reconnecting') {
        this.connect(this.config!).catch((error) => {
          console.error('[WebSocket] Reconnection failed:', error);
        });
      }
    }, delay);
  }

  /**
   * Cancel scheduled reconnection
   */
  private cancelReconnect() {
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }
  }

  /**
   * Disconnect WebSocket
   */
  disconnect() {
    this.isManualDisconnect = true;
    this.cancelReconnect();
    this.stopHeartbeat();

    // Reject all pending acks
    this.pendingAcks.forEach((callback) => {
      clearTimeout(callback.timeout);
      callback.reject(new Error('Connection closed'));
    });
    this.pendingAcks.clear();

    if (this.ws) {
      this.ws.close(1000, 'Manual disconnect'); // 1000 = Normal closure
      this.ws = null;
    }

    this.connectionState = 'disconnected';
    this.reconnectAttempts = 0;

    console.log('[WebSocket] Disconnected manually');
  }

  /**
   * Reconnect manually (useful for "Retry" buttons)
   */
  reconnect(): Promise<void> {
    this.cancelReconnect();
    this.reconnectAttempts = 0;
    this.isManualDisconnect = false;

    if (this.config) {
      return this.connect(this.config);
    }
    return Promise.reject(new Error('No config available for reconnect'));
  }

  /**
   * Subscribe to connection state changes
   */
  onConnectionChange(handler: ConnectionChangeHandler): () => void {
    this.connectionChangeHandlers.add(handler);
    // Immediately notify current state
    handler(this.isConnected());

    return () => {
      this.connectionChangeHandlers.delete(handler);
    };
  }

  /**
   * Subscribe to errors
   */
  onError(handler: ErrorHandler): () => void {
    this.errorHandlers.add(handler);
    return () => {
      this.errorHandlers.delete(handler);
    };
  }

  private notifyConnectionChange(connected: boolean) {
    this.connectionChangeHandlers.forEach(handler => {
      try {
        handler(connected);
      } catch (error) {
        console.error('[WebSocket] Error in connection change handler:', error);
      }
    });
  }

  private notifyError(error: Error) {
    this.errorHandlers.forEach(handler => {
      try {
        handler(error);
      } catch (e) {
        console.error('[WebSocket] Error in error handler:', e);
      }
    });
  }

  /**
   * Check if WebSocket is connected
   */
  isConnected(): boolean {
    return this.ws !== null && this.ws.readyState === WebSocket.OPEN;
  }

  /**
   * Get current connection state
   */
  getConnectionState(): 'disconnected' | 'connecting' | 'connected' | 'reconnecting' {
    return this.connectionState;
  }

  /**
   * Get reconnection attempt count
   */
  getReconnectAttempts(): number {
    return this.reconnectAttempts;
  }

  /**
   * Get queued message count
   */
  getQueueSize(): number {
    return this.messageQueue.length;
  }

  /**
   * Clear message queue
   */
  clearQueue(): void {
    this.messageQueue = [];
    console.log('[WebSocket] Message queue cleared');
  }
}

// Singleton instance
export const wsService = new WebSocketService();
