import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { useNavigate } from 'react-router-dom';
import {
  Box,
  Card,
  CardContent,
  Button,
  TextField,
  Dialog,
  DialogContent,
  DialogTitle,
  DialogActions,
  Chip,
  Typography,
  CircularProgress,
  IconButton,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Paper,
} from '@mui/material';
import { Building2, Plus, Eye } from 'lucide-react';
import { toast } from 'sonner';
import { api } from '@/lib/api';
import { formatRelativeTime } from '@/lib/utils';

export default function OrganizationList() {
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [isCreateOpen, setIsCreateOpen] = useState(false);
  const [formData, setFormData] = useState({
    name: '',
    subdomain: '',
    description: '',
  });

  const { data: organizations = [], isLoading } = useQuery({
    queryKey: ['organizations'],
    queryFn: async () => {
      const response = await api.organizations.list();
      return response.data;
    },
  });

  const createMutation = useMutation({
    mutationFn: (data: typeof formData) => api.organizations.create(data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['organizations'] });
      setIsCreateOpen(false);
      resetForm();
      toast.success('Organization created successfully');
    },
    onError: (error: any) => {
      toast.error(error.response?.data?.error || 'Failed to create organization');
    },
  });

  const resetForm = () => {
    setFormData({ name: '', subdomain: '', description: '' });
  };

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    createMutation.mutate(formData);
  };

  return (
    <Box sx={{ display: 'flex', flexDirection: 'column', gap: 3 }}>
      {/* Header */}
      <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
        <Box>
          <Typography variant="h4" sx={{ fontWeight: 700, mb: 0.5 }}>
            Organizations
          </Typography>
          <Typography variant="body2" color="text.secondary">
            Manage organizations and their settings
          </Typography>
        </Box>
        <Button
          variant="contained"
          startIcon={<Plus size={16} />}
          onClick={() => setIsCreateOpen(true)}
          sx={{ bgcolor: '#667eea', '&:hover': { bgcolor: '#5568d3' } }}
        >
          Create Organization
        </Button>
      </Box>

      {/* Organizations Table */}
      <Card>
        <CardContent sx={{ py: 4 }}>
          <Box sx={{ mb: 3 }}>
            <Typography variant="h6" gutterBottom>
              All Organizations
            </Typography>
            <Typography variant="body2" color="text.secondary">
              {organizations.length} organization{organizations.length !== 1 ? 's' : ''} registered
            </Typography>
          </Box>

          {isLoading ? (
            <Box sx={{ textAlign: 'center', py: 8 }}>
              <CircularProgress />
              <Typography variant="body2" color="text.secondary" sx={{ mt: 2 }}>
                Loading organizations...
              </Typography>
            </Box>
          ) : organizations.length === 0 ? (
            <Box sx={{ textAlign: 'center', py: 8 }}>
              <Building2 size={48} style={{ color: '#9e9e9e', opacity: 0.5, margin: '0 auto 16px' }} />
              <Typography variant="body2" color="text.secondary">
                No organizations yet. Create your first one!
              </Typography>
            </Box>
          ) : (
            <TableContainer component={Paper} variant="outlined">
              <Table>
                <TableHead>
                  <TableRow>
                    <TableCell sx={{ fontWeight: 600 }}>Name</TableCell>
                    <TableCell sx={{ fontWeight: 600 }}>Domain</TableCell>
                    <TableCell sx={{ fontWeight: 600 }}>Description</TableCell>
                    <TableCell sx={{ fontWeight: 600 }}>Created</TableCell>
                    <TableCell sx={{ fontWeight: 600 }}>Status</TableCell>
                    <TableCell align="right" sx={{ fontWeight: 600 }}>Actions</TableCell>
                  </TableRow>
                </TableHead>
                <TableBody>
                  {organizations.map((org) => (
                    <TableRow
                      key={org.id}
                      hover
                      sx={{
                        cursor: 'pointer',
                        '&:hover': {
                          bgcolor: 'action.hover',
                        },
                      }}
                      onClick={() => navigate(`/organizations/${org.id}`)}
                    >
                      <TableCell>
                        <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                          <Building2 size={16} style={{ color: '#667eea' }} />
                          <Typography variant="body2" sx={{ fontWeight: 500 }}>
                            {org.name}
                          </Typography>
                        </Box>
                      </TableCell>
                      <TableCell>
                        <Chip
                          label={org.full_domain}
                          color="secondary"
                          variant="outlined"
                          size="small"
                          sx={{ fontFamily: 'monospace', fontSize: '0.75rem', fontWeight: 500 }}
                        />
                      </TableCell>
                      <TableCell>
                        <Typography variant="body2" color="text.secondary" noWrap sx={{ maxWidth: 250 }}>
                          {org.description || '—'}
                        </Typography>
                      </TableCell>
                      <TableCell>
                        <Typography variant="body2" color="text.secondary">
                          {formatRelativeTime(org.created_at)}
                        </Typography>
                      </TableCell>
                      <TableCell>
                        <Chip
                          label={org.is_active ? 'Active' : 'Inactive'}
                          color={org.is_active ? 'success' : 'default'}
                          variant="outlined"
                          size="small"
                        />
                      </TableCell>
                      <TableCell align="right">
                        <IconButton
                          size="small"
                          onClick={(e) => {
                            e.stopPropagation();
                            navigate(`/organizations/${org.id}`);
                          }}
                          title="View Details"
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

      {/* Create Organization Dialog */}
      <Dialog
        open={isCreateOpen}
        onClose={() => {
          setIsCreateOpen(false);
          resetForm();
        }}
        maxWidth="sm"
        fullWidth
      >
        <DialogTitle>Create Organization</DialogTitle>
        <form onSubmit={handleSubmit}>
          <DialogContent>
            <Typography variant="body2" color="text.secondary" sx={{ mb: 3 }}>
              Add a new organization to the system
            </Typography>
            <Box sx={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
              <TextField
                fullWidth
                label="Organization Name"
                placeholder="ACME Corp"
                value={formData.name}
                onChange={(e) => setFormData({ ...formData, name: e.target.value })}
                required
              />
              <TextField
                fullWidth
                label="Subdomain"
                placeholder="acme"
                value={formData.subdomain}
                onChange={(e) =>
                  setFormData({ ...formData, subdomain: e.target.value.toLowerCase() })
                }
                required
                helperText={`Lowercase letters, numbers, and hyphens only. Will be used as: custom-${formData.subdomain}.domain.com`}
              />
              <TextField
                fullWidth
                label="Description (optional)"
                placeholder="Organization description..."
                value={formData.description}
                onChange={(e) => setFormData({ ...formData, description: e.target.value })}
                multiline
                rows={3}
              />
            </Box>
          </DialogContent>
          <DialogActions>
            <Button
              onClick={() => {
                setIsCreateOpen(false);
                resetForm();
              }}
            >
              Cancel
            </Button>
            <Button
              type="submit"
              variant="contained"
              disabled={createMutation.isPending}
            >
              Create
            </Button>
          </DialogActions>
        </form>
      </Dialog>
    </Box>
  );
}
