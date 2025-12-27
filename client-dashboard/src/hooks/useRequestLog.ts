import { useState, useEffect, useCallback } from 'react';
import type { RequestLog, RequestCompletedData } from '@/types';
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

  // Handle new request completion events from SSE
  useSSE<RequestCompletedData>(
    'request_completed',
    useCallback((data: RequestCompletedData) => {
      setRequests((prev) => {
        // Find existing request or create new one
        const existingIndex = prev.findIndex(r => r.id === data.request_id);

        const newRequest: RequestLog = {
          id: data.request_id,
          timestamp: new Date().toISOString(),
          method: 'GET', // Will be updated from backend
          path: '/',
          protocol: 'http',
          remote_addr: '',
          status_code: data.status_code,
          bytes_in: data.bytes_in,
          bytes_out: data.bytes_out,
          duration_ms: data.duration / 1000000, // Convert nanoseconds to milliseconds
          completed: true,
          error: data.error,
        };

        let updated: RequestLog[];
        if (existingIndex >= 0) {
          // Update existing request
          updated = [...prev];
          updated[existingIndex] = { ...updated[existingIndex], ...newRequest };
        } else {
          // Add new request at the beginning
          updated = [newRequest, ...prev];

          // Limit to MAX_REQUESTS
          if (updated.length > MAX_REQUESTS) {
            updated = updated.slice(0, MAX_REQUESTS);
          }
        }

        return updated;
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
