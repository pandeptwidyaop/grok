import { useEffect, useState, useCallback } from 'react';
import { sseService } from '@/services/sseService';
import type { SSEEvent, SSEEventHandler } from '@/services/sseService';

/**
 * Hook to subscribe to SSE events using the global SSE service
 * No longer creates/destroys EventSource on mount/unmount
 * Instead, subscribes/unsubscribes to the global connection
 *
 * @param eventType - Event type to listen for (e.g., 'tunnel_created', '*' for all)
 * @param onMessage - Callback when event is received
 * @returns Connection status
 */
export function useSSE(eventType: string = '*', onMessage?: SSEEventHandler) {
  const [isConnected, setIsConnected] = useState(false);
  const [error, setError] = useState<Error | null>(null);

  // Memoize the message handler to prevent unnecessary re-subscriptions
  const handleMessage = useCallback((event: SSEEvent) => {
    onMessage?.(event);
  }, [onMessage]);

  useEffect(() => {
    // Update connection status when connected/disconnected
    const handleConnectionChange = (event: SSEEvent) => {
      if (event.type === '_connected') {
        setIsConnected(true);
        setError(null);
      } else if (event.type === '_disconnected') {
        setIsConnected(false);
      }
    };

    // Subscribe to connection status changes
    const unsubscribeStatus = sseService.subscribe('_connected', handleConnectionChange);
    const unsubscribeDisconnect = sseService.subscribe('_disconnected', handleConnectionChange);

    // Subscribe to the specific event type or all events
    const unsubscribe = sseService.subscribe(eventType, handleMessage);

    // Set initial connection status
    setIsConnected(sseService.isConnected());

    // Cleanup: unsubscribe when component unmounts
    // IMPORTANT: This does NOT close the EventSource, just removes this component's listeners
    return () => {
      unsubscribe();
      unsubscribeStatus();
      unsubscribeDisconnect();
    };
  }, [eventType, handleMessage]);

  return { isConnected, error };
}

/**
 * Hook specifically for tunnel events
 * Listens to all tunnel-related events (created, updated, deleted)
 */
export function useTunnelEvents(onTunnelEvent: SSEEventHandler) {
  // Listen to all events and filter for tunnel events
  const handleEvent = useCallback((event: SSEEvent) => {
    if (event.type.startsWith('tunnel_') || event.type.includes('tunnel')) {
      onTunnelEvent(event);
    }
  }, [onTunnelEvent]);

  return useSSE('*', handleEvent);
}

/**
 * Hook for all real-time events
 * Use this when you need to listen to all event types
 */
export function useAllEvents(onEvent: SSEEventHandler) {
  return useSSE('*', onEvent);
}

/**
 * Hook to listen to a specific event type only
 * More efficient than useAllEvents when you only care about one type
 */
export function useSSEEvent(eventType: string, onEvent: SSEEventHandler) {
  return useSSE(eventType, onEvent);
}

/**
 * Export SSE service for manual control if needed
 */
export { sseService };
export type { SSEEvent, SSEEventHandler };
