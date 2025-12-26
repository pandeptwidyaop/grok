import { useParams, useNavigate } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import { Box, CircularProgress, Typography } from '@mui/material';
import { api } from '@/lib/api';
import { WebhookAppDetail } from './WebhookAppDetail';

export function WebhookAppDetailPage() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();

  const { data: app, isLoading } = useQuery({
    queryKey: ['webhook-app', id],
    queryFn: async () => {
      if (!id) throw new Error('Webhook app ID is required');
      const response = await api.webhooks.getApp(id);
      return response.data;
    },
    enabled: !!id,
  });

  if (isLoading) {
    return (
      <Box sx={{ display: 'flex', justifyContent: 'center', alignItems: 'center', minHeight: '400px' }}>
        <CircularProgress />
      </Box>
    );
  }

  if (!app) {
    return (
      <Box>
        <Typography variant="h6" color="error">
          Webhook app not found
        </Typography>
      </Box>
    );
  }

  return <WebhookAppDetail app={app} onBack={() => navigate('/webhooks')} />;
}
