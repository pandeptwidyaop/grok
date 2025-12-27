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
  Divider,
  useMediaQuery,
  useTheme,
  FormControlLabel,
  Switch,
} from '@mui/material';
import { Globe, Copy, Check, Trash2, Eye, Circle } from 'lucide-react';
import { api, type Tunnel } from '@/lib/api';
import { toast } from 'sonner';

function TunnelList() {
  const [copiedUrl, setCopiedUrl] = useState<string | null>(null);
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false);
  const [tunnelToDelete, setTunnelToDelete] = useState<Tunnel | null>(null);
  const [showAll, setShowAll] = useState(false);
  const queryClient = useQueryClient();
  const navigate = useNavigate();
  const theme = useTheme();
  const isMobile = useMediaQuery(theme.breakpoints.down('md'));

  const { data: tunnels, isLoading } = useQuery({
    queryKey: ['tunnels', showAll],
    queryFn: async () => {
      const response = await api.tunnels.list({ show_all: showAll });
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
          <Box sx={{ mb: 3, display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', flexWrap: 'wrap', gap: 2 }}>
            <Box>
              <Typography variant="h6" gutterBottom>
                Active Tunnels
              </Typography>
              <Typography variant="body2" color="text.secondary">
                {showAll ? 'No tunnels found.' : 'No active tunnels. Start a tunnel using the CLI client.'}
              </Typography>
            </Box>
            <FormControlLabel
              control={
                <Switch
                  checked={showAll}
                  onChange={(e) => setShowAll(e.target.checked)}
                  color="primary"
                />
              }
              label={
                <Typography variant="body2" color="text.secondary">
                  Show all tunnels
                </Typography>
              }
            />
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
          <Box sx={{ mb: 3, display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', flexWrap: 'wrap', gap: 2 }}>
            <Box>
              <Typography variant="h6" gutterBottom>
                Active Tunnels
              </Typography>
              <Typography variant="body2" color="text.secondary">
                {tunnels.length} tunnel{tunnels.length !== 1 ? 's' : ''} {showAll ? 'total' : 'active or recently offline'}
              </Typography>
            </Box>
            <FormControlLabel
              control={
                <Switch
                  checked={showAll}
                  onChange={(e) => setShowAll(e.target.checked)}
                  color="primary"
                />
              }
              label={
                <Typography variant="body2" color="text.secondary">
                  Show all tunnels
                </Typography>
              }
            />
          </Box>

          {/* Mobile Card View */}
          {isMobile ? (
            <Box sx={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
              {tunnels.map((tunnel) => (
                <Card
                  key={tunnel.id}
                  variant="outlined"
                  sx={{
                    cursor: 'pointer',
                    transition: 'box-shadow 0.2s',
                    '&:hover': {
                      boxShadow: 3,
                    },
                  }}
                  onClick={() => handleViewDetails(tunnel.id)}
                >
                  <CardContent sx={{ p: 2 }}>
                    {/* Header: Name, Type, Status */}
                    <Box sx={{ display: 'flex', alignItems: 'flex-start', justifyContent: 'space-between', mb: 2 }}>
                      <Box sx={{ flex: 1, minWidth: 0 }}>
                        <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mb: 0.5 }}>
                          <Globe size={18} style={{ color: '#667eea', flexShrink: 0 }} />
                          <Typography variant="body1" sx={{ fontWeight: 600, wordBreak: 'break-word' }}>
                            {tunnel.saved_name || tunnel.subdomain}
                          </Typography>
                        </Box>
                        {tunnel.saved_name && tunnel.tunnel_type?.toLowerCase() !== 'tcp' && tunnel.subdomain !== 'pending-allocation' && (
                          <Typography variant="caption" color="text.secondary" sx={{ ml: '26px', display: 'block' }}>
                            {tunnel.subdomain}
                          </Typography>
                        )}
                      </Box>
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
                          flexShrink: 0,
                          '& .MuiChip-icon': {
                            marginLeft: '8px',
                          },
                        }}
                      />
                    </Box>

                    {/* Type & Organization */}
                    <Box sx={{ display: 'flex', gap: 1, mb: 2, flexWrap: 'wrap' }}>
                      <Chip
                        label={tunnel.tunnel_type?.toUpperCase() || 'HTTP'}
                        size="small"
                        variant="outlined"
                        color={tunnel.tunnel_type?.toLowerCase() === 'tcp' ? 'secondary' : 'primary'}
                        sx={{ fontWeight: 500 }}
                      />
                      {tunnel.organization_name && (
                        <Chip
                          label={tunnel.organization_name}
                          color="secondary"
                          variant="outlined"
                          size="small"
                          sx={{ fontWeight: 500 }}
                        />
                      )}
                    </Box>

                    <Divider sx={{ my: 2 }} />

                    {/* Owner Info */}
                    {tunnel.owner_name && (
                      <Box sx={{ mb: 1.5 }}>
                        <Typography variant="caption" color="text.secondary" sx={{ display: 'block', mb: 0.5 }}>
                          Owner
                        </Typography>
                        <Typography variant="body2" sx={{ fontWeight: 500 }}>
                          {tunnel.owner_name}
                        </Typography>
                        {tunnel.owner_email && (
                          <Typography variant="caption" color="text.secondary">
                            {tunnel.owner_email}
                          </Typography>
                        )}
                      </Box>
                    )}

                    {/* Local Address */}
                    <Box sx={{ mb: 1.5 }}>
                      <Typography variant="caption" color="text.secondary" sx={{ display: 'block', mb: 0.5 }}>
                        Local Address
                      </Typography>
                      <Typography variant="body2" sx={{ fontFamily: 'monospace', fontSize: '0.875rem', wordBreak: 'break-all' }}>
                        {tunnel.local_addr}
                      </Typography>
                    </Box>

                    {/* Public URL */}
                    <Box sx={{ mb: 2 }}>
                      <Typography variant="caption" color="text.secondary" sx={{ display: 'block', mb: 0.5 }}>
                        Public URL
                      </Typography>
                      {tunnel.tunnel_type?.toLowerCase() === 'tcp' ? (
                        <Box
                          component="code"
                          sx={{
                            bgcolor: 'action.hover',
                            px: 1,
                            py: 0.5,
                            borderRadius: 1,
                            fontSize: '0.875rem',
                            fontFamily: 'monospace',
                            display: 'inline-block',
                            wordBreak: 'break-all',
                          }}
                        >
                          {tunnel.public_url}
                        </Box>
                      ) : (
                        <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                          <Typography
                            variant="body2"
                            sx={{
                              fontFamily: 'monospace',
                              fontSize: '0.875rem',
                              color: 'primary.main',
                              wordBreak: 'break-all',
                              flex: 1,
                            }}
                          >
                            {tunnel.public_url}
                          </Typography>
                          <IconButton
                            size="medium"
                            onClick={(e) => {
                              e.stopPropagation();
                              handleCopyUrl(tunnel.public_url);
                            }}
                            sx={{
                              minWidth: 44,
                              minHeight: 44,
                              flexShrink: 0,
                            }}
                          >
                            {copiedUrl === tunnel.public_url ? (
                              <Check size={20} style={{ color: '#4caf50' }} />
                            ) : (
                              <Copy size={20} />
                            )}
                          </IconButton>
                        </Box>
                      )}
                    </Box>

                    {/* Actions */}
                    <Box sx={{ display: 'flex', gap: 1, pt: 1 }}>
                      <Button
                        fullWidth
                        variant="outlined"
                        color="primary"
                        startIcon={<Eye size={18} />}
                        onClick={(e) => {
                          e.stopPropagation();
                          handleViewDetails(tunnel.id);
                        }}
                        sx={{ minHeight: 44 }}
                      >
                        View Details
                      </Button>
                      <Button
                        fullWidth
                        variant="outlined"
                        color="error"
                        startIcon={<Trash2 size={18} />}
                        onClick={(e) => {
                          e.stopPropagation();
                          handleDeleteClick(tunnel);
                        }}
                        sx={{ minHeight: 44 }}
                      >
                        Delete
                      </Button>
                    </Box>
                  </CardContent>
                </Card>
              ))}
            </Box>
          ) : (
            /* Desktop Table View */
            <TableContainer>
              <Table>
                <TableHead>
                  <TableRow>
                    <TableCell sx={{ fontWeight: 600 }}>Tunnel</TableCell>
                    <TableCell sx={{ fontWeight: 600 }}>Type</TableCell>
                    <TableCell sx={{ fontWeight: 600 }}>Organization</TableCell>
                    <TableCell sx={{ fontWeight: 600 }}>Owner</TableCell>
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
                        {tunnel.organization_name ? (
                          <Chip
                            label={tunnel.organization_name}
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
                      <TableCell onClick={() => handleViewDetails(tunnel.id)}>
                        <Box>
                          <Typography variant="body2" sx={{ fontWeight: 500 }}>
                            {tunnel.owner_name || '—'}
                          </Typography>
                          {tunnel.owner_email && (
                            <Typography variant="caption" color="text.secondary">
                              {tunnel.owner_email}
                            </Typography>
                          )}
                        </Box>
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
                      <TableCell onClick={() => handleViewDetails(tunnel.id)} sx={{ maxWidth: 200 }}>
                        <Tooltip title={tunnel.local_addr} arrow>
                          <Typography
                            variant="body2"
                            sx={{
                              fontFamily: 'monospace',
                              fontSize: '0.875rem',
                              overflow: 'hidden',
                              textOverflow: 'ellipsis',
                              whiteSpace: 'nowrap',
                            }}
                          >
                            {tunnel.local_addr}
                          </Typography>
                        </Tooltip>
                      </TableCell>
                      <TableCell onClick={() => handleViewDetails(tunnel.id)} sx={{ maxWidth: 250 }}>
                        {tunnel.tunnel_type?.toLowerCase() === 'tcp' ? (
                          <Tooltip title={tunnel.public_url} arrow>
                            <Box
                              component="code"
                              sx={{
                                bgcolor: 'action.hover',
                                px: 1,
                                py: 0.5,
                                borderRadius: 1,
                                fontSize: '0.875rem',
                                fontFamily: 'monospace',
                                display: 'block',
                                overflow: 'hidden',
                                textOverflow: 'ellipsis',
                                whiteSpace: 'nowrap',
                              }}
                            >
                              {tunnel.public_url}
                            </Box>
                          </Tooltip>
                        ) : (
                          <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, minWidth: 0 }}>
                            <Tooltip title={tunnel.public_url} arrow>
                              <Typography
                                variant="body2"
                                sx={{
                                  fontFamily: 'monospace',
                                  fontSize: '0.875rem',
                                  color: 'primary.main',
                                  overflow: 'hidden',
                                  textOverflow: 'ellipsis',
                                  whiteSpace: 'nowrap',
                                  flex: 1,
                                  minWidth: 0,
                                }}
                              >
                                {tunnel.public_url}
                              </Typography>
                            </Tooltip>
                            <IconButton
                              size="small"
                              onClick={(e) => {
                                e.stopPropagation();
                                handleCopyUrl(tunnel.public_url);
                              }}
                              sx={{ flexShrink: 0 }}
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
          )}
        </CardContent>
      </Card>

      {/* Delete Confirmation Dialog */}
      <Dialog
        open={deleteDialogOpen}
        onClose={handleDeleteCancel}
        maxWidth="sm"
        fullWidth
        fullScreen={isMobile}
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
