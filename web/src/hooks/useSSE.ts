import { useEffect, useRef, useState, useCallback } from 'react';
import { useAuth } from '@/contexts/AuthContext';

export interface SSEEvent {
  type: string;
  data: any;
}

export type SSEEventHandler = (event: SSEEvent) => void;

export function useSSE(url: string, onMessage?: SSEEventHandler) {
  const [isConnected, setIsConnected] = useState(false);
  const [error, setError] = useState<Error | null>(null);
  const eventSourceRef = useRef<EventSource | null>(null);
  const reconnectTimeoutRef = useRef<number | null>(null);
  const reconnectAttemptsRef = useRef(0);
  const { isAuthenticated } = useAuth();

  const connect = useCallback(() => {
    if (!isAuthenticated) {
      setIsConnected(false);
      return;
    }

    // Clear any existing connection
    if (eventSourceRef.current) {
      eventSourceRef.current.close();
      eventSourceRef.current = null;
    }

    // Build full URL
    const baseURL = import.meta.env.VITE_API_URL || '';
    const fullURL = `${baseURL}${url}`;

    console.log('[SSE] Connecting to:', fullURL);

    try {
      // Create EventSource with credentials
      const eventSource = new EventSource(fullURL, {
        withCredentials: true,
      });

      eventSourceRef.current = eventSource;

      eventSource.onopen = () => {
        console.log('[SSE] Connection established');
        setIsConnected(true);
        setError(null);
        reconnectAttemptsRef.current = 0; // Reset reconnect counter on success
      };

      eventSource.onerror = (err) => {
        console.error('[SSE] Connection error:', err);
        setIsConnected(false);

        // Close the failed connection
        eventSource.close();

        // Implement exponential backoff for reconnection
        const maxAttempts = 5;
        const baseDelay = 1000; // 1 second
        const maxDelay = 30000; // 30 seconds

        if (reconnectAttemptsRef.current < maxAttempts) {
          const delay = Math.min(
            baseDelay * Math.pow(2, reconnectAttemptsRef.current),
            maxDelay
          );

          console.log(`[SSE] Reconnecting in ${delay}ms (attempt ${reconnectAttemptsRef.current + 1}/${maxAttempts})`);

          reconnectTimeoutRef.current = setTimeout(() => {
            reconnectAttemptsRef.current++;
            connect();
          }, delay);
        } else {
          console.error('[SSE] Max reconnection attempts reached');
          setError(new Error('SSE connection failed after multiple attempts'));
        }
      };

      eventSource.onmessage = (event) => {
        try {
          const data: SSEEvent = JSON.parse(event.data);
          onMessage?.(data);
        } catch (err) {
          console.error('[SSE] Failed to parse event data:', err);
        }
      };
    } catch (err) {
      console.error('[SSE] Failed to create EventSource:', err);
      setError(err as Error);
    }
  }, [url, isAuthenticated, onMessage]);

  useEffect(() => {
    connect();

    return () => {
      // Cleanup on unmount
      if (reconnectTimeoutRef.current) {
        clearTimeout(reconnectTimeoutRef.current);
      }
      if (eventSourceRef.current) {
        eventSourceRef.current.close();
        eventSourceRef.current = null;
      }
    };
  }, [connect]);

  return { isConnected, error };
}

// Hook specifically for tunnel events
export function useTunnelEvents(onTunnelEvent: SSEEventHandler) {
  return useSSE('/api/sse', onTunnelEvent);
}

// Hook for all real-time events (tunnels, webhooks, etc.)
// Use this when you need to listen to all event types
export function useAllEvents(onEvent: SSEEventHandler) {
  return useSSE('/api/sse', onEvent);
}
