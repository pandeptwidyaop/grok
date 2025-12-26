import { useEffect, useRef, useState } from 'react';
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
  const { token } = useAuth();

  useEffect(() => {
    if (!token) {
      setIsConnected(false);
      return;
    }

    // Build full URL with token
    // In development, use relative path for Vite proxy
    // In production, VITE_API_URL will be the actual API URL
    const baseURL = import.meta.env.VITE_API_URL || '';
    const fullURL = `${baseURL}${url}?token=${encodeURIComponent(token)}`;

    // Create EventSource with auth token
    const eventSource = new EventSource(fullURL, {
      withCredentials: true,
    });

    eventSourceRef.current = eventSource;

    eventSource.onopen = () => {
      setIsConnected(true);
      setError(null);
    };

    eventSource.onerror = (err) => {
      console.error('[SSE] Connection error:', err);
      setIsConnected(false);
      setError(new Error('SSE connection failed'));
    };

    eventSource.onmessage = (event) => {
      try {
        const data: SSEEvent = JSON.parse(event.data);
        onMessage?.(data);
      } catch (err) {
        console.error('[SSE] Failed to parse event data:', err);
      }
    };

    return () => {
      eventSource.close();
      eventSourceRef.current = null;
    };
  }, [url, token, onMessage]);

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
