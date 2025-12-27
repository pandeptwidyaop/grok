import axios from 'axios';
import type {
  TunnelStatus,
  RequestsResponse,
  RequestLog,
  MetricsSnapshot,
  DashboardStats,
} from '@/types';

// Create axios instance with base configuration
const api = axios.create({
  baseURL: import.meta.env.DEV ? 'http://localhost:4041/api' : '/api',
  timeout: 10000,
  headers: {
    'Content-Type': 'application/json',
  },
});

// Tunnel API
export const tunnelAPI = {
  /**
   * Get current tunnel status
   */
  getStatus: async (): Promise<TunnelStatus> => {
    const response = await api.get<TunnelStatus>('/status');
    return response.data;
  },
};

// Requests API
export const requestsAPI = {
  /**
   * Get list of recent requests
   */
  list: async (params?: { limit?: number; offset?: number }): Promise<RequestsResponse> => {
    const response = await api.get<RequestsResponse>('/requests', { params });
    return response.data;
  },

  /**
   * Get detailed information about a specific request
   */
  get: async (id: string): Promise<RequestLog> => {
    const response = await api.get<RequestLog>(`/requests/${id}`);
    return response.data;
  },
};

// Metrics API
export const metricsAPI = {
  /**
   * Get current performance metrics
   */
  get: async (): Promise<MetricsSnapshot> => {
    const response = await api.get<MetricsSnapshot>('/metrics');
    return response.data;
  },
};

// Stats API
export const statsAPI = {
  /**
   * Get dashboard statistics
   */
  get: async (): Promise<DashboardStats> => {
    const response = await api.get<DashboardStats>('/stats');
    return response.data;
  },
};

// Admin API
export const adminAPI = {
  /**
   * Clear all stored requests
   */
  clear: async (): Promise<void> => {
    await api.post('/clear');
  },
};

// Health check
export const healthAPI = {
  /**
   * Check if dashboard server is healthy
   */
  check: async (): Promise<boolean> => {
    try {
      const response = await api.get('/health', { baseURL: '/health' });
      return response.status === 200;
    } catch {
      return false;
    }
  },
};

// Export combined API
export default {
  tunnel: tunnelAPI,
  requests: requestsAPI,
  metrics: metricsAPI,
  stats: statsAPI,
  admin: adminAPI,
  health: healthAPI,
};
