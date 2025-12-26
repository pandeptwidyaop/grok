import { useQuery } from '@tanstack/react-query';
import {
  Box,
  Card,
  CardContent,
  Typography,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Chip,
  Link,
  CircularProgress,
  Paper,
} from '@mui/material';
import { Globe, Activity, ArrowUpDown } from 'lucide-react';
import { api, type Tunnel } from '@/lib/api';

function TunnelList() {
  const { data: tunnels, isLoading } = useQuery({
    queryKey: ['tunnels'],
    queryFn: async () => {
      const response = await api.tunnels.list();
      return response.data;
    },
    // Real-time updates via SSE in Dashboard - no need for polling
  });

  const formatBytes = (bytes: number) => {
    if (bytes === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
  };

  const formatDate = (date: string) => {
    if (!date) return 'N/A';
    const d = new Date(date);
    return isNaN(d.getTime()) ? 'N/A' : d.toLocaleString();
  };

  const getStatusBadge = (status: string) => {
    if (!status) return <Chip label="Unknown" variant="outlined" size="small" />;

    switch (status.toLowerCase()) {
      case 'active':
        return <Chip label="Active" color="success" size="small" />;
      case 'offline':
        return <Chip label="Offline" color="warning" size="small" />;
      case 'disconnected':
        return <Chip label="Disconnected" color="error" size="small" />;
      case 'inactive':
        return <Chip label="Inactive" color="default" size="small" />;
      default:
        return <Chip label={status} variant="outlined" size="small" />;
    }
  };

  const getTypeBadge = (type: string) => {
    if (!type) return <Chip label="Unknown" size="small" />;

    const colors: Record<string, 'primary' | 'success' | 'secondary'> = {
      http: 'primary',
      https: 'success',
      tcp: 'secondary',
    };
    return (
      <Chip
        label={type.toUpperCase()}
        color={colors[type.toLowerCase()] || 'default'}
        size="small"
      />
    );
  };

  if (isLoading) {
    return (
      <Card>
        <CardContent sx={{ py: 8 }}>
          <Box sx={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 2 }}>
            <CircularProgress />
            <Typography variant="body2" color="text.secondary">
              Loading tunnels...
            </Typography>
          </Box>
        </CardContent>
      </Card>
    );
  }

  if (!tunnels || tunnels.length === 0) {
    return (
      <Card>
        <CardContent sx={{ py: 4 }}>
          <Box sx={{ mb: 3 }}>
            <Typography variant="h6" gutterBottom>
              Active Tunnels
            </Typography>
            <Typography variant="body2" color="text.secondary">
              No active tunnels. Start a tunnel with the Grok client.
            </Typography>
          </Box>
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
            <Globe size={64} style={{ opacity: 0.3 }} />
            <Typography variant="h6" color="text.secondary">
              No tunnels running
            </Typography>
            <Typography variant="body2" color="text.secondary">
              Run{' '}
              <Box
                component="code"
                sx={{
                  bgcolor: 'grey.100',
                  px: 1,
                  py: 0.5,
                  borderRadius: 1,
                  fontFamily: 'monospace',
                }}
              >
                grok http 3000
              </Box>{' '}
              to create your first tunnel
            </Typography>
          </Box>
        </CardContent>
      </Card>
    );
  }

  return (
    <Card>
      <CardContent sx={{ py: 4 }}>
        <Box sx={{ mb: 3 }}>
          <Typography variant="h6" gutterBottom>
            Active Tunnels
          </Typography>
          <Typography variant="body2" color="text.secondary">
            {tunnels.length} tunnel{tunnels.length !== 1 ? 's' : ''} currently active
          </Typography>
        </Box>
        <TableContainer component={Paper} variant="outlined">
          <Table>
            <TableHead>
              <TableRow>
                <TableCell>Type</TableCell>
                <TableCell>Name</TableCell>
                <TableCell>Public URL</TableCell>
                <TableCell>Local Address</TableCell>
                <TableCell>Status</TableCell>
                <TableCell align="right">Requests</TableCell>
                <TableCell align="right">Data In/Out</TableCell>
                <TableCell>Connected</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {tunnels.map((tunnel: Tunnel) => (
                <TableRow
                  key={tunnel.id}
                  sx={{
                    '&:hover': {
                      bgcolor: 'action.hover',
                    },
                  }}
                >
                  <TableCell>{getTypeBadge(tunnel.tunnel_type)}</TableCell>
                  <TableCell>
                    {tunnel.saved_name ? (
                      <Chip
                        label={tunnel.saved_name}
                        color="primary"
                        variant="outlined"
                        size="small"
                        sx={{ fontFamily: 'monospace', fontWeight: 500 }}
                      />
                    ) : (
                      <Typography variant="body2" color="text.secondary" sx={{ fontStyle: 'italic' }}>
                        —
                      </Typography>
                    )}
                  </TableCell>
                  <TableCell>
                    <Link
                      href={tunnel.public_url}
                      target="_blank"
                      rel="noopener noreferrer"
                      sx={{
                        display: 'flex',
                        alignItems: 'center',
                        gap: 1,
                        color: 'primary.main',
                        textDecoration: 'none',
                        '&:hover': {
                          textDecoration: 'underline',
                        },
                      }}
                    >
                      {tunnel.public_url}
                      <Globe size={16} />
                    </Link>
                  </TableCell>
                  <TableCell>
                    <Box
                      component="code"
                      sx={{
                        fontSize: '0.875rem',
                        fontFamily: 'monospace',
                      }}
                    >
                      {tunnel.local_addr}
                    </Box>
                  </TableCell>
                  <TableCell>{getStatusBadge(tunnel.status)}</TableCell>
                  <TableCell align="right">
                    <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'flex-end', gap: 1 }}>
                      <Activity size={16} style={{ color: '#9e9e9e' }} />
                      {(tunnel.requests_count ?? 0).toLocaleString()}
                    </Box>
                  </TableCell>
                  <TableCell align="right">
                    <Box sx={{ display: 'flex', flexDirection: 'column', alignItems: 'flex-end' }}>
                      <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.5, fontSize: '0.875rem' }}>
                        <ArrowUpDown size={12} style={{ color: '#9e9e9e' }} />
                        {formatBytes(tunnel.bytes_in ?? 0)} / {formatBytes(tunnel.bytes_out ?? 0)}
                      </Box>
                    </Box>
                  </TableCell>
                  <TableCell>
                    <Typography variant="body2" color="text.secondary">
                      {formatDate(tunnel.connected_at)}
                    </Typography>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </TableContainer>
      </CardContent>
    </Card>
  );
}

export default TunnelList;
