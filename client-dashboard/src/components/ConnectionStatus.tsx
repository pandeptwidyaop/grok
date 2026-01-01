import {
  Paper,
  Grid,
  Typography,
  Box,
  Chip,
  Card,
  CardContent,
} from '@mui/material';
import {
  CheckCircle,
  Cancel,
  Link as LinkIcon,
  Storage,
  Speed,
} from '@mui/icons-material';
import type { TunnelStatus } from '@/types';

interface ConnectionStatusProps {
  status: TunnelStatus | null;
}

function ConnectionStatus({ status }: ConnectionStatusProps) {
  if (!status) {
    return (
      <Paper sx={{ p: 3 }}>
        <Typography variant="h6" gutterBottom>
          Connection Status
        </Typography>
        <Typography variant="body2" color="text.secondary">
          Loading...
        </Typography>
      </Paper>
    );
  }

  const connected = status.connected;

  return (
    <Paper sx={{ p: 3 }}>
      <Box sx={{ display: 'flex', alignItems: 'center', mb: 3 }}>
        <Typography variant="h6" sx={{ flexGrow: 1 }}>
          Connection Status
        </Typography>
        <Chip
          icon={connected ? <CheckCircle /> : <Cancel />}
          label={connected ? 'Connected' : 'Disconnected'}
          color={connected ? 'success' : 'error'}
        />
      </Box>

      {connected && (
        <Grid container spacing={2}>
          {/* Public URL */}
          <Grid item xs={12} md={6}>
            <Card variant="outlined">
              <CardContent>
                <Box sx={{ display: 'flex', alignItems: 'center', mb: 1 }}>
                  <LinkIcon sx={{ mr: 1, color: 'primary.main' }} />
                  <Typography variant="subtitle2" color="text.secondary">
                    Public URL
                  </Typography>
                </Box>
                <Typography variant="body1" sx={{ fontFamily: 'monospace' }}>
                  {status.public_url || 'N/A'}
                </Typography>
              </CardContent>
            </Card>
          </Grid>

          {/* Local Address */}
          <Grid item xs={12} md={6}>
            <Card variant="outlined">
              <CardContent>
                <Box sx={{ display: 'flex', alignItems: 'center', mb: 1 }}>
                  <Storage sx={{ mr: 1, color: 'primary.main' }} />
                  <Typography variant="subtitle2" color="text.secondary">
                    Local Address
                  </Typography>
                </Box>
                <Typography variant="body1" sx={{ fontFamily: 'monospace' }}>
                  {status.local_addr || 'N/A'}
                </Typography>
              </CardContent>
            </Card>
          </Grid>

          {/* Protocol */}
          <Grid item xs={12} md={6}>
            <Card variant="outlined">
              <CardContent>
                <Box sx={{ display: 'flex', alignItems: 'center', mb: 1 }}>
                  <Speed sx={{ mr: 1, color: 'primary.main' }} />
                  <Typography variant="subtitle2" color="text.secondary">
                    Protocol
                  </Typography>
                </Box>
                <Typography variant="body1" sx={{ textTransform: 'uppercase' }}>
                  {status.protocol || 'N/A'}
                </Typography>
              </CardContent>
            </Card>
          </Grid>

          {/* Uptime */}
          <Grid item xs={12} md={6}>
            <Card variant="outlined">
              <CardContent>
                <Typography variant="subtitle2" color="text.secondary" gutterBottom>
                  Uptime
                </Typography>
                <Typography variant="body1">
                  {formatUptime(status.uptime_seconds)}
                </Typography>
              </CardContent>
            </Card>
          </Grid>


        </Grid>
      )}

      {!connected && (
        <Typography variant="body2" color="text.secondary">
          Tunnel is not active. Start a tunnel to see connection details.
        </Typography>
      )}
    </Paper>
  );
}

function formatUptime(seconds: number): string {
  if (seconds < 60) return `${seconds}s`;
  if (seconds < 3600) return `${Math.floor(seconds / 60)}m ${seconds % 60}s`;
  if (seconds < 86400) {
    const hours = Math.floor(seconds / 3600);
    const minutes = Math.floor((seconds % 3600) / 60);
    return `${hours}h ${minutes}m`;
  }
  const days = Math.floor(seconds / 86400);
  const hours = Math.floor((seconds % 86400) / 3600);
  return `${days}d ${hours}h`;
}

export default ConnectionStatus;
