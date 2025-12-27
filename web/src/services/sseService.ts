/**
 * Global SSE Service
 * Maintains a single EventSource connection that persists across route changes
 * Multiple components can subscribe/unsubscribe without affecting the connection
 */

export interface SSEEvent {
  type: string;
  data: any;
}

export type SSEEventHandler = (event: SSEEvent) => void;

class SSEService {
  private eventSource: EventSource | null = null;
  private subscribers: Map<string, Set<SSEEventHandler>> = new Map();
  private reconnectTimeout: number | null = null;
  private reconnectAttempts = 0;
  private isAuthenticated = false;
  private baseURL = '';

  private readonly MAX_RECONNECT_ATTEMPTS = 5;
  private readonly BASE_RECONNECT_DELAY = 1000; // 1 second
  private readonly MAX_RECONNECT_DELAY = 30000; // 30 seconds

  /**
   * Initialize the SSE service
   * Should be called once when the app starts or when user authenticates
   */
  public init(authenticated: boolean, apiURL?: string) {
    this.isAuthenticated = authenticated;
    this.baseURL = apiURL || import.meta.env.VITE_API_URL || '';

    if (authenticated && !this.eventSource) {
      this.connect();
    } else if (!authenticated && this.eventSource) {
      this.disconnect();
    }
  }

  /**
   * Connect to SSE endpoint
   */
  private connect() {
    if (!this.isAuthenticated) {
      console.log('[SSE Service] Not authenticated, skipping connection');
      return;
    }

    // Don't create duplicate connections
    if (this.eventSource?.readyState === EventSource.OPEN) {
      console.log('[SSE Service] Already connected');
      return;
    }

    const url = `${this.baseURL}/api/sse`;
    console.log('[SSE Service] Connecting to:', url);

    try {
      this.eventSource = new EventSource(url, {
        withCredentials: true,
      });

      this.eventSource.onopen = () => {
        console.log('[SSE Service] ✅ Connection established');
        this.reconnectAttempts = 0; // Reset on successful connection
        this.notifyConnectionStatus(true);
      };

      this.eventSource.onerror = (error) => {
        console.error('[SSE Service] ❌ Connection error:', error);
        this.notifyConnectionStatus(false);

        // Close failed connection
        this.eventSource?.close();
        this.eventSource = null;

        // Attempt reconnection with exponential backoff
        this.scheduleReconnect();
      };

      this.eventSource.onmessage = (event) => {
        try {
          const data: SSEEvent = JSON.parse(event.data);
          console.log('[SSE Service] 📨 Event received:', data.type);
          this.broadcast(data);
        } catch (err) {
          console.error('[SSE Service] Failed to parse event:', err);
        }
      };
    } catch (err) {
      console.error('[SSE Service] Failed to create EventSource:', err);
      this.scheduleReconnect();
    }
  }

  /**
   * Schedule reconnection with exponential backoff
   */
  private scheduleReconnect() {
    if (this.reconnectAttempts >= this.MAX_RECONNECT_ATTEMPTS) {
      console.error('[SSE Service] Max reconnection attempts reached');
      return;
    }

    const delay = Math.min(
      this.BASE_RECONNECT_DELAY * Math.pow(2, this.reconnectAttempts),
      this.MAX_RECONNECT_DELAY
    );

    console.log(
      `[SSE Service] 🔄 Reconnecting in ${delay}ms (attempt ${this.reconnectAttempts + 1}/${this.MAX_RECONNECT_ATTEMPTS})`
    );

    this.reconnectTimeout = window.setTimeout(() => {
      this.reconnectAttempts++;
      this.connect();
    }, delay);
  }

  /**
   * Disconnect from SSE endpoint
   */
  public disconnect() {
    console.log('[SSE Service] Disconnecting...');

    if (this.reconnectTimeout) {
      clearTimeout(this.reconnectTimeout);
      this.reconnectTimeout = null;
    }

    if (this.eventSource) {
      this.eventSource.close();
      this.eventSource = null;
    }

    this.reconnectAttempts = 0;
    this.notifyConnectionStatus(false);
  }

  /**
   * Subscribe to a specific event type
   * @param eventType - The event type to listen for (e.g., 'tunnel_created', '*' for all)
   * @param handler - Callback function to handle the event
   * @returns Unsubscribe function
   */
  public subscribe(eventType: string, handler: SSEEventHandler): () => void {
    if (!this.subscribers.has(eventType)) {
      this.subscribers.set(eventType, new Set());
    }

    this.subscribers.get(eventType)!.add(handler);
    console.log(`[SSE Service] 📥 Subscribed to '${eventType}' (${this.subscribers.get(eventType)!.size} subscribers)`);

    // Return unsubscribe function
    return () => {
      const handlers = this.subscribers.get(eventType);
      if (handlers) {
        handlers.delete(handler);
        console.log(`[SSE Service] 📤 Unsubscribed from '${eventType}' (${handlers.size} remaining)`);

        // Clean up empty sets
        if (handlers.size === 0) {
          this.subscribers.delete(eventType);
        }
      }
    };
  }

  /**
   * Broadcast event to all subscribers
   */
  private broadcast(event: SSEEvent) {
    // Notify specific event type subscribers
    const typeHandlers = this.subscribers.get(event.type);
    if (typeHandlers) {
      typeHandlers.forEach(handler => {
        try {
          handler(event);
        } catch (err) {
          console.error(`[SSE Service] Error in handler for '${event.type}':`, err);
        }
      });
    }

    // Notify wildcard subscribers (listening to all events)
    const wildcardHandlers = this.subscribers.get('*');
    if (wildcardHandlers) {
      wildcardHandlers.forEach(handler => {
        try {
          handler(event);
        } catch (err) {
          console.error('[SSE Service] Error in wildcard handler:', err);
        }
      });
    }
  }

  /**
   * Notify connection status to subscribers
   */
  private notifyConnectionStatus(connected: boolean) {
    const event: SSEEvent = {
      type: connected ? '_connected' : '_disconnected',
      data: { connected },
    };
    this.broadcast(event);
  }

  /**
   * Check if currently connected
   */
  public isConnected(): boolean {
    return this.eventSource?.readyState === EventSource.OPEN;
  }

  /**
   * Get subscriber count for debugging
   */
  public getSubscriberCount(): number {
    let total = 0;
    this.subscribers.forEach(handlers => {
      total += handlers.size;
    });
    return total;
  }

  /**
   * Force reconnect (useful for testing or recovery)
   */
  public forceReconnect() {
    console.log('[SSE Service] Force reconnect requested');
    this.disconnect();
    this.reconnectAttempts = 0;
    this.connect();
  }
}

// Export singleton instance
export const sseService = new SSEService();

// Export for debugging in browser console
if (typeof window !== 'undefined') {
  (window as any).sseService = sseService;
}
