import axios from 'axios';

// API Base URL - will be same origin in production
const API_BASE = import.meta.env.VITE_API_URL || '/api';

const apiClient = axios.create({
  baseURL: API_BASE,
  headers: {
    'Content-Type': 'application/json',
  },
});

// Add request interceptor to include auth token
apiClient.interceptors.request.use((config) => {
  const token = localStorage.getItem('auth_token');
  if (token) {
    config.headers.Authorization = `Bearer ${token}`;
  }
  return config;
});

// Add response interceptor to handle auth errors
apiClient.interceptors.response.use(
  (response) => response,
  (error) => {
    if (error.response?.status === 401) {
      // Clear auth and redirect to login
      localStorage.removeItem('auth_token');
      localStorage.removeItem('auth_user');
      window.location.href = '/login';
    }
    return Promise.reject(error);
  }
);

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
  owner_email?: string;
  owner_name?: string;
  organization_name?: string;
}

export interface Tunnel {
  id: string;
  tunnel_type: string;
  subdomain: string;
  remote_port?: number;
  local_addr: string;
  public_url: string;
  status: string;
  saved_name?: string;
  is_persistent: boolean;
  bytes_in: number;
  bytes_out: number;
  requests_count: number;
  connected_at: string;
  disconnected_at?: string;
  last_activity_at: string;
  owner_email?: string;
  owner_name?: string;
  organization_name?: string;
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

export interface PaginatedLogs {
  logs: RequestLog[];
  total: number;
  page: number;
  limit: number;
  total_pages: number;
}

export interface Stats {
  total_tunnels: number;
  active_tunnels: number;
  total_requests: number;
  total_bytes_in: number;
  total_bytes_out: number;
}

export interface Organization {
  id: string;
  name: string;
  subdomain: string;
  full_domain: string;
  description?: string;
  is_active: boolean;
  created_at: string;
  updated_at: string;
}

export interface User {
  id: string;
  email: string;
  name: string;
  role: 'super_admin' | 'org_admin' | 'org_user';
  organization_id?: string;
  is_active: boolean;
  created_at: string;
}

export interface CreateOrganizationRequest {
  name: string;
  subdomain: string;
  description?: string;
}

export interface CreateUserRequest {
  email: string;
  name: string;
  password: string;
  role: 'org_admin' | 'org_user';
}

export interface CreateOrgUserRequest {
  email: string;
  name: string;
  password: string;
  role: 'org_admin' | 'org_user';
}

// Webhook Types
export interface WebhookApp {
  id: string;
  organization_id: string;
  user_id: string;
  name: string;
  description: string;
  is_active: boolean;
  created_at: string;
  updated_at: string;
  webhook_url?: string;
  routes?: WebhookRoute[];
  owner_name?: string;
  owner_email?: string;
  organization_name?: string;
}

export interface WebhookRoute {
  id: string;
  webhook_app_id: string;
  tunnel_id: string;
  is_enabled: boolean;
  priority: number;
  health_status: string;
  failure_count: number;
  last_health_check?: string;
  created_at: string;
  updated_at: string;
  tunnel?: Tunnel;
}

export interface WebhookEvent {
  id: string;
  webhook_app_id: string;
  request_path: string;
  method: string;
  status_code: number;
  duration_ms: number;
  bytes_in: number;
  bytes_out: number;
  client_ip: string;
  routing_status: string;
  tunnel_count: number;
  success_count: number;
  error_message?: string;
  created_at: string;
}

export interface WebhookStats {
  total_events: number;
  success_count: number;
  failure_count: number;
  average_duration_ms: number;
  total_bytes_in: number;
  total_bytes_out: number;
}

export interface CreateWebhookAppRequest {
  name: string;
  description: string;
}

export interface AddWebhookRouteRequest {
  tunnel_id: string;
  priority: number;
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
    logs: (id: string, params?: { page?: number; limit?: number; path?: string }) =>
      apiClient.get<PaginatedLogs>(`/tunnels/${id}/logs`, {
        params: {
          page: params?.page || 1,
          limit: params?.limit || 50,
          ...(params?.path && { path: params.path }),
        },
      }),
    delete: (id: string) => apiClient.delete(`/tunnels/${id}`),
  },

  // Stats
  stats: {
    get: () => apiClient.get<Stats>('/stats'),
  },

