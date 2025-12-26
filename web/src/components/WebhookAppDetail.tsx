import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import {
  Box,
  Card,
  CardContent,
  Button,
  IconButton,
  Typography,
  Tabs,
  Tab,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
  TextField,
  Select,
  MenuItem,
  FormControl,
  InputLabel,
  Chip,
  CircularProgress,
  Paper,
  FormHelperText,
} from '@mui/material';
import {
  ArrowLeft,
  Plus,
  Trash2,
  ToggleLeft,
  ToggleRight,
  Activity,
  TrendingUp,
  Clock,
  CheckCircle2,
  XCircle,
  AlertCircle,
} from 'lucide-react';
import { toast } from 'sonner';
import { api } from '@/lib/api';
import type { WebhookApp, WebhookRoute, WebhookEvent, Tunnel } from '@/lib/api';
import { WebhookNetworkDiagram } from './WebhookNetworkDiagram';

interface WebhookAppDetailProps {
  app: WebhookApp;
  onBack: () => void;
}

export function WebhookAppDetail({ app, onBack }: WebhookAppDetailProps) {
  const [addRouteDialogOpen, setAddRouteDialogOpen] = useState(false);
  const [selectedTunnelId, setSelectedTunnelId] = useState('');
  const [routePriority, setRoutePriority] = useState('100');
  const [activeTab, setActiveTab] = useState(0);
  const queryClient = useQueryClient();

  // Fetch domain config for webhook URL
  const { data: config } = useQuery({
    queryKey: ['config'],
    queryFn: async () => {
      const response = await api.config.get();
      return response.data;
    },
  });

  // Fetch routes for this app
  const { data: routes, isLoading: routesLoading } = useQuery({
    queryKey: ['webhook-routes', app.id],
    queryFn: async () => {
      const response = await api.webhooks.listRoutes(app.id);
      return response.data;
    },
    refetchInterval: 5000,
  });

  // Fetch available tunnels
  const { data: tunnels } = useQuery({
    queryKey: ['tunnels'],
    queryFn: async () => {
      const response = await api.tunnels.list();
      return response.data;
    },
    refetchInterval: 5000,
  });

  // Fetch webhook events
  const { data: events } = useQuery({
    queryKey: ['webhook-events', app.id],
    queryFn: async () => {
      const response = await api.webhooks.getEvents(app.id, 50);
      return response.data;
    },
    refetchInterval: 10000,
  });

  // Fetch webhook stats
  const { data: stats } = useQuery({
    queryKey: ['webhook-stats', app.id],
    queryFn: async () => {
      const response = await api.webhooks.getStats(app.id);
      return response.data;
    },
    refetchInterval: 10000,
  });

  // Add route mutation
  const addRouteMutation = useMutation({
    mutationFn: (data: { tunnel_id: string; priority: number }) =>
      api.webhooks.addRoute(app.id, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['webhook-routes', app.id] });
      setAddRouteDialogOpen(false);
      setSelectedTunnelId('');
      setRoutePriority('100');
      toast.success('Route added successfully');
    },
    onError: (error: any) => {
      toast.error(error.response?.data?.error || 'Failed to add route');
    },
  });

  // Toggle route mutation
  const toggleRouteMutation = useMutation({
    mutationFn: (routeId: string) => api.webhooks.toggleRoute(app.id, routeId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['webhook-routes', app.id] });
      toast.success('Route toggled');
    },
    onError: (error: any) => {
      toast.error(error.response?.data?.error || 'Failed to toggle route');
    },
  });

  // Delete route mutation
  const deleteRouteMutation = useMutation({
    mutationFn: (routeId: string) => api.webhooks.deleteRoute(app.id, routeId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['webhook-routes', app.id] });
      toast.success('Route deleted');
    },
    onError: (error: any) => {
      toast.error(error.response?.data?.error || 'Failed to delete route');
    },
  });

  // Update route priority mutation
  const updateRouteMutation = useMutation({
    mutationFn: ({ routeId, priority }: { routeId: string; priority: number }) =>
      api.webhooks.updateRoute(app.id, routeId, { priority }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['webhook-routes', app.id] });
      toast.success('Priority updated');
    },
    onError: (error: any) => {
      toast.error(error.response?.data?.error || 'Failed to update priority');
    },
  });

  const handleAddRoute = () => {
    if (!selectedTunnelId) {
      toast.error('Please select a tunnel');
      return;
    }
    const priority = parseInt(routePriority, 10);
    if (isNaN(priority) || priority < 1) {
      toast.error('Priority must be a positive number');
      return;
    }
    addRouteMutation.mutate({ tunnel_id: selectedTunnelId, priority });
  };

  // Get org subdomain from user context (you may need to fetch this from auth context)
  const orgSubdomain = 'trofeo'; // TODO: Get from auth context
  const baseDomain = config?.domain || 'grok.io';
  const webhookUrl = `${orgSubdomain}-webhook.${baseDomain}/${app.name}/*`;

  // Filter available tunnels (exclude already added ones)
  const availableTunnels =
    tunnels?.filter(
      (tunnel: Tunnel) => !routes?.some((route: WebhookRoute) => route.tunnel_id === tunnel.id)
    ) || [];

  const getStatusIcon = (status: string) => {
    switch (status) {
      case 'healthy':
        return <CheckCircle2 size={16} style={{ color: '#10b981' }} />;
      case 'unhealthy':
        return <XCircle size={16} style={{ color: '#ef4444' }} />;
      default:
        return <AlertCircle size={16} style={{ color: '#6b7280' }} />;
    }
  };

  const getStatusChip = (status: string) => {
    switch (status) {
      case 'healthy':
        return <Chip label="Healthy" color="success" size="small" />;
      case 'unhealthy':
        return <Chip label="Unhealthy" color="error" size="small" />;
      default:
        return <Chip label="Unknown" color="default" size="small" />;
    }
  };

  const statsCards = [
    {
      title: 'Total Events',
      value: stats?.total_events || 0,
      icon: Activity,
    },
    {
      title: 'Success Rate',
      value: stats?.total_events
        ? `${Math.round((stats.success_count / stats.total_events) * 100)}%`
        : '0%',
      subtitle: `${stats?.success_count || 0} / ${stats?.total_events || 0} successful`,
      icon: TrendingUp,
    },
    {
      title: 'Avg Duration',
      value: `${stats?.average_duration_ms ? Math.round(stats.average_duration_ms) : 0}ms`,
      icon: Clock,
    },
    {
      title: 'Total Bytes In',
      value: `${stats?.total_bytes_in ? (stats.total_bytes_in / 1024).toFixed(2) : 0} KB`,
    },
    {
      title: 'Total Bytes Out',
      value: `${stats?.total_bytes_out ? (stats.total_bytes_out / 1024).toFixed(2) : 0} KB`,
    },
    {
      title: 'Failed Events',
      value: stats?.failure_count || 0,
      icon: XCircle,
    },
  ];

  return (
    <Box>
      {/* Header */}
      <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', mb: 4 }}>
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 2 }}>
          <IconButton onClick={onBack}>
            <ArrowLeft size={20} />
          </IconButton>
          <Box>
            <Typography variant="h4" sx={{ fontWeight: 700 }}>
              {app.name}
            </Typography>
            <Typography variant="body2" color="text.secondary">
              {app.description || 'No description'}
            </Typography>
          </Box>
        </Box>
        <Chip
          label={app.is_active ? 'Active' : 'Inactive'}
          color={app.is_active ? 'success' : 'default'}
        />
      </Box>

      {/* Webhook URL */}
      <Card sx={{ mb: 3 }}>
        <CardContent sx={{ py: 3 }}>
          <Typography variant="h6" sx={{ mb: 1 }}>
            Webhook URL
          </Typography>
          <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
            Send webhook requests to this URL. All requests will be broadcast to all enabled
            tunnels.
          </Typography>
          <Box sx={{ display: 'flex', gap: 2 }}>
            <TextField
              fullWidth
              value={webhookUrl}
              InputProps={{ readOnly: true }}
              size="small"
              sx={{ fontFamily: 'monospace', fontSize: '0.875rem' }}
            />
            <Button
              variant="outlined"
              onClick={() => {
                navigator.clipboard.writeText(webhookUrl);
                toast.success('URL copied to clipboard');
              }}
            >
              Copy
            </Button>
          </Box>
          <Typography variant="caption" color="text.secondary" sx={{ mt: 1, display: 'block' }}>
            Replace * with your webhook path (e.g., /stripe/payment_intent)
          </Typography>
        </CardContent>
      </Card>

      {/* Tabs */}
      <Box sx={{ borderBottom: 1, borderColor: 'divider', mb: 3 }}>
        <Tabs value={activeTab} onChange={(_, newValue) => setActiveTab(newValue)}>
          <Tab label="Routes" />
          <Tab label="Events" />
          <Tab label="Statistics" />
        </Tabs>
      </Box>

      {/* Routes Tab */}
      {activeTab === 0 && (
        <Box sx={{ display: 'flex', flexDirection: 'column', gap: 3 }}>
          {/* Network Diagram */}
          <Card>
            <CardContent sx={{ py: 3 }}>
              <Typography variant="h6" sx={{ mb: 1 }}>
                Network Diagram
              </Typography>
              <Typography variant="body2" color="text.secondary" sx={{ mb: 3 }}>
                Visual representation of webhook routing to tunnels
              </Typography>
              <WebhookNetworkDiagram
                appName={app.name}
                orgSubdomain={orgSubdomain}
                baseDomain={baseDomain}
                routes={routes || []}
              />
            </CardContent>
          </Card>

          {/* Routes Table */}
          <Card>
            <CardContent sx={{ py: 3 }}>
              <Box
                sx={{
                  display: 'flex',
                  justifyContent: 'space-between',
                  alignItems: 'flex-start',
                  mb: 3,
                }}
              >
                <Box>
                  <Typography variant="h6" sx={{ mb: 0.5 }}>
                    Routes
                  </Typography>
                  <Typography variant="body2" color="text.secondary">
                    Manage tunnel routes for this webhook app
                  </Typography>
                </Box>
                <Button
                  variant="contained"
                  startIcon={<Plus size={16} />}
                  onClick={() => setAddRouteDialogOpen(true)}
                  sx={{ bgcolor: '#667eea', '&:hover': { bgcolor: '#5568d3' } }}
                >
                  Add Route
                </Button>
              </Box>

              {routesLoading ? (
                <Box sx={{ textAlign: 'center', py: 8 }}>
                  <CircularProgress />
                </Box>
              ) : !routes || routes.length === 0 ? (
                <Box sx={{ textAlign: 'center', py: 8 }}>
                  <Activity size={48} style={{ color: '#9e9e9e', opacity: 0.5, margin: '0 auto 8px' }} />
                  <Typography variant="body2" color="text.secondary">
                    No routes configured
                  </Typography>
                  <Typography variant="caption" color="text.secondary">
                    Add a tunnel to start receiving webhooks
                  </Typography>
                </Box>
              ) : (
                <TableContainer component={Paper} variant="outlined">
                  <Table>
                    <TableHead>
                      <TableRow>
                        <TableCell>Tunnel</TableCell>
                        <TableCell>Local Address</TableCell>
                        <TableCell>Priority</TableCell>
                        <TableCell>Health</TableCell>
                        <TableCell>Status</TableCell>
                        <TableCell align="right">Actions</TableCell>
                      </TableRow>
                    </TableHead>
                    <TableBody>
                      {routes.map((route: WebhookRoute) => (
                        <TableRow key={route.id}>
                          <TableCell>
                            <Typography variant="body2" fontWeight={500}>
                              {route.tunnel?.subdomain || 'Unknown'}
                            </Typography>
                          </TableCell>
                          <TableCell>
                            <Box component="code" sx={{ fontSize: '0.875rem', fontFamily: 'monospace' }}>
                              {route.tunnel?.local_addr || 'N/A'}
                            </Box>
                          </TableCell>
                          <TableCell>
                            <TextField
                              type="number"
                              size="small"
                              sx={{ width: 80 }}
                              value={route.priority}
                              onChange={(e) => {
                                const priority = parseInt(e.target.value, 10);
                                if (!isNaN(priority) && priority > 0) {
                                  updateRouteMutation.mutate({
                                    routeId: route.id,
                                    priority,
                                  });
                                }
                              }}
                            />
                          </TableCell>
                          <TableCell>
                            <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                              {getStatusIcon(route.health_status)}
                              {getStatusChip(route.health_status)}
                            </Box>
                          </TableCell>
                          <TableCell>
                            <Button
                              size="small"
                              onClick={() => toggleRouteMutation.mutate(route.id)}
                              disabled={toggleRouteMutation.isPending}
                              startIcon={
                                route.is_enabled ? (
                                  <ToggleRight size={20} style={{ color: '#10b981' }} />
                                ) : (
                                  <ToggleLeft size={20} style={{ color: '#9e9e9e' }} />
                                )
                              }
                            >
                              {route.is_enabled ? 'Enabled' : 'Disabled'}
                            </Button>
                          </TableCell>
                          <TableCell align="right">
                            <IconButton
                              size="small"
                              color="error"
                              onClick={() => {
                                if (confirm('Are you sure you want to remove this route?')) {
                                  deleteRouteMutation.mutate(route.id);
                                }
                              }}
                              disabled={deleteRouteMutation.isPending}
                            >
                              <Trash2 size={16} />
                            </IconButton>
                          </TableCell>
                        </TableRow>
                      ))}
                    </TableBody>
                  </Table>
                </TableContainer>
              )}
            </CardContent>
          </Card>
        </Box>
      )}

      {/* Events Tab */}
      {activeTab === 1 && (
        <Card>
          <CardContent sx={{ py: 3 }}>
            <Typography variant="h6" sx={{ mb: 1 }}>
              Recent Events
            </Typography>
            <Typography variant="body2" color="text.secondary" sx={{ mb: 3 }}>
              Last 50 webhook requests received
            </Typography>

            {!events || events.length === 0 ? (
              <Box sx={{ textAlign: 'center', py: 8 }}>
                <Clock size={48} style={{ color: '#9e9e9e', opacity: 0.5, margin: '0 auto 8px' }} />
                <Typography variant="body2" color="text.secondary">
                  No events yet
                </Typography>
                <Typography variant="caption" color="text.secondary">
                  Events will appear when webhooks are received
                </Typography>
              </Box>
            ) : (
              <TableContainer component={Paper} variant="outlined">
                <Table>
                  <TableHead>
                    <TableRow>
                      <TableCell>Timestamp</TableCell>
                      <TableCell>Method</TableCell>
                      <TableCell>Path</TableCell>
                      <TableCell>Status</TableCell>
                      <TableCell>Duration</TableCell>
                      <TableCell>Tunnels</TableCell>
                      <TableCell>Success</TableCell>
                    </TableRow>
                  </TableHead>
                  <TableBody>
                    {events.map((event: WebhookEvent) => (
                      <TableRow key={event.id}>
                        <TableCell>
                          <Typography variant="body2">
                            {new Date(event.created_at).toLocaleString()}
                          </Typography>
                        </TableCell>
                        <TableCell>
                          <Chip label={event.method} variant="outlined" size="small" />
                        </TableCell>
                        <TableCell>
                          <Box component="code" sx={{ fontSize: '0.875rem', fontFamily: 'monospace' }}>
                            {event.request_path}
                          </Box>
                        </TableCell>
                        <TableCell>
                          <Chip
                            label={event.status_code}
                            color={
                              event.status_code >= 200 && event.status_code < 300
                                ? 'success'
                                : event.status_code >= 400
                                ? 'error'
                                : 'warning'
                            }
                            size="small"
                          />
                        </TableCell>
                        <TableCell>
                          <Typography variant="body2">{event.duration_ms}ms</Typography>
                        </TableCell>
                        <TableCell>{event.tunnel_count}</TableCell>
                        <TableCell>
                          <Chip
                            label={`${event.success_count}/${event.tunnel_count}`}
                            color={event.success_count > 0 ? 'success' : 'error'}
                            size="small"
                          />
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </TableContainer>
            )}
          </CardContent>
        </Card>
      )}

      {/* Stats Tab */}
      {activeTab === 2 && (
        <Box
          sx={{
            display: 'grid',
            gridTemplateColumns: { xs: '1fr', md: 'repeat(2, 1fr)', lg: 'repeat(3, 1fr)' },
            gap: 3,
          }}
        >
          {statsCards.map((card, index) => {
            const IconComponent = card.icon;
            return (
              <Card key={index}>
                <CardContent>
                  <Box
                    sx={{
                      display: 'flex',
                      justifyContent: 'space-between',
                      alignItems: 'flex-start',
                      mb: 2,
                    }}
                  >
                    <Typography variant="body2" color="text.secondary" fontWeight={500}>
                      {card.title}
                    </Typography>
                    {IconComponent && <IconComponent size={16} style={{ color: '#9e9e9e' }} />}
                  </Box>
                  <Typography variant="h4" fontWeight={700} sx={{ mb: 0.5 }}>
                    {card.value}
                  </Typography>
                  {card.subtitle && (
                    <Typography variant="caption" color="text.secondary">
                      {card.subtitle}
                    </Typography>
                  )}
                </CardContent>
              </Card>
            );
          })}
        </Box>
      )}

      {/* Add Route Dialog */}
      <Dialog open={addRouteDialogOpen} onClose={() => setAddRouteDialogOpen(false)} maxWidth="sm" fullWidth>
        <DialogTitle>Add Webhook Route</DialogTitle>
        <DialogContent>
          <Typography variant="body2" color="text.secondary" sx={{ mb: 3 }}>
            Add a tunnel to receive webhook broadcasts
          </Typography>
          <Box sx={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
            <FormControl fullWidth>
              <InputLabel>Tunnel</InputLabel>
              <Select
                value={selectedTunnelId}
                label="Tunnel"
                onChange={(e) => setSelectedTunnelId(e.target.value)}
              >
                <MenuItem value="">Select a tunnel</MenuItem>
                {availableTunnels.map((tunnel: Tunnel) => (
                  <MenuItem key={tunnel.id} value={tunnel.id}>
                    {tunnel.subdomain} → {tunnel.local_addr}
                  </MenuItem>
                ))}
              </Select>
              {availableTunnels.length === 0 && (
                <FormHelperText>
                  No available tunnels. All active tunnels are already added.
                </FormHelperText>
              )}
            </FormControl>
            <TextField
              fullWidth
              label="Priority"
              type="number"
              value={routePriority}
              onChange={(e) => setRoutePriority(e.target.value)}
              helperText="Lower number = higher priority for response selection"
              InputProps={{ inputProps: { min: 1 } }}
            />
          </Box>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setAddRouteDialogOpen(false)}>Cancel</Button>
          <Button
            variant="contained"
            onClick={handleAddRoute}
            disabled={addRouteMutation.isPending || !selectedTunnelId}
          >
            {addRouteMutation.isPending ? 'Adding...' : 'Add Route'}
          </Button>
        </DialogActions>
      </Dialog>
    </Box>
  );
}
