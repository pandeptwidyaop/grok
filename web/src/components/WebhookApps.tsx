import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
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
} from '@mui/material';
import { Webhook, Plus } from 'lucide-react';
import { toast } from 'sonner';
import { api } from '@/lib/api';
import type { WebhookApp } from '@/lib/api';
import { WebhookAppDetail } from './WebhookAppDetail';

interface WebhookAppsProps {}

export function WebhookApps({}: WebhookAppsProps) {
  const [selectedApp, setSelectedApp] = useState<WebhookApp | null>(null);
  const [createDialogOpen, setCreateDialogOpen] = useState(false);
  const [newAppName, setNewAppName] = useState('');
  const [newAppDescription, setNewAppDescription] = useState('');
  const queryClient = useQueryClient();

  // Fetch webhook apps
  const { data: apps, isLoading } = useQuery({
    queryKey: ['webhook-apps'],
    queryFn: async () => {
      const response = await api.webhooks.listApps();
      return response.data;
    },
    refetchInterval: 5000,
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

  // If an app is selected, show detail view
  if (selectedApp) {
    return <WebhookAppDetail app={selectedApp} onBack={() => setSelectedApp(null)} />;
  }

  // List view
  return (
    <Box>
      {/* Header */}
      <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', mb: 4 }}>
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
          sx={{ bgcolor: '#667eea', '&:hover': { bgcolor: '#5568d3' } }}
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

      {/* Apps Grid */}
      {isLoading ? (
        <Box sx={{ textAlign: 'center', py: 12 }}>
          <CircularProgress />
          <Typography variant="body2" color="text.secondary" sx={{ mt: 2 }}>
            Loading webhook apps...
          </Typography>
        </Box>
      ) : !apps || apps.length === 0 ? (
        <Card>
          <CardContent
            sx={{
              display: 'flex',
              flexDirection: 'column',
              alignItems: 'center',
              justifyContent: 'center',
              py: 16,
            }}
          >
            <Webhook size={64} style={{ color: '#9e9e9e', opacity: 0.5, marginBottom: 16 }} />
            <Typography variant="h6" sx={{ mb: 1, fontWeight: 600 }}>
              No Webhook Apps
            </Typography>
            <Typography
              variant="body2"
              color="text.secondary"
              sx={{ textAlign: 'center', mb: 3, maxWidth: 400 }}
            >
              Create your first webhook app to start broadcasting events to multiple tunnels
            </Typography>
            <Button
              variant="contained"
              startIcon={<Plus size={16} />}
              onClick={() => setCreateDialogOpen(true)}
              sx={{ bgcolor: '#667eea', '&:hover': { bgcolor: '#5568d3' } }}
            >
              Create Your First App
            </Button>
          </CardContent>
        </Card>
      ) : (
        <Box
          sx={{
            display: 'grid',
            gridTemplateColumns: {
              xs: '1fr',
              md: 'repeat(2, 1fr)',
              lg: 'repeat(3, 1fr)',
            },
            gap: 3,
          }}
        >
          {apps.map((app) => (
            <Card
              key={app.id}
              sx={{
                cursor: 'pointer',
                transition: 'box-shadow 0.3s',
                '&:hover': {
                  boxShadow: 6,
                },
              }}
              onClick={() => setSelectedApp(app)}
            >
              <CardContent>
                <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', mb: 2 }}>
                  <Webhook size={32} style={{ color: '#667eea' }} />
                  <Chip
                    label={app.is_active ? 'Active' : 'Inactive'}
                    color={app.is_active ? 'success' : 'default'}
                    size="small"
                  />
                </Box>
                <Typography variant="h6" sx={{ fontWeight: 600, mb: 1 }}>
                  {app.name}
                </Typography>
                <Typography
                  variant="body2"
                  color="text.secondary"
                  sx={{
                    mb: 3,
                    display: '-webkit-box',
                    WebkitLineClamp: 2,
                    WebkitBoxOrient: 'vertical',
                    overflow: 'hidden',
                  }}
                >
                  {app.description || 'No description'}
                </Typography>
                <Box sx={{ display: 'flex', flexDirection: 'column', gap: 0.5 }}>
                  <Box sx={{ display: 'flex', justifyContent: 'space-between' }}>
                    <Typography variant="body2" color="text.secondary">
                      Routes:
                    </Typography>
                    <Typography variant="body2" fontWeight={500}>
                      {app.routes?.length || 0}
                    </Typography>
                  </Box>
                  <Box sx={{ display: 'flex', justifyContent: 'space-between' }}>
                    <Typography variant="body2" color="text.secondary">
                      Created:
                    </Typography>
                    <Typography variant="body2" fontWeight={500}>
                      {new Date(app.created_at).toLocaleDateString()}
                    </Typography>
                  </Box>
                </Box>
              </CardContent>
            </Card>
          ))}
        </Box>
      )}
    </Box>
  );
}
