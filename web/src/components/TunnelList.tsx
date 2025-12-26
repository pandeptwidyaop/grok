import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { useNavigate } from 'react-router-dom';
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
  IconButton,
  Tooltip,
  CircularProgress,
  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
  Button,
  DialogContentText,
} from '@mui/material';
import { Globe, Copy, Check, Trash2, Eye, Circle } from 'lucide-react';
import { api, type Tunnel } from '@/lib/api';
import { toast } from 'sonner';

function TunnelList() {
  const [copiedUrl, setCopiedUrl] = useState<string | null>(null);
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false);
  const [tunnelToDelete, setTunnelToDelete] = useState<Tunnel | null>(null);
  const queryClient = useQueryClient();
  const navigate = useNavigate();

  const { data: tunnels, isLoading } = useQuery({
    queryKey: ['tunnels'],
    queryFn: async () => {
      const response = await api.tunnels.list();
      return response.data;
    },
  });

  const deleteMutation = useMutation({
    mutationFn: (id: string) => api.tunnels.delete(id),
    onSuccess: () => {
      queryClient.refetchQueries({ queryKey: ['tunnels'] });
      queryClient.refetchQueries({ queryKey: ['stats'] });
      toast.success('Tunnel deleted successfully');
      setDeleteDialogOpen(false);
      setTunnelToDelete(null);
    },
    onError: (error: any) => {
      toast.error(error.response?.data?.error || 'Failed to delete tunnel');
    },
  });

  const handleCopyUrl = (url: string) => {
    navigator.clipboard.writeText(url);
    setCopiedUrl(url);
    toast.success('URL copied to clipboard');
    setTimeout(() => setCopiedUrl(null), 2000);
  };

  const handleDeleteClick = (tunnel: Tunnel) => {
    setTunnelToDelete(tunnel);
    setDeleteDialogOpen(true);
  };

  const handleDeleteCancel = () => {
    setDeleteDialogOpen(false);
    setTunnelToDelete(null);
  };

  const handleDeleteConfirm = () => {
    if (tunnelToDelete) {
      deleteMutation.mutate(tunnelToDelete.id);
    }
  };

  const handleViewDetails = (tunnelId: string) => {
    navigate(`/tunnels/${tunnelId}`);
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
              No active tunnels. Start a tunnel using the CLI client.
            </Typography>
          </Box>
        </CardContent>
      </Card>
    );
  }

  return (
    <Box>
      <Box sx={{ mb: 4 }}>
        <Typography variant="h4" sx={{ fontWeight: 700, color: '#667eea', mb: 1 }}>
          Tunnels
        </Typography>
        <Typography variant="body1" color="text.secondary">
          Manage your active and persistent tunnels
        </Typography>
      </Box>

      <Card>
        <CardContent sx={{ py: 4 }}>
          <Box sx={{ mb: 3 }}>
            <Typography variant="h6" gutterBottom>
              Active Tunnels
            </Typography>
            <Typography variant="body2" color="text.secondary">
              {tunnels.length} tunnel{tunnels.length !== 1 ? 's' : ''} running
            </Typography>
          </Box>

          <TableContainer>
            <Table>
              <TableHead>
                <TableRow>
                  <TableCell sx={{ fontWeight: 600 }}>Tunnel</TableCell>
                  <TableCell sx={{ fontWeight: 600 }}>Type</TableCell>
                  <TableCell sx={{ fontWeight: 600 }}>Status</TableCell>
                  <TableCell sx={{ fontWeight: 600 }}>Local Address</TableCell>
                  <TableCell sx={{ fontWeight: 600 }}>Public URL</TableCell>
                  <TableCell align="right" sx={{ fontWeight: 600 }}>
                    Actions
                  </TableCell>
                </TableRow>
              </TableHead>
              <TableBody>
                {tunnels.map((tunnel) => (
                  <TableRow
                    key={tunnel.id}
                    hover
                    sx={{
                      cursor: 'pointer',
                      '&:hover': {
                        bgcolor: 'action.hover',
                      },
                    }}
                  >
                    <TableCell onClick={() => handleViewDetails(tunnel.id)}>
                      <Box>
                        <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                          <Globe size={16} style={{ color: '#667eea' }} />
                          <Typography variant="body2" sx={{ fontWeight: 500 }}>
                            {tunnel.saved_name || tunnel.subdomain}
                          </Typography>
                        </Box>
                        {tunnel.saved_name && tunnel.tunnel_type?.toLowerCase() !== 'tcp' && tunnel.subdomain !== 'pending-allocation' && (
                          <Typography variant="caption" color="text.secondary" sx={{ ml: 3 }}>
                            {tunnel.subdomain}
                          </Typography>
                        )}
                      </Box>
                    </TableCell>
                    <TableCell onClick={() => handleViewDetails(tunnel.id)}>
                      <Chip
                        label={tunnel.tunnel_type?.toUpperCase() || 'HTTP'}
                        size="small"
                        variant="outlined"
                        color={tunnel.tunnel_type?.toLowerCase() === 'tcp' ? 'secondary' : 'primary'}
                        sx={{ fontWeight: 500 }}
                      />
                    </TableCell>
                    <TableCell onClick={() => handleViewDetails(tunnel.id)}>
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
                    </TableCell>
                    <TableCell onClick={() => handleViewDetails(tunnel.id)}>
                      <Typography variant="body2" sx={{ fontFamily: 'monospace', fontSize: '0.875rem' }}>
                        {tunnel.local_addr}
                      </Typography>
                    </TableCell>
                    <TableCell onClick={() => handleViewDetails(tunnel.id)}>
                      {tunnel.tunnel_type?.toLowerCase() === 'tcp' ? (
                        <Box>
                          <Box
                            component="code"
                            sx={{
                              bgcolor: 'action.hover',
                              px: 1,
                              py: 0.5,
                              borderRadius: 1,
                              fontSize: '0.875rem',
                              fontFamily: 'monospace',
                            }}
                          >
                            {tunnel.public_url}
                          </Box>
                        </Box>
                      ) : (
                        <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                          <Typography
                            variant="body2"
                            sx={{
                              fontFamily: 'monospace',
                              fontSize: '0.875rem',
                              color: 'primary.main',
                            }}
                          >
                            {tunnel.public_url}
                          </Typography>
                          <IconButton
                            size="small"
                            onClick={(e) => {
                              e.stopPropagation();
                              handleCopyUrl(tunnel.public_url);
                            }}
                          >
                            {copiedUrl === tunnel.public_url ? (
                              <Check size={16} style={{ color: '#4caf50' }} />
                            ) : (
                              <Copy size={16} />
                            )}
                          </IconButton>
                        </Box>
                      )}
                    </TableCell>
                    <TableCell align="right">
                      <Box sx={{ display: 'flex', justifyContent: 'flex-end', gap: 1 }}>
                        <Tooltip title="View details">
                          <IconButton
                            size="small"
                            onClick={(e) => {
                              e.stopPropagation();
                              handleViewDetails(tunnel.id);
                            }}
                            color="primary"
                          >
                            <Eye size={18} />
                          </IconButton>
                        </Tooltip>
                        <Tooltip title="Delete tunnel">
                          <IconButton
                            size="small"
                            onClick={(e) => {
                              e.stopPropagation();
                              handleDeleteClick(tunnel);
                            }}
                            color="error"
                          >
                            <Trash2 size={18} />
                          </IconButton>
                        </Tooltip>
                      </Box>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </TableContainer>
        </CardContent>
      </Card>

      {/* Delete Confirmation Dialog */}
      <Dialog
        open={deleteDialogOpen}
        onClose={handleDeleteCancel}
        maxWidth="sm"
        fullWidth
      >
        <DialogTitle>Delete Tunnel</DialogTitle>
        <DialogContent>
          <DialogContentText component="div">
            Are you sure you want to delete the tunnel{' '}
            <strong>{tunnelToDelete?.saved_name || tunnelToDelete?.subdomain}</strong>?
            {tunnelToDelete?.status === 'active' && (
              <Box sx={{ mt: 2, color: 'warning.main' }}>
                ⚠️ This tunnel is currently active and will be forcefully disconnected.
              </Box>
            )}
            <Box sx={{ mt: 1 }}>
              This action cannot be undone.
            </Box>
          </DialogContentText>
        </DialogContent>
        <DialogActions>
          <Button onClick={handleDeleteCancel} color="inherit">
            Cancel
          </Button>
          <Button
            onClick={handleDeleteConfirm}
            color="error"
            variant="contained"
            disabled={deleteMutation.isPending}
          >
            {deleteMutation.isPending ? 'Deleting...' : 'Delete'}
          </Button>
        </DialogActions>
      </Dialog>
    </Box>
  );
}

export default TunnelList;
