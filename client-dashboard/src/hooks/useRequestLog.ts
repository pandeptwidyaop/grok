import { useState, useEffect, useCallback } from 'react';
import type { RequestLog, RequestStartedData, RequestCompletedData } from '@/types';
import { useSSE } from './useSSE';
import api from '@/services/api';

interface UseRequestLogReturn {
  requests: RequestLog[];
  loading: boolean;
  error: Error | null;
  autoScroll: boolean;
  setAutoScroll: (enabled: boolean) => void;
  filterMethod: string;
  setFilterMethod: (method: string) => void;
  filterStatus: string;
  setFilterStatus: (status: string) => void;
  searchQuery: string;
  setSearchQuery: (query: string) => void;
  clear: () => void;
  refresh: () => Promise<void>;
}

const MAX_REQUESTS = 500; // Keep last 500 requests in memory

/**
 * React hook for managing request log state with real-time updates
 */
export function useRequestLog(): UseRequestLogReturn {
  const [requests, setRequests] = useState<RequestLog[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<Error | null>(null);
  const [autoScroll, setAutoScroll] = useState(true);
  const [filterMethod, setFilterMethod] = useState('ALL');
  const [filterStatus, setFilterStatus] = useState('ALL');
  const [searchQuery, setSearchQuery] = useState('');

  // Load initial requests
  const loadRequests = useCallback(async () => {
    try {
      setLoading(true);
      setError(null);

      const response = await api.requests.list({ limit: 100 });
      setRequests(response.requests || []);
    } catch (err) {
      console.error('Failed to load requests:', err);
      setError(err as Error);
    } finally {
      setLoading(false);
    }
  }, []);

  // Initial load
  useEffect(() => {
    loadRequests();
  }, [loadRequests]);

  // Handle request started events from SSE
  useSSE<RequestStartedData>(
    'request_started',
    useCallback((data: RequestStartedData) => {
      setRequests((prev) => {
        // Create new request with start data
        const newRequest: RequestLog = {
          id: data.request_id,
          timestamp: new Date().toISOString(),
          method: data.method,
          path: data.path,
          protocol: data.protocol,
          remote_addr: data.remote_addr,
          status_code: 0, // Not yet completed
          bytes_in: 0,
          bytes_out: 0,
          duration_ms: 0,
          completed: false,
          request_headers: data.headers ? { ...data.headers } as any : undefined,
        };

        // Add new request at the beginning
        const updated = [newRequest, ...prev];

        // Limit to MAX_REQUESTS
        if (updated.length > MAX_REQUESTS) {
          return updated.slice(0, MAX_REQUESTS);
        }

        return updated;
      });
    }, [])
  );

  // Handle request completion events from SSE
  useSSE<RequestCompletedData>(
    'request_completed',
    useCallback((data: RequestCompletedData) => {
      setRequests((prev) => {
        const existingIndex = prev.findIndex(r => r.id === data.request_id);

        if (existingIndex >= 0) {
          // Update existing request with completion data
          const updated = [...prev];
          updated[existingIndex] = {
            ...updated[existingIndex],
            status_code: data.status_code,
            bytes_in: data.bytes_in,
            bytes_out: data.bytes_out,
            duration_ms: data.duration / 1000000, // Convert nanoseconds to milliseconds
            completed: true,
            error: data.error,
          };
          return updated;
        } else {
          // Request started event might have been missed, create incomplete record
          console.warn('[useRequestLog] Received completion for unknown request:', data.request_id);
          return prev;
        }
      });
    }, [])
  );

  // Clear all requests
  const clear = useCallback(() => {
    setRequests([]);
  }, []);

  // Refresh requests
  const refresh = useCallback(async () => {
    await loadRequests();
  }, [loadRequests]);

  return {
    requests,
    loading,
    error,
    autoScroll,
    setAutoScroll,
    filterMethod,
    setFilterMethod,
    filterStatus,
    setFilterStatus,
    searchQuery,
    setSearchQuery,
    clear,
    refresh,
  };
}

export default useRequestLog;
