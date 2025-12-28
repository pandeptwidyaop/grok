import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { useNavigate } from 'react-router-dom';
import {
  Box,
  Card,
  CardContent,
  Button,
  Typography,
  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
  TextField,
  CircularProgress,
  Chip,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Paper,
  IconButton,
  Tooltip,
  Divider,
  useMediaQuery,
  useTheme,
} from '@mui/material';
import { Webhook, Plus, Eye } from 'lucide-react';
import { toast } from 'sonner';
import { api } from '@/lib/api';
import { useAllEvents } from '@/hooks/useSSE';
import { formatRelativeTime } from '@/lib/utils';

interface WebhookAppsProps {}

export function WebhookApps({}: WebhookAppsProps) {
  const [createDialogOpen, setCreateDialogOpen] = useState(false);
  const [newAppName, setNewAppName] = useState('');
  const [newAppDescription, setNewAppDescription] = useState('');
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const theme = useTheme();
  const isMobile = useMediaQuery(theme.breakpoints.down('md'));

  // Subscribe to real-time events via SSE
  useAllEvents((event) => {
    // Handle webhook-related events
    if (event.type.startsWith('webhook_')) {
      queryClient.refetchQueries({ queryKey: ['webhook-apps'] });
    }
  });

  // Fetch webhook apps - real-time updates via SSE, no polling needed
  const { data: apps, isLoading } = useQuery({
    queryKey: ['webhook-apps'],
    queryFn: async () => {
      const response = await api.webhooks.listApps();
      return response.data;
    },
  });

  // Create webhook app mutation
  const createMutation = useMutation({
    mutationFn: (data: { name: string; description: string }) => api.webhooks.createApp(data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['webhook-apps'] });
      setCreateDialogOpen(false);
      setNewAppName('');
      setNewAppDescription('');
      toast.success('Webhook app created successfully');
    },
    onError: (error: any) => {
      toast.error(error.response?.data?.error || 'Failed to create webhook app');
    },
  });

  const handleCreateApp = () => {
    if (!newAppName.trim()) {
      toast.error('App name is required');
      return;
    }
    createMutation.mutate({
      name: newAppName,
      description: newAppDescription,
    });
  };

  return (
    <Box>
      {/* Header */}
      <Box
        sx={{
          display: 'flex',
          flexDirection: { xs: 'column', sm: 'row' },
          justifyContent: 'space-between',
          alignItems: { xs: 'stretch', sm: 'flex-start' },
          gap: { xs: 2, sm: 0 },
          mb: 4,
        }}
      >
        <Box>
          <Typography variant="h4" sx={{ fontWeight: 700, mb: 0.5 }}>
            Webhook Apps
          </Typography>
          <Typography variant="body2" color="text.secondary">
            Manage webhook applications and broadcast routing
          </Typography>
        </Box>

        <Button
          variant="contained"
          startIcon={<Plus size={16} />}
          onClick={() => setCreateDialogOpen(true)}
          sx={{
            bgcolor: '#667eea',
            '&:hover': { bgcolor: '#5568d3' },
            minWidth: { xs: '100%', sm: 'auto' },
          }}
        >
          Create Webhook App
        </Button>
      </Box>

      {/* Create Dialog */}
      <Dialog
        open={createDialogOpen}
        onClose={() => setCreateDialogOpen(false)}
        maxWidth="sm"
        fullWidth
      >
        <DialogTitle>Create Webhook App</DialogTitle>
        <DialogContent>
          <Typography variant="body2" color="text.secondary" sx={{ mb: 3 }}>
            Create a new webhook app to receive broadcast events via multiple tunnels
          </Typography>
          <Box sx={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
            <Box>
              <TextField
                fullWidth
                label="App Name"
                placeholder="payment-app"
                value={newAppName}
                onChange={(e) => setNewAppName(e.target.value)}
                helperText="Lowercase alphanumeric with hyphens (3-50 chars)"
              />
            </Box>
            <Box>
              <TextField
                fullWidth
                label="Description"
                placeholder="Stripe webhook receiver"
                value={newAppDescription}
                onChange={(e) => setNewAppDescription(e.target.value)}
                multiline
                rows={3}
              />
            </Box>
          </Box>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setCreateDialogOpen(false)}>Cancel</Button>
          <Button
            variant="contained"
            onClick={handleCreateApp}
            disabled={createMutation.isPending || !newAppName.trim()}
          >
            {createMutation.isPending ? 'Creating...' : 'Create App'}
          </Button>
        </DialogActions>
      </Dialog>

      {/* Apps Table */}
      <Card>
        <CardContent sx={{ py: 4 }}>
          <Box sx={{ mb: 3 }}>
            <Typography variant="h6" gutterBottom>
              Webhook Applications
            </Typography>
            <Typography variant="body2" color="text.secondary">
              {apps?.length || 0} app{apps?.length !== 1 ? 's' : ''} configured
            </Typography>
          </Box>

          {isLoading ? (
            <Box sx={{ textAlign: 'center', py: 8 }}>
              <CircularProgress />
              <Typography variant="body2" color="text.secondary" sx={{ mt: 2 }}>
                Loading webhook apps...
              </Typography>
            </Box>
          ) : !apps || apps.length === 0 ? (
            <Box
              sx={{
                textAlign: 'center',
                py: 8,
                display: 'flex',
                flexDirection: 'column',
                alignItems: 'center',
                gap: 2,
              }}
            >
              <Webhook size={64} style={{ color: '#9e9e9e', opacity: 0.5 }} />
              <Typography variant="h6" color="text.secondary">
                No webhook apps
              </Typography>
              <Typography variant="body2" color="text.secondary">
                Create a webhook app to start broadcasting events to multiple tunnels
              </Typography>
            </Box>
          ) : isMobile ? (
            /* Mobile Card View */
            <Box sx={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
              {apps.map((app) => (
                <Card
                  key={app.id}
                  variant="outlined"
                  sx={{
                    cursor: 'pointer',
                    transition: 'box-shadow 0.2s',
                    '&:hover': {
                      boxShadow: 3,
                    },
                  }}
                  onClick={() => navigate(`/webhooks/${app.id}`)}
                >
                  <CardContent sx={{ p: 2 }}>
                    {/* Header: Name and Status */}
                    <Box sx={{ display: 'flex', alignItems: 'flex-start', justifyContent: 'space-between', mb: 2 }}>
                      <Box sx={{ flex: 1, minWidth: 0 }}>
                        <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mb: 0.5 }}>
                          <Webhook size={18} style={{ color: '#667eea', flexShrink: 0 }} />
                          <Typography variant="body1" sx={{ fontWeight: 600, wordBreak: 'break-word' }}>
                            {app.name}
                          </Typography>
                        </Box>
                        {app.description && (
                          <Typography variant="body2" color="text.secondary" sx={{ ml: '26px' }}>
                            {app.description}
                          </Typography>
                        )}
                      </Box>
                      <Chip
                        label={app.is_active ? 'Active' : 'Inactive'}
                        color={app.is_active ? 'success' : 'default'}
                        variant="outlined"
                        size="small"
                        sx={{ flexShrink: 0, ml: 1 }}
                      />
                    </Box>

                    <Divider sx={{ my: 2 }} />

                    {/* Organization */}
                    {app.organization_name && (
                      <Box sx={{ mb: 1.5 }}>
                        <Typography variant="caption" color="text.secondary" sx={{ display: 'block', mb: 0.5 }}>
                          Organization
                        </Typography>
                        <Chip
                          label={app.organization_name}
                          color="secondary"
                          variant="outlined"
                          size="small"
                          sx={{ fontWeight: 500 }}
                        />
                      </Box>
                    )}

                    {/* Owner */}
                    {app.owner_name && (
                      <Box sx={{ mb: 1.5 }}>
                        <Typography variant="caption" color="text.secondary" sx={{ display: 'block', mb: 0.5 }}>
                          Owner
                        </Typography>
                        <Typography variant="body2" sx={{ fontWeight: 500 }}>
                          {app.owner_name}
                        </Typography>
                        {app.owner_email && (
                          <Typography variant="caption" color="text.secondary">
                            {app.owner_email}
                          </Typography>
                        )}
                      </Box>
                    )}

                    {/* Routes and Created */}
                    <Box sx={{ display: 'flex', gap: 3, mb: 2 }}>
                      <Box>
                        <Typography variant="caption" color="text.secondary" sx={{ display: 'block', mb: 0.5 }}>
                          Routes
                        </Typography>
                        <Typography variant="body2" sx={{ fontWeight: 500 }}>
                          {app.routes?.length || 0}
                        </Typography>
                      </Box>
                      <Box>
                        <Typography variant="caption" color="text.secondary" sx={{ display: 'block', mb: 0.5 }}>
                          Created
                        </Typography>
                        <Typography variant="body2" color="text.secondary">
                          {formatRelativeTime(app.created_at)}
                        </Typography>
                      </Box>
                    </Box>

                    {/* Action Button */}
                    <Button
                      fullWidth
                      variant="outlined"
                      color="primary"
                      startIcon={<Eye size={18} />}
                      onClick={(e) => {
                        e.stopPropagation();
                        navigate(`/webhooks/${app.id}`);
                      }}
                    >
                      View Details
                    </Button>
                  </CardContent>
                </Card>
              ))}
            </Box>
          ) : (
            /* Desktop Table View */
            <TableContainer component={Paper} variant="outlined" sx={{ overflowX: 'auto' }}>
              <Table>
                <TableHead>
                  <TableRow>
                    <TableCell sx={{ fontWeight: 600 }}>Name</TableCell>
                    <TableCell sx={{ fontWeight: 600 }}>Organization</TableCell>
                    <TableCell sx={{ fontWeight: 600 }}>User</TableCell>
                    <TableCell sx={{ fontWeight: 600 }}>Description</TableCell>
                    <TableCell sx={{ fontWeight: 600 }}>Routes</TableCell>
                    <TableCell sx={{ fontWeight: 600 }}>Created</TableCell>
                    <TableCell sx={{ fontWeight: 600 }}>Status</TableCell>
                    <TableCell align="right" sx={{ fontWeight: 600 }}>
                      Actions
                    </TableCell>
                  </TableRow>
                </TableHead>
                <TableBody>
                  {apps.map((app) => (
                    <TableRow
                      key={app.id}
                      hover
                      sx={{
                        cursor: 'pointer',
                        '&:hover': {
                          bgcolor: 'action.hover',
                        },
                      }}
                      onClick={() => navigate(`/webhooks/${app.id}`)}
                    >
                      <TableCell>
                        <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                          <Webhook size={16} style={{ color: '#667eea' }} />
                          <Typography variant="body2" sx={{ fontWeight: 500 }}>
                            {app.name}
                          </Typography>
                        </Box>
                      </TableCell>
                      <TableCell>
                        {app.organization_name ? (
                          <Chip
                            label={app.organization_name}
                            color="secondary"
                            variant="outlined"
                            size="small"
                            sx={{ fontWeight: 500 }}
                          />
                        ) : (
                          <Typography variant="body2" color="text.secondary">
                            —
                          </Typography>
                        )}
                      </TableCell>
                      <TableCell>
                        <Box>
                          <Typography variant="body2" sx={{ fontWeight: 500 }}>
                            {app.owner_name || '—'}
                          </Typography>
                          <Typography variant="caption" color="text.secondary">
                            {app.owner_email || ''}
                          </Typography>
                        </Box>
                      </TableCell>
                      <TableCell sx={{ maxWidth: 300 }}>
                        <Tooltip title={app.description || '—'} arrow>
                          <Typography
                            variant="body2"
                            color="text.secondary"
                            sx={{
                              overflow: 'hidden',
                              textOverflow: 'ellipsis',
                              whiteSpace: 'nowrap',
                            }}
                          >
                            {app.description || '—'}
                          </Typography>
                        </Tooltip>
                      </TableCell>
                      <TableCell>
                        <Typography variant="body2">
                          {app.routes?.length || 0}
                        </Typography>
                      </TableCell>
                      <TableCell>
                        <Typography variant="body2" color="text.secondary">
                          {formatRelativeTime(app.created_at)}
                        </Typography>
                      </TableCell>
                      <TableCell>
                        <Chip
                          label={app.is_active ? 'Active' : 'Inactive'}
                          color={app.is_active ? 'success' : 'default'}
                          variant="outlined"
                          size="small"
                        />
                      </TableCell>
                      <TableCell align="right">
                        <IconButton
                          size="small"
                          onClick={(e) => {
                            e.stopPropagation();
                            navigate(`/webhooks/${app.id}`);
                          }}
                          color="primary"
                        >
                          <Eye size={18} />
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
  );
}
