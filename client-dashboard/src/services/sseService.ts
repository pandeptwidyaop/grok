type EventCallback = (data: any) => void;
type ConnectionCallback = (connected: boolean) => void;

/**
 * SSE Service for managing Server-Sent Events connection
 * Singleton pattern with auto-reconnect
 */
class SSEService {
  private eventSource: EventSource | null = null;
  private subscribers: Map<string, Set<EventCallback>> = new Map();
  private connectionListeners: Set<ConnectionCallback> = new Set();
  private reconnectAttempts = 0;
  private maxReconnectAttempts = 5;
  private reconnectDelay = 1000; // Start with 1 second
  private maxReconnectDelay = 30000; // Max 30 seconds
  private reconnectTimeout: number | null = null;
  private isConnected = false;
  private url: string;

  constructor() {
    this.url = import.meta.env.DEV
      ? 'http://localhost:4041/api/sse'
      : '/api/sse';
  }

  /**
   * Connect to SSE endpoint
   */
  connect(): void {
    if (this.eventSource) {
      return; // Already connected
    }

    console.log('[SSE] Connecting to', this.url);

    this.eventSource = new EventSource(this.url);

    this.eventSource.onopen = () => {
      console.log('[SSE] Connected');
      this.reconnectAttempts = 0;
      this.reconnectDelay = 1000;
      this.isConnected = true;
      this.notifyConnectionListeners(true);
    };

    this.eventSource.onerror = (error) => {
      console.error('[SSE] Error:', error);
      this.isConnected = false;
      this.notifyConnectionListeners(false);

      // Close current connection
      this.eventSource?.close();
      this.eventSource = null;

      // Attempt reconnection
      this.scheduleReconnect();
    };

    // Listen for all event types
    this.setupEventListeners();
  }

  /**
   * Setup event listeners for all event types
   */
  private setupEventListeners(): void {
    if (!this.eventSource) return;

    // Generic message handler
    this.eventSource.onmessage = (event) => {
      try {
        const data = JSON.parse(event.data);
        this.notifySubscribers('message', data);
      } catch (error) {
        console.error('[SSE] Failed to parse message:', error);
      }
    };

    // Specific event handlers
    const eventTypes = [
      'connected',
      'request_completed',
      'connection_established',
      'connection_lost',
      'metrics_update',
    ];

    eventTypes.forEach((eventType) => {
      this.eventSource!.addEventListener(eventType, (event: Event) => {
        try {
          const messageEvent = event as MessageEvent;
          const data = JSON.parse(messageEvent.data);
          this.notifySubscribers(eventType, data);
        } catch (error) {
          console.error(`[SSE] Failed to parse ${eventType} event:`, error);
        }
      });
    });
  }

  /**
   * Schedule reconnection with exponential backoff
   */
  private scheduleReconnect(): void {
    if (this.reconnectAttempts >= this.maxReconnectAttempts) {
      console.error('[SSE] Max reconnection attempts reached');
      return;
    }

    this.reconnectAttempts++;
    const delay = Math.min(
      this.reconnectDelay * Math.pow(2, this.reconnectAttempts - 1),
      this.maxReconnectDelay
    );

    console.log(`[SSE] Reconnecting in ${delay}ms (attempt ${this.reconnectAttempts}/${this.maxReconnectAttempts})`);

    this.reconnectTimeout = window.setTimeout(() => {
      this.connect();
    }, delay);
  }

  /**
   * Disconnect from SSE endpoint
   */
  disconnect(): void {
    console.log('[SSE] Disconnecting');

    if (this.reconnectTimeout) {
      clearTimeout(this.reconnectTimeout);
      this.reconnectTimeout = null;
    }

    if (this.eventSource) {
      this.eventSource.close();
      this.eventSource = null;
    }

    this.isConnected = false;
    this.reconnectAttempts = 0;
    this.notifyConnectionListeners(false);
  }

  /**
   * Subscribe to specific event type
   */
  subscribe(eventType: string, callback: EventCallback): () => void {
    if (!this.subscribers.has(eventType)) {
      this.subscribers.set(eventType, new Set());
    }

    this.subscribers.get(eventType)!.add(callback);

    // Return unsubscribe function
    return () => {
      const callbacks = this.subscribers.get(eventType);
      if (callbacks) {
        callbacks.delete(callback);
        if (callbacks.size === 0) {
          this.subscribers.delete(eventType);
        }
      }
    };
  }

  /**
   * Subscribe to connection status changes
   */
  onConnectionChange(callback: ConnectionCallback): () => void {
    this.connectionListeners.add(callback);

    // Immediately notify with current status
    callback(this.isConnected);

    // Return unsubscribe function
    return () => {
      this.connectionListeners.delete(callback);
    };
  }

  /**
   * Notify all subscribers of an event
   */
  private notifySubscribers(eventType: string, data: any): void {
    const callbacks = this.subscribers.get(eventType);
    if (callbacks) {
      callbacks.forEach((callback) => {
        try {
          callback(data);
        } catch (error) {
          console.error(`[SSE] Error in ${eventType} callback:`, error);
        }
      });
    }
  }

  /**
   * Notify connection listeners
   */
  private notifyConnectionListeners(connected: boolean): void {
    this.connectionListeners.forEach((callback) => {
      try {
        callback(connected);
      } catch (error) {
        console.error('[SSE] Error in connection callback:', error);
      }
    });
  }

  /**
   * Get connection status
   */
  getConnectionStatus(): boolean {
    return this.isConnected;
  }

  /**
   * Reset reconnection attempts (useful for manual reconnect)
   */
  resetReconnectAttempts(): void {
    this.reconnectAttempts = 0;
    this.reconnectDelay = 1000;
  }
}

// Export singleton instance
export const sseService = new SSEService();
export default sseService;
