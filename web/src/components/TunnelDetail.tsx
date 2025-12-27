import { useParams, useNavigate } from 'react-router-dom';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import {
  Box,
  Card,
  CardContent,
  Typography,
  IconButton,
  Chip,
  Avatar,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  CircularProgress,
  Paper,
  TextField,
  Pagination,
  Divider,
  useMediaQuery,
  useTheme,
} from '@mui/material';
import {
  ArrowLeft,
  Activity,
  Download,
  Upload,
  Clock,
  Server,
  Link as LinkIcon,
  Copy,
  Check,
  Circle,
} from 'lucide-react';
import { api } from '@/lib/api';
import { useState } from 'react';
import { toast } from 'sonner';
import { useTunnelEvents } from '@/hooks/useSSE';
import { formatRelativeTime, formatBytes, formatDuration } from '@/lib/utils';

function TunnelDetail() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const [copiedUrl, setCopiedUrl] = useState(false);
  const [page, setPage] = useState(1);
  const [pathFilter, setPathFilter] = useState('');
  const queryClient = useQueryClient();
  const theme = useTheme();
  const isMobile = useMediaQuery(theme.breakpoints.down('md'));

  // Subscribe to real-time tunnel events via SSE
  useTunnelEvents((event) => {
    // If this tunnel's data changed, refetch
    if (event.data?.tunnel_id === id) {
      queryClient.refetchQueries({ queryKey: ['tunnel', id] });
      queryClient.refetchQueries({ queryKey: ['tunnel-logs', id] });
    }
  });

  // Fetch tunnel details
  const { data: tunnel, isLoading: tunnelLoading } = useQuery({
    queryKey: ['tunnel', id],
    queryFn: async () => {
      if (!id) throw new Error('Tunnel ID is required');
      const response = await api.tunnels.get(id);
      return response.data;
    },
    enabled: !!id,
  });

  // Fetch request logs (only for HTTP/HTTPS tunnels, not TCP)
  const { data: logsData, isLoading: logsLoading } = useQuery({
    queryKey: ['tunnel-logs', id, page, pathFilter],
    queryFn: async () => {
      if (!id) throw new Error('Tunnel ID is required');
      const response = await api.tunnels.logs(id, {
        page,
        limit: 20,
        ...(pathFilter && { path: pathFilter }),
      });
      return response.data;
    },
    enabled: !!id && tunnel?.tunnel_type?.toLowerCase() !== 'tcp',
  });

  const logs = logsData?.logs || [];
  const totalPages = logsData?.total_pages || 0;

  const getUptime = (connectedAt: string) => {
    const connected = new Date(connectedAt);
    const now = new Date();
    const diff = now.getTime() - connected.getTime();

    const days = Math.floor(diff / (1000 * 60 * 60 * 24));
    const hours = Math.floor((diff % (1000 * 60 * 60 * 24)) / (1000 * 60 * 60));
    const minutes = Math.floor((diff % (1000 * 60 * 60)) / (1000 * 60));

    if (days > 0) return `${days}d ${hours}h ${minutes}m`;
    if (hours > 0) return `${hours}h ${minutes}m`;
    return `${minutes}m`;
  };

  const handleCopyUrl = (url: string) => {
    navigator.clipboard.writeText(url);
    setCopiedUrl(true);
    toast.success('URL copied to clipboard');
    setTimeout(() => setCopiedUrl(false), 2000);
  };

  const getStatusColor = (code: number) => {
    if (code >= 200 && code < 300) return '#4caf50';
    if (code >= 300 && code < 400) return '#2196f3';
    if (code >= 400 && code < 500) return '#ff9800';
    return '#f44336';
  };

  if (tunnelLoading) {
    return (
      <Box sx={{ display: 'flex', justifyContent: 'center', alignItems: 'center', minHeight: '400px' }}>
        <CircularProgress />
      </Box>
    );
  }

  if (!tunnel) {
    return (
      <Box>
        <Typography variant="h6" color="error">
          Tunnel not found
        </Typography>
      </Box>
    );
  }

  const statsCards = [
    {
      title: 'Total Requests',
      value: (tunnel.requests_count ?? 0).toLocaleString(),
      subtitle: 'All time requests',
      icon: Activity,
      color: '#667eea',
    },
    {
      title: 'Data Received',
      value: formatBytes(tunnel.bytes_in || 0),
      subtitle: 'Inbound traffic',
      icon: Download,
      color: '#4caf50',
    },
    {
      title: 'Data Sent',
      value: formatBytes(tunnel.bytes_out || 0),
      subtitle: 'Outbound traffic',
      icon: Upload,
      color: '#2196f3',
    },
    {
      title: 'Uptime',
      value: tunnel.status === 'active' ? getUptime(tunnel.connected_at) : 'Offline',
      subtitle: tunnel.status === 'active' ? 'Connected' : 'Disconnected',
      icon: Clock,
      color: tunnel.status === 'active' ? '#667eea' : '#9e9e9e',
    },
  ];

  return (
    <Box>
      {/* Header */}
      <Box sx={{ display: 'flex', alignItems: 'center', gap: { xs: 1, md: 2 }, mb: 4 }}>
        <IconButton onClick={() => navigate('/tunnels')}>
          <ArrowLeft size={20} />
        </IconButton>
        <Box sx={{ flex: 1, minWidth: 0 }}>
          <Box sx={{ display: 'flex', alignItems: 'center', gap: { xs: 1, md: 2 }, mb: 0.5, flexWrap: 'wrap' }}>
            <Typography
              variant={isMobile ? 'h5' : 'h4'}
              sx={{
                fontWeight: 700,
                color: '#667eea',
                wordBreak: 'break-word',
              }}
            >
              {tunnel.saved_name || tunnel.subdomain}
            </Typography>
            <Chip
              icon={
                <Circle
                  size={8}
                  fill={tunnel.status === 'active' ? '#4caf50' : '#9e9e9e'}
                  color={tunnel.status === 'active' ? '#4caf50' : '#9e9e9e'}
                />
              }
              label={tunnel.status === 'active' ? 'online' : 'offline'}
              variant="outlined"
              size="small"
              sx={{
                fontWeight: 500,
                borderColor: tunnel.status === 'active' ? '#4caf50' : '#9e9e9e',
                color: tunnel.status === 'active' ? '#4caf50' : '#9e9e9e',
                '& .MuiChip-icon': {
                  marginLeft: '8px',
                }
              }}
            />
          </Box>
          <Typography variant="body2" color="text.secondary">
            {tunnel.tunnel_type?.toUpperCase()} Tunnel
          </Typography>
        </Box>
      </Box>

      {/* Stats Cards */}
      <Box
        sx={{
          display: 'grid',
          gridTemplateColumns: {
            xs: '1fr',
            sm: 'repeat(2, 1fr)',
            lg: 'repeat(4, 1fr)',
          },
          gap: 3,
          mb: 4,
        }}
      >
        {statsCards.map((card, index) => {
          const IconComponent = card.icon;
          return (
            <Card
              key={index}
              elevation={2}
              sx={{
                transition: 'box-shadow 0.3s',
                '&:hover': {
                  boxShadow: 6,
                },
              }}
            >
              <CardContent>
                <Box
                  sx={{
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'space-between',
                    mb: 2,
                  }}
                >
                  <Typography variant="body2" color="text.secondary" fontWeight={500}>
                    {card.title}
                  </Typography>
                  <Avatar
                    sx={{
                      width: 40,
                      height: 40,
                      bgcolor: card.color,
                    }}
                  >
                    <IconComponent size={20} color="white" />
                  </Avatar>
                </Box>
                <Typography variant="h4" sx={{ fontWeight: 700, color: card.color, mb: 0.5 }}>
                  {card.value}
                </Typography>
                <Typography variant="caption" color="text.secondary">
                  {card.subtitle}
                </Typography>
              </CardContent>
            </Card>
          );
        })}
      </Box>

      {/* Connection Details */}
      <Card sx={{ mb: 4 }}>
        <CardContent>
          <Typography variant="h6" gutterBottom sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
            <Server size={20} />
            Connection Details
          </Typography>
          <Box sx={{ display: 'grid', gap: 2, mt: 3 }}>
            <Box>
              <Typography variant="caption" color="text.secondary">
                Public URL
              </Typography>
              <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mt: 0.5 }}>
                <Typography
                  variant="body1"
                  sx={{
                    fontFamily: 'monospace',
                    color: 'primary.main',
                    fontWeight: 500,
                  }}
                >
                  {tunnel.public_url}
                </Typography>
                <IconButton size="small" onClick={() => handleCopyUrl(tunnel.public_url)}>
                  {copiedUrl ? (
                    <Check size={16} style={{ color: '#4caf50' }} />
                  ) : (
                    <Copy size={16} />
                  )}
                </IconButton>
              </Box>
            </Box>

            <Box>
              <Typography variant="caption" color="text.secondary">
                Local Address
              </Typography>
              <Typography variant="body1" sx={{ fontFamily: 'monospace', mt: 0.5 }}>
                {tunnel.local_addr}
              </Typography>
            </Box>

            <Box sx={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(200px, 1fr))', gap: 2 }}>
              <Box>
                <Typography variant="caption" color="text.secondary">
                  Subdomain
                </Typography>
                <Typography variant="body1" sx={{ mt: 0.5 }}>
                  {tunnel.subdomain}
                </Typography>
              </Box>

              <Box>
                <Typography variant="caption" color="text.secondary">
                  Protocol
                </Typography>
                <Typography variant="body1" sx={{ mt: 0.5 }}>
                  {tunnel.tunnel_type?.toUpperCase()}
                </Typography>
              </Box>

              {tunnel.remote_port && (
                <Box>
                  <Typography variant="caption" color="text.secondary">
                    Remote Port
                  </Typography>
                  <Typography variant="body1" sx={{ mt: 0.5 }}>
                    {tunnel.remote_port}
                  </Typography>
                </Box>
              )}

              <Box>
                <Typography variant="caption" color="text.secondary">
                  Connected At
                </Typography>
                <Typography variant="body1" sx={{ mt: 0.5 }}>
                  {formatRelativeTime(tunnel.connected_at)}
                </Typography>
              </Box>

              {tunnel.last_activity_at && (
                <Box>
                  <Typography variant="caption" color="text.secondary">
                    Last Activity
                  </Typography>
                  <Typography variant="body1" sx={{ mt: 0.5 }}>
                    {formatRelativeTime(tunnel.last_activity_at)}
                  </Typography>
                </Box>
              )}
            </Box>
          </Box>
        </CardContent>
      </Card>

      {/* HTTP Request Logs (only for HTTP/HTTPS tunnels, not TCP) */}
      {tunnel.tunnel_type?.toLowerCase() !== 'tcp' && (
        <Card>
          <CardContent>
            <Typography variant="h6" gutterBottom sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
              <LinkIcon size={20} />
              HTTP Request Logs
            </Typography>

            {/* Filter Controls */}
            <Box sx={{ mt: 2, mb: 3 }}>
              <TextField
                size="small"
                placeholder="Filter by path (e.g., /api/users)"
                value={pathFilter}
                onChange={(e) => {
                  setPathFilter(e.target.value);
                  setPage(1); // Reset to first page when filtering
                }}
                sx={{ width: { xs: '100%', sm: '400px' } }}
              />
            </Box>

            {logsLoading ? (
              <Box sx={{ textAlign: 'center', py: 8 }}>
                <CircularProgress />
              </Box>
            ) : !logs || logs.length === 0 ? (
              <Box sx={{ textAlign: 'center', py: 8 }}>
                <Activity size={48} style={{ color: '#9e9e9e', opacity: 0.5, margin: '0 auto 8px' }} />
                <Typography variant="body2" color="text.secondary">
                  No requests yet
                </Typography>
                <Typography variant="caption" color="text.secondary">
                  Send a request to {tunnel.public_url} to see logs here
                </Typography>
              </Box>
            ) : isMobile ? (
              /* Mobile Card View */
              <Box sx={{ display: 'flex', flexDirection: 'column', gap: 2, mt: 2 }}>
                {logs.map((log) => (
                  <Card key={log.id} variant="outlined">
                    <CardContent sx={{ p: 2 }}>
                      {/* Header: Method, Status, Time */}
                      <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', mb: 2 }}>
                        <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                          <Chip
                            label={log.method}
                            size="small"
                            sx={{
                              fontFamily: 'monospace',
                              fontWeight: 600,
                              fontSize: '0.75rem',
                            }}
                          />
                          <Chip
                            label={log.status_code}
                            size="small"
                            sx={{
                              bgcolor: getStatusColor(log.status_code),
                              color: 'white',
                              fontWeight: 600,
                            }}
                          />
                        </Box>
                        <Typography variant="caption" color="text.secondary">
                          {formatRelativeTime(log.created_at)}
                        </Typography>
                      </Box>

                      {/* Path */}
                      <Box sx={{ mb: 2 }}>
                        <Typography variant="caption" color="text.secondary" sx={{ display: 'block', mb: 0.5 }}>
                          Path
                        </Typography>
                        <Typography
                          variant="body2"
                          sx={{
                            fontFamily: 'monospace',
                            fontSize: '0.875rem',
                            wordBreak: 'break-all',
                          }}
                        >
                          {log.path}
                        </Typography>
                      </Box>

                      <Divider sx={{ my: 1.5 }} />

                      {/* Stats */}
                      <Box sx={{ display: 'grid', gridTemplateColumns: 'repeat(2, 1fr)', gap: 2 }}>
                        <Box>
                          <Typography variant="caption" color="text.secondary" sx={{ display: 'block', mb: 0.5 }}>
                            Duration
                          </Typography>
                          <Typography variant="body2" sx={{ fontWeight: 500 }}>
                            {formatDuration(log.duration_ms)}
                          </Typography>
                        </Box>
                        <Box>
                          <Typography variant="caption" color="text.secondary" sx={{ display: 'block', mb: 0.5 }}>
                            Size
                          </Typography>
                          <Typography variant="body2" sx={{ fontWeight: 500 }}>
                            {formatBytes(log.bytes_in + log.bytes_out)}
                          </Typography>
                        </Box>
                      </Box>
                    </CardContent>
                  </Card>
                ))}
              </Box>
            ) : (
              /* Desktop Table View */
              <TableContainer component={Paper} variant="outlined" sx={{ mt: 2 }}>
                <Table size="small">
                  <TableHead>
                    <TableRow>
                      <TableCell sx={{ fontWeight: 600 }}>Timestamp</TableCell>
                      <TableCell sx={{ fontWeight: 600 }}>Method</TableCell>
                      <TableCell sx={{ fontWeight: 600 }}>Path</TableCell>
                      <TableCell align="center" sx={{ fontWeight: 600 }}>
                        Status
                      </TableCell>
                      <TableCell align="right" sx={{ fontWeight: 600 }}>
                        Duration
                      </TableCell>
                      <TableCell align="right" sx={{ fontWeight: 600 }}>
                        Size
                      </TableCell>
                    </TableRow>
                  </TableHead>
                  <TableBody>
                    {logs.map((log) => (
                      <TableRow key={log.id} hover>
                        <TableCell>
                          <Typography variant="caption" color="text.secondary">
                            {formatRelativeTime(log.created_at)}
                          </Typography>
                        </TableCell>
                        <TableCell>
                          <Chip
                            label={log.method}
                            size="small"
                            sx={{
                              fontFamily: 'monospace',
                              fontWeight: 600,
                              fontSize: '0.75rem',
                            }}
                          />
                        </TableCell>
                        <TableCell>
                          <Typography
                            variant="body2"
                            sx={{
                              fontFamily: 'monospace',
                              fontSize: '0.875rem',
                            }}
                          >
                            {log.path}
                          </Typography>
                        </TableCell>
                        <TableCell align="center">
                          <Chip
                            label={log.status_code}
                            size="small"
                            sx={{
                              bgcolor: getStatusColor(log.status_code),
                              color: 'white',
                              fontWeight: 600,
                            }}
                          />
                        </TableCell>
                        <TableCell align="right">
                          <Typography variant="body2">
                            {formatDuration(log.duration_ms)}
                          </Typography>
                        </TableCell>
                        <TableCell align="right">
                          <Typography variant="body2" color="text.secondary">
                            {formatBytes(log.bytes_in + log.bytes_out)}
                          </Typography>
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </TableContainer>
            )}

            {/* Pagination */}
            {!logsLoading && logs && logs.length > 0 && totalPages > 1 && (
              <Box sx={{ display: 'flex', justifyContent: 'center', mt: 3 }}>
                <Pagination
                  count={totalPages}
                  page={page}
                  onChange={(_, newPage) => setPage(newPage)}
                  color="primary"
                  showFirstButton
                  showLastButton
                />
              </Box>
            )}
          </CardContent>
        </Card>
      )}
    </Box>
  );
}

export default TunnelDetail;
