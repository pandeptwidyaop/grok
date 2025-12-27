// TunnelStatus represents the current tunnel connection status
export interface TunnelStatus {
  connected: boolean;
  tunnel_id?: string;
  public_url?: string;
  local_addr?: string;
  protocol?: string;
  uptime_seconds: number;
  connected_at?: string;
}

// RequestLog represents a single HTTP/TCP request
export interface RequestLog {
  id: string;
  timestamp: string;
  method: string;
  path: string;
  protocol: string;
  remote_addr: string;
  status_code: number;
  bytes_in: number;
  bytes_out: number;
  duration_ms: number;
  completed: boolean;
  error?: string;

  // Headers
  request_headers?: Record<string, string[]>;
  response_headers?: Record<string, string[]>;

  // Body (truncated)
  request_body?: string;
  response_body?: string;
}

// MetricsSnapshot represents performance metrics at a point in time
export interface MetricsSnapshot {
  total_requests: number;
  active_requests: number;
  bytes_in: number;
  bytes_out: number;
  avg_latency_ms: number;
  p50_latency_ms: number;
  p95_latency_ms: number;
  p99_latency_ms: number;
  request_rate: number; // requests per second
  error_count: number;
  timestamp: string;
}

// SSEEvent represents a server-sent event
export interface SSEEvent {
  type: 'request_started' | 'request_completed' | 'connection_established' | 'connection_lost' | 'metrics_update' | 'connected';
  data: RequestStartedData | RequestCompletedData | TunnelStatus | MetricsSnapshot | ConnectedData;
}

export interface RequestStartedData {
  request_id: string;
  method: string;
  path: string;
  remote_addr: string;
  protocol: string;
  headers?: Record<string, string>;
}

export interface RequestCompletedData {
  request_id: string;
  status_code: number;
  bytes_in: number;
  bytes_out: number;
  duration: number;
  error?: string;
}

export interface ConnectedData {
  message: string;
}

// API response types
export interface RequestsResponse {
  requests: RequestLog[];
  total: number;
}

export interface DashboardStats {
  request_store: {
    size: number;
    max_size: number;
  };
  sse_clients: number;
  event_queue: number;
  uptime_seconds: number;
}
