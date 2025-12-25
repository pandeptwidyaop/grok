import axios from 'axios';

// API Base URL - will be same origin in production
const API_BASE = import.meta.env.VITE_API_URL || '/api';

const apiClient = axios.create({
  baseURL: API_BASE,
  headers: {
    'Content-Type': 'application/json',
  },
});

// Types
export interface AuthToken {
  id: string;
  name: string;
  token?: string; // Only returned on creation
  scopes: string[];
  last_used_at?: string;
  expires_at?: string;
  is_active: boolean;
  created_at: string;
}

export interface Tunnel {
  id: string;
  tunnel_type: string;
  subdomain: string;
  remote_port?: number;
  local_addr: string;
  public_url: string;
  status: string;
  bytes_in: number;
  bytes_out: number;
  requests_count: number;
  connected_at: string;
  last_activity_at: string;
}

export interface RequestLog {
  id: string;
  tunnel_id: string;
  method: string;
  path: string;
  status_code: number;
  duration_ms: number;
  bytes_in: number;
  bytes_out: number;
  client_ip: string;
  created_at: string;
}

export interface Stats {
  total_tunnels: number;
  active_tunnels: number;
  total_requests: number;
  total_bytes_in: number;
  total_bytes_out: number;
}

// API Methods
export const api = {
  // Auth Tokens
  tokens: {
    list: () => apiClient.get<AuthToken[]>('/tokens'),
    create: (name: string, scopes: string[]) =>
      apiClient.post<AuthToken>('/tokens', { name, scopes }),
    delete: (id: string) => apiClient.delete(`/tokens/${id}`),
    toggle: (id: string) => apiClient.patch(`/tokens/${id}/toggle`),
  },

  // Tunnels
  tunnels: {
    list: () => apiClient.get<Tunnel[]>('/tunnels'),
    get: (id: string) => apiClient.get<Tunnel>(`/tunnels/${id}`),
    logs: (id: string, limit = 100) =>
      apiClient.get<RequestLog[]>(`/tunnels/${id}/logs`, {
        params: { limit },
      }),
  },

  // Stats
  stats: {
    get: () => apiClient.get<Stats>('/stats'),
  },
};

export default apiClient;
