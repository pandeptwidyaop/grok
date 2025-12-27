import { useEffect } from 'react';
import { Container, Box, Typography } from '@mui/material';
import { useQuery } from '@tanstack/react-query';
import { useSSEConnection } from './hooks/useSSE';
import { sseService } from './services/sseService';
import api from './services/api';
import ConnectionStatus from './components/ConnectionStatus';
import RequestLog from './components/RequestLog';
import PerformanceCharts from './components/PerformanceCharts';

function App() {
  const sseConnected = useSSEConnection();

  // Initialize SSE connection on mount
  useEffect(() => {
    console.log('[App] Initializing SSE connection...');
    sseService.connect();

    // Cleanup on unmount
    return () => {
      console.log('[App] Disconnecting SSE...');
      sseService.disconnect();
    };
  }, []);

  // Fetch tunnel status
  const { data: tunnelStatus } = useQuery({
    queryKey: ['tunnel-status'],
    queryFn: api.tunnel.getStatus,
    refetchInterval: 5000, // Refetch every 5 seconds
  });

  return (
    <Container maxWidth="xl" sx={{ py: 4 }}>
      {/* Header */}
      <Box sx={{ mb: 4 }}>
        <Typography variant="h3" component="h1" gutterBottom>
          Grok Client Dashboard
        </Typography>
        <Typography variant="body1" color="text.secondary">
          Real-time monitoring for your tunnel traffic
        </Typography>

        {/* SSE Connection Indicator */}
        <Box sx={{ mt: 2 }}>
          <Typography variant="body2" color={sseConnected ? 'success.main' : 'error.main'}>
            {sseConnected ? '● Live updates active' : '○ Reconnecting...'}
          </Typography>
        </Box>
      </Box>

      {/* Connection Status */}
      <Box sx={{ mb: 4 }}>
        <ConnectionStatus status={tunnelStatus || null} />
      </Box>

      {/* Performance Charts */}
      <Box sx={{ mb: 4 }}>
        <PerformanceCharts />
      </Box>

      {/* Request Log */}
      <Box>
        <RequestLog />
      </Box>
    </Container>
  );
}

export default App;
