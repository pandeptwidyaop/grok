import { useEffect, useState } from 'react';
import { sseService } from '@/services/sseService';

/**
 * React hook for subscribing to SSE events
 * @param eventType - The type of event to listen for
 * @param callback - Callback function when event is received
 */
export function useSSE<T = any>(
  eventType: string,
  callback: (data: T) => void
): void {
  useEffect(() => {
    const unsubscribe = sseService.subscribe(eventType, callback);

    return () => {
      unsubscribe();
    };
  }, [eventType, callback]);
}

/**
 * React hook for SSE connection status
 * @returns Connection status (true if connected, false otherwise)
 */
export function useSSEConnection(): boolean {
  const [isConnected, setIsConnected] = useState(false);

  useEffect(() => {
    const unsubscribe = sseService.onConnectionChange((connected) => {
      setIsConnected(connected);
    });

    return () => {
      unsubscribe();
    };
  }, []);

  return isConnected;
}

export default useSSE;
