import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import {
  Box,
  Card,
  CardContent,
  Typography,
  TextField,
  Button,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Chip,
  IconButton,
  Alert,
  AlertTitle,
  Paper,
  CircularProgress,
  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
  DialogContentText,
} from '@mui/material';
import { Key, Trash2, Copy, Check } from 'lucide-react';
import { api, type AuthToken } from '@/lib/api';
import { toast } from 'sonner';
import { formatRelativeTime } from '@/lib/utils';

function TokenManager() {
  const [newTokenName, setNewTokenName] = useState('');
  const [createdToken, setCreatedToken] = useState<AuthToken | null>(null);
  const [copiedId, setCopiedId] = useState<string | null>(null);
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false);
  const [tokenToDelete, setTokenToDelete] = useState<AuthToken | null>(null);
  const queryClient = useQueryClient();

  const { data: tokens, isLoading } = useQuery({
    queryKey: ['tokens'],
    queryFn: async () => {
      const response = await api.tokens.list();
      return response.data;
    },
  });

  const createMutation = useMutation({
    mutationFn: (name: string) => api.tokens.create(name, ['tunnel:create', 'tunnel:list']),
    onSuccess: (response) => {
      setCreatedToken(response.data);
      setNewTokenName('');
      queryClient.invalidateQueries({ queryKey: ['tokens'] });
    },
  });

  const deleteMutation = useMutation({
    mutationFn: (id: string) => api.tokens.delete(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['tokens'] });
    },
  });

  const handleCreate = () => {
    if (newTokenName.trim()) {
      createMutation.mutate(newTokenName);
    }
  };

  const handleCopy = async (token: string, id: string) => {
    await navigator.clipboard.writeText(token);
    setCopiedId(id);
    setTimeout(() => setCopiedId(null), 2000);
  };

  if (isLoading) {
    return (
      <Card>
        <CardContent sx={{ py: 8 }}>
          <Box sx={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 2 }}>
            <CircularProgress />
            <Typography variant="body2" color="text.secondary">
              Loading tokens...
            </Typography>
          </Box>
        </CardContent>
      </Card>
    );
  }

  return (
    <Box sx={{ display: 'flex', flexDirection: 'column', gap: 3 }}>
      {/* Create New Token Card */}
      <Card>
        <CardContent sx={{ py: 4 }}>
          <Box sx={{ mb: 3 }}>
            <Typography variant="h6" gutterBottom>
              Create New Token
            </Typography>
            <Typography variant="body2" color="text.secondary">
              Generate a new authentication token for the Grok client
            </Typography>
          </Box>
          <Box sx={{ display: 'flex', gap: 2 }}>
            <TextField
              fullWidth
              placeholder="Token name (e.g., laptop, server)"
              value={newTokenName}
              onChange={(e) => setNewTokenName(e.target.value)}
              onKeyPress={(e) => e.key === 'Enter' && handleCreate()}
              size="small"
            />
            <Button
              variant="contained"
              onClick={handleCreate}
              disabled={!newTokenName.trim()}
              sx={{ whiteSpace: 'nowrap' }}
            >
              Create Token
            </Button>
          </Box>
        </CardContent>
      </Card>

      {/* Success Alert when token is created */}
      {createdToken && createdToken.token && (
        <Alert
          severity="success"
          sx={{
            '& .MuiAlert-message': { width: '100%' },
          }}
        >
          <AlertTitle sx={{ fontWeight: 600 }}>Token Created Successfully!</AlertTitle>
          <Typography variant="body2" sx={{ mb: 2 }}>
            Make sure to copy your token now. You won't be able to see it again!
          </Typography>
          <Paper
            variant="outlined"
            sx={{
              p: 2,
              bgcolor: 'background.paper',
              display: 'flex',
              alignItems: 'center',
              gap: 2,
            }}
          >
            <Box
              component="code"
              sx={{
                flex: 1,
                fontFamily: 'monospace',
                fontSize: '0.875rem',
                wordBreak: 'break-all',
              }}
            >
              {createdToken.token}
            </Box>
            <Button
              size="small"
              variant="outlined"
              onClick={() => handleCopy(createdToken.token!, createdToken.id)}
              startIcon={copiedId === createdToken.id ? <Check size={16} /> : <Copy size={16} />}
            >
              {copiedId === createdToken.id ? 'Copied' : 'Copy'}
            </Button>
          </Paper>
          <Typography variant="caption" color="text.secondary" sx={{ mt: 2, display: 'block' }}>
            Use this token with:{' '}
            <Box
              component="code"
              sx={{
                bgcolor: 'action.hover',
                px: 1,
                py: 0.5,
                borderRadius: 1,
                fontFamily: 'monospace',
              }}
            >
              grok config set-token {createdToken.token}
            </Box>
          </Typography>
        </Alert>
      )}

      {/* Token List Card */}
      <Card>
        <CardContent sx={{ py: 4 }}>
          <Box sx={{ mb: 3 }}>
            <Typography variant="h6" gutterBottom>
              Authentication Tokens
            </Typography>
            <Typography variant="body2" color="text.secondary">
              {tokens?.length || 0} token{tokens?.length !== 1 ? 's' : ''} configured
            </Typography>
          </Box>

          {!tokens || tokens.length === 0 ? (
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
              <Key size={64} style={{ opacity: 0.3 }} />
              <Typography variant="h6" color="text.secondary">
                No tokens created
              </Typography>
              <Typography variant="body2" color="text.secondary">
                Create a token above to start using the Grok client
              </Typography>
            </Box>
          ) : (
            <TableContainer component={Paper} variant="outlined">
              <Table>
                <TableHead>
                  <TableRow>
                    <TableCell>Name</TableCell>
                    <TableCell>Owner</TableCell>
                    <TableCell>Organization</TableCell>
                    <TableCell>Last Used</TableCell>
                    <TableCell>Created</TableCell>
                    <TableCell>Status</TableCell>
                    <TableCell align="right">Actions</TableCell>
                  </TableRow>
                </TableHead>
                <TableBody>
                  {tokens.map((token: AuthToken) => (
                    <TableRow
                      key={token.id}
                      sx={{
                        '&:hover': {
                          bgcolor: 'action.hover',
                        },
                      }}
                    >
                      <TableCell>
                        <Typography variant="body2" fontWeight={500}>
                          {token.name}
                        </Typography>
                      </TableCell>
                      <TableCell>
                        <Typography variant="body2" sx={{ fontSize: '0.875rem' }}>
                          {token.owner_name || token.owner_email || '—'}
                        </Typography>
                      </TableCell>
                      <TableCell>
                        {token.organization_name ? (
                          <Chip
                            label={token.organization_name}
                            color="secondary"
                            variant="outlined"
                            size="small"
                            sx={{ fontWeight: 500 }}
                          />
                        ) : (
                          <Typography variant="body2" color="text.secondary" sx={{ fontStyle: 'italic' }}>
                            Personal
                          </Typography>
                        )}
                      </TableCell>
                      <TableCell>
                        <Typography variant="body2" color="text.secondary">
                          {token.last_used_at ? formatRelativeTime(token.last_used_at) : 'Never'}
                        </Typography>
                      </TableCell>
                      <TableCell>
                        <Typography variant="body2" color="text.secondary">
                          {formatRelativeTime(token.created_at)}
                        </Typography>
                      </TableCell>
                      <TableCell>
                        {token.is_active ? (
                          <Chip label="Active" color="success" variant="outlined" size="small" />
                        ) : (
                          <Chip label="Inactive" color="default" variant="outlined" size="small" />
                        )}
                      </TableCell>
                      <TableCell align="right">
                        <IconButton
                          size="small"
                          color="error"
                          onClick={() => {
                            setTokenToDelete(token);
                            setDeleteDialogOpen(true);
                          }}
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

      {/* Delete Confirmation Dialog */}
      <Dialog
        open={deleteDialogOpen}
        onClose={() => setDeleteDialogOpen(false)}
        maxWidth="sm"
        fullWidth
      >
        <DialogTitle>Delete Token</DialogTitle>
        <DialogContent>
          <DialogContentText component="div">
            Are you sure you want to delete the token <strong>{tokenToDelete?.name}</strong>?
            <Box sx={{ mt: 2, color: 'error.main' }}>
              ⚠️ Any applications using this token will lose access immediately.
            </Box>
            <Box sx={{ mt: 1 }}>
              This action cannot be undone.
            </Box>
          </DialogContentText>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setDeleteDialogOpen(false)} color="inherit">
            Cancel
          </Button>
          <Button
            onClick={() => {
              if (tokenToDelete) {
                deleteMutation.mutate(tokenToDelete.id);
                setDeleteDialogOpen(false);
                setTokenToDelete(null);
                toast.success('Token deleted successfully');
              }
            }}
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

export default TokenManager;