  // Config
  config: {
    get: () => apiClient.get<{ domain: string }>('/config'),
  },

  // Organizations (Super Admin only)
  organizations: {
    list: () => apiClient.get<Organization[]>('/organizations'),
    get: (id: string) => apiClient.get<Organization>(`/organizations/${id}`),
    create: (data: CreateOrganizationRequest) =>
      apiClient.post<Organization>('/organizations', data),
    users: {
      list: (orgId: string) => apiClient.get<User[]>(`/organizations/${orgId}/users`),
      create: (orgId: string, data: CreateUserRequest) =>
        apiClient.post<User>(`/organizations/${orgId}/users`, data),
      updateRole: (orgId: string, userId: string, role: string) =>
        apiClient.patch(`/organizations/${orgId}/users/${userId}`, { role }),
      delete: (orgId: string, userId: string) =>
        apiClient.delete(`/organizations/${orgId}/users/${userId}`),
      resetPassword: (orgId: string, userId: string, newPassword: string) =>
        apiClient.post(`/organizations/${orgId}/users/${userId}/reset-password`, { new_password: newPassword }),
    },
    update: (id: string, data: Partial<CreateOrganizationRequest>) =>
      apiClient.patch<Organization>(`/organizations/${id}`, data),
    delete: (id: string) => apiClient.delete(`/organizations/${id}`),
    toggle: (id: string) => apiClient.patch<Organization>(`/organizations/${id}/toggle`),

    // Organization users
    listUsers: (orgId: string) =>
      apiClient.get<User[]>(`/organizations/${orgId}/users`),
    createUser: (orgId: string, data: CreateOrgUserRequest) =>
      apiClient.post<User>(`/organizations/${orgId}/users`, data),
    updateUserRole: (orgId: string, userId: string, role: string) =>
      apiClient.patch(`/organizations/${orgId}/users/${userId}`, { role }),
    deleteUser: (orgId: string, userId: string) =>
      apiClient.delete(`/organizations/${orgId}/users/${userId}`),

    // Organization stats and tunnels
    getStats: (orgId: string) =>
      apiClient.get<Stats>(`/organizations/${orgId}/stats`),
    getTunnels: (orgId: string) =>
      apiClient.get<Tunnel[]>(`/organizations/${orgId}/tunnels`),
  },

  // Webhooks
  webhooks: {
    // Webhook App Management
    listApps: () => apiClient.get<WebhookApp[]>('/webhooks/apps'),
    getApp: (id: string) => apiClient.get<WebhookApp>(`/webhooks/apps/${id}`),
    createApp: (data: CreateWebhookAppRequest) =>
      apiClient.post<WebhookApp>('/webhooks/apps', data),
    updateApp: (id: string, data: Partial<CreateWebhookAppRequest>) =>
      apiClient.patch<WebhookApp>(`/webhooks/apps/${id}`, data),
    deleteApp: (id: string) => apiClient.delete(`/webhooks/apps/${id}`),
    toggleApp: (id: string) => apiClient.patch<WebhookApp>(`/webhooks/apps/${id}/toggle`),

    // Webhook Route Management
    listRoutes: (appId: string) =>
      apiClient.get<WebhookRoute[]>(`/webhooks/apps/${appId}/routes`),
    addRoute: (appId: string, data: AddWebhookRouteRequest) =>
      apiClient.post<WebhookRoute>(`/webhooks/apps/${appId}/routes`, data),
    updateRoute: (appId: string, routeId: string, data: { priority?: number }) =>
      apiClient.patch<WebhookRoute>(`/webhooks/apps/${appId}/routes/${routeId}`, data),
    deleteRoute: (appId: string, routeId: string) =>
      apiClient.delete(`/webhooks/apps/${appId}/routes/${routeId}`),
    toggleRoute: (appId: string, routeId: string) =>
      apiClient.patch<WebhookRoute>(`/webhooks/apps/${appId}/routes/${routeId}/toggle`),

    // Webhook Events & Stats
    getEvents: (appId: string, limit = 100) =>
      apiClient.get<WebhookEvent[]>(`/webhooks/apps/${appId}/events`, { params: { limit } }),
    getStats: (appId: string) =>
      apiClient.get<WebhookStats>(`/webhooks/apps/${appId}/stats`),
  },
};

export default apiClient;
