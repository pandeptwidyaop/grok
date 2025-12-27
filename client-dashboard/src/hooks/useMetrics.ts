import { useState, useEffect, useCallback } from 'react';
import type { MetricsSnapshot } from '@/types';
import { useSSE } from './useSSE';
import api from '@/services/api';

interface UseMetricsReturn {
  metrics: MetricsSnapshot | null;
  history: MetricsSnapshot[];
  loading: boolean;
  error: Error | null;
  refresh: () => Promise<void>;
}

const MAX_HISTORY = 100; // Keep last 100 metric snapshots

/**
 * React hook for managing metrics with real-time updates
 */
export function useMetrics(): UseMetricsReturn {
  const [metrics, setMetrics] = useState<MetricsSnapshot | null>(null);
  const [history, setHistory] = useState<MetricsSnapshot[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<Error | null>(null);

  // Load initial metrics
  const loadMetrics = useCallback(async () => {
    try {
      setLoading(true);
      setError(null);

      const data = await api.metrics.get();
      setMetrics(data);

      // Add to history
      setHistory((prev) => {
        const updated = [data, ...prev];
        if (updated.length > MAX_HISTORY) {
          return updated.slice(0, MAX_HISTORY);
        }
        return updated;
      });
    } catch (err) {
      console.error('Failed to load metrics:', err);
      setError(err as Error);
    } finally {
      setLoading(false);
    }
  }, []);

  // Initial load
  useEffect(() => {
    loadMetrics();
  }, [loadMetrics]);

  // Handle metrics update events from SSE
  useSSE<MetricsSnapshot>(
    'metrics_update',
    useCallback((data: MetricsSnapshot) => {
      setMetrics(data);

      // Add to history
      setHistory((prev) => {
        const updated = [data, ...prev];
        if (updated.length > MAX_HISTORY) {
          return updated.slice(0, MAX_HISTORY);
        }
        return updated;
      });
    }, [])
  );

  // Fallback polling if SSE is not available
  useEffect(() => {
    const interval = setInterval(() => {
      // Only poll if we haven't received metrics in the last 10 seconds
      if (metrics && new Date().getTime() - new Date(metrics.timestamp).getTime() > 10000) {
        loadMetrics();
      }
    }, 5000);

    return () => clearInterval(interval);
  }, [metrics, loadMetrics]);

  return {
    metrics,
    history,
    loading,
    error,
    refresh: loadMetrics,
  };
}

export default useMetrics;
