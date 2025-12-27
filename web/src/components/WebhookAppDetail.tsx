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
  DialogContentText,
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
  Tooltip,
} from '@mui/material';
import {
  ArrowLeft,
  Plus,
  Trash2,
  ToggleLeft,
  ToggleRight,
  CheckCircle2,
  XCircle,
} from 'lucide-react';
import { toast } from 'sonner';
import { api } from '@/lib/api';
import type { WebhookApp, WebhookRoute, WebhookEvent, Tunnel } from '@/lib/api';
import { WebhookNetworkDiagram } from './WebhookNetworkDiagram';
import { useAllEvents } from '@/hooks/useSSE';
import { formatRelativeTime } from '@/lib/utils';

interface WebhookAppDetailProps {
  app: WebhookApp;
  onBack: () => void;
}

export function WebhookAppDetail({ app, onBack }: WebhookAppDetailProps) {
  const [addRouteDialogOpen, setAddRouteDialogOpen] = useState(false);
  const [deleteRouteDialogOpen, setDeleteRouteDialogOpen] = useState(false);
  const [deleteAppDialogOpen, setDeleteAppDialogOpen] = useState(false);
  const [routeToDelete, setRouteToDelete] = useState<WebhookRoute | null>(null);
  const [selectedTunnelId, setSelectedTunnelId] = useState('');
  const [routePriority, setRoutePriority] = useState('100');
  const [activeTab, setActiveTab] = useState(0);
  const queryClient = useQueryClient();

  // Subscribe to real-time events via SSE
  useAllEvents((event) => {
    // Handle webhook events for this specific app
    if (event.type.startsWith('webhook_') && event.data?.app_id === app.id) {
      queryClient.refetchQueries({ queryKey: ['webhook-routes', app.id] });
      queryClient.refetchQueries({ queryKey: ['webhook-events', app.id] });
    }
    // Handle tunnel events (for route management)
    if (event.type.startsWith('tunnel_')) {
      queryClient.refetchQueries({ queryKey: ['tunnels'] });
    }
  });

  // Fetch domain config for webhook URL
  const { data: config } = useQuery({
    queryKey: ['config'],
    queryFn: async () => {
      const response = await api.config.get();
      return response.data;
    },
  });

  // Fetch routes for this app - real-time updates via SSE
  const { data: routes, isLoading: routesLoading } = useQuery({
    queryKey: ['webhook-routes', app.id],
    queryFn: async () => {
      const response = await api.webhooks.listRoutes(app.id);
      return response.data;
    },
  });

  // Fetch available tunnels - real-time updates via SSE
  const { data: tunnels } = useQuery({
    queryKey: ['tunnels'],
    queryFn: async () => {
      const response = await api.tunnels.list();
      return response.data;
    },
  });

  // Fetch webhook events - real-time updates via SSE
  const { data: events } = useQuery({
    queryKey: ['webhook-events', app.id],
    queryFn: async () => {
      const response = await api.webhooks.getEvents(app.id, 50);
      return response.data;
    },
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

  // Delete app mutation
  const deleteAppMutation = useMutation({
    mutationFn: () => api.webhooks.deleteApp(app.id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['webhook-apps'] });
      toast.success('Webhook app deleted successfully');
      onBack(); // Navigate back to list
    },
    onError: (error: any) => {
      toast.error(error.response?.data?.error || 'Failed to delete webhook app');
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

  // Use webhook URL from backend (includes proper protocol and port)
  const webhookUrl = app.webhook_url || '';

  // Extract org subdomain and base domain for network diagram
  // Parse from webhook_url if available, otherwise use config/defaults
  const baseDomain = config?.domain || 'localhost';
  let orgSubdomain = 'org';
  if (webhookUrl) {
    // Extract org subdomain from new webhook URL format
    // Pattern: http://{app_name}-{org}-webhook.{domain}/*
    // Example: "http://payment-app-trofeo-webhook.localhost:8080/*" → "trofeo"
    const match = webhookUrl.match(/\/\/(.+?)-webhook\./);
    if (match) {
      // Extract full subdomain part: "payment-app-trofeo"
      const fullSubdomain = match[1];
      // Remove app name prefix to get org subdomain
      // Since app.name is "payment-app", we get "trofeo"
      if (fullSubdomain.startsWith(app.name + '-')) {
        orgSubdomain = fullSubdomain.substring(app.name.length + 1);
      } else {
        // Fallback: use the last segment before -webhook
        const parts = fullSubdomain.split('-');
        orgSubdomain = parts[parts.length - 1];
      }
    }
  }

  // Filter available tunnels (exclude already added ones and TCP tunnels)
  const availableTunnels =
    tunnels?.filter(
      (tunnel: Tunnel) =>
        !routes?.some((route: WebhookRoute) => route.tunnel_id === tunnel.id) &&
        tunnel.tunnel_type?.toLowerCase() !== 'tcp'
    ) || [];

  // Get health status from tunnel's online/offline status
  const getTunnelHealth = (route: WebhookRoute) => {
    return route.tunnel?.status === 'active' ? 'online' : 'offline';
  };

  const getStatusIcon = (route: WebhookRoute) => {
    const health = getTunnelHealth(route);
    if (health === 'online') {
      return <CheckCircle2 size={16} style={{ color: '#10b981' }} />;
    }
    return <XCircle size={16} style={{ color: '#ef4444' }} />;
  };

  const getStatusChip = (route: WebhookRoute) => {
    const health = getTunnelHealth(route);
    if (health === 'online') {
      return <Chip label="Online" color="success" variant="outlined" size="small" />;
    }
    return <Chip label="Offline" color="error" variant="outlined" size="small" />;
  };

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
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 2 }}>
          <Chip
            label={app.is_active ? 'Active' : 'Inactive'}
            color={app.is_active ? 'success' : 'default'}
            variant="outlined"
          />
          <Button
            variant="outlined"
            color="error"
            startIcon={<Trash2 size={16} />}
            onClick={() => setDeleteAppDialogOpen(true)}
          >
            Delete App
          </Button>
        </Box>
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
                          <TableCell sx={{ maxWidth: 200 }}>
                            <Tooltip title={route.tunnel?.local_addr || 'N/A'} arrow>
                              <Box
                                component="code"
                                sx={{
                                  fontSize: '0.875rem',
                                  fontFamily: 'monospace',
                                  overflow: 'hidden',
                                  textOverflow: 'ellipsis',
                                  whiteSpace: 'nowrap',
                                  display: 'block',
                                }}
                              >
                                {route.tunnel?.local_addr || 'N/A'}
                              </Box>
                            </Tooltip>
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
                              {getStatusIcon(route)}
                              {getStatusChip(route)}
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
                                setRouteToDelete(route);
                                setDeleteRouteDialogOpen(true);
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
                          <Typography variant="body2" color="text.secondary">
                            {formatRelativeTime(event.created_at)}
                          </Typography>
                        </TableCell>
                        <TableCell>
                          <Chip label={event.method} variant="outlined" size="small" />
                        </TableCell>
                        <TableCell sx={{ maxWidth: 300 }}>
                          <Tooltip title={event.request_path} arrow>
                            <Box
                              component="code"
                              sx={{
                                fontSize: '0.875rem',
                                fontFamily: 'monospace',
                                overflow: 'hidden',
                                textOverflow: 'ellipsis',
                                whiteSpace: 'nowrap',
                                display: 'block',
                              }}
                            >
                              {event.request_path}
                            </Box>
                          </Tooltip>
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
                            variant="outlined"
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
                            variant="outlined"
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

      {/* Add Route Dialog */}
      <Dialog open={addRouteDialogOpen} onClose={() => setAddRouteDialogOpen(false)} maxWidth="sm" fullWidth>
        <DialogTitle>Add Webhook Route</DialogTitle>
        <DialogContent>
          <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
            Add a tunnel to receive webhook broadcasts
          </Typography>
          <Box
            sx={{
              mb: 3,
              p: 2,
              borderRadius: 1,
              bgcolor: 'info.lighter',
              border: '1px solid',
              borderColor: 'info.light',
            }}
          >
            <Typography variant="body2" color="info.main" sx={{ fontWeight: 500 }}>
              ℹ️ Only HTTP tunnels are supported for webhook routing. TCP tunnels cannot be added as webhook routes.
            </Typography>
          </Box>
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

      {/* Delete Route Confirmation Dialog */}
      <Dialog
        open={deleteRouteDialogOpen}
        onClose={() => setDeleteRouteDialogOpen(false)}
        maxWidth="sm"
        fullWidth
      >
        <DialogTitle>Remove Route</DialogTitle>
        <DialogContent>
          <DialogContentText component="div">
            Are you sure you want to remove this route?
            {routeToDelete?.tunnel && (
              <Box sx={{ mt: 2 }}>
                <strong>Tunnel:</strong> {routeToDelete.tunnel.saved_name || routeToDelete.tunnel.subdomain}
                <br />
                <strong>Priority:</strong> {routeToDelete.priority}
              </Box>
            )}
            <Box sx={{ mt: 1 }}>
              This action cannot be undone.
            </Box>
          </DialogContentText>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setDeleteRouteDialogOpen(false)} color="inherit">
            Cancel
          </Button>
          <Button
            onClick={() => {
              if (routeToDelete) {
                deleteRouteMutation.mutate(routeToDelete.id);
                setDeleteRouteDialogOpen(false);
                setRouteToDelete(null);
              }
            }}
            color="error"
            variant="contained"
            disabled={deleteRouteMutation.isPending}
          >
            {deleteRouteMutation.isPending ? 'Removing...' : 'Remove'}
          </Button>
        </DialogActions>
      </Dialog>

      {/* Delete App Confirmation Dialog */}
      <Dialog
        open={deleteAppDialogOpen}
        onClose={() => setDeleteAppDialogOpen(false)}
        maxWidth="sm"
        fullWidth
      >
        <DialogTitle>Delete Webhook App</DialogTitle>
        <DialogContent>
          <DialogContentText component="div">
            Are you sure you want to delete the webhook app <strong>{app.name}</strong>?
            <Box sx={{ mt: 2, color: 'warning.main' }}>
              ⚠️ This will permanently delete:
            </Box>
            <Box component="ul" sx={{ mt: 1, pl: 2 }}>
              <li>All routes associated with this app</li>
              <li>All webhook event history</li>
              <li>The webhook URL will become invalid</li>
            </Box>
            <Box sx={{ mt: 2 }}>
              This action cannot be undone.
            </Box>
          </DialogContentText>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setDeleteAppDialogOpen(false)} color="inherit">
            Cancel
          </Button>
          <Button
            onClick={() => {
              deleteAppMutation.mutate();
              setDeleteAppDialogOpen(false);
            }}
            color="error"
            variant="contained"
            disabled={deleteAppMutation.isPending}
          >
            {deleteAppMutation.isPending ? 'Deleting...' : 'Delete App'}
          </Button>
        </DialogActions>
      </Dialog>
    </Box>
  );
}
