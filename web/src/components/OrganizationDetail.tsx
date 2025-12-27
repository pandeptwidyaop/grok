import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { useNavigate, useParams } from 'react-router-dom';
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
  FormControl,
  InputLabel,
  Select,
  MenuItem,
  Paper,
  useMediaQuery,
  useTheme,
} from '@mui/material';
import {
  ArrowLeft,
  Edit,
  Trash2,
  Power,
  UserPlus,
  Key,
  Users,
} from 'lucide-react';
import { toast } from 'sonner';
import { api } from '@/lib/api';
import { formatRelativeTime } from '@/lib/utils';

export default function OrganizationDetail() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const theme = useTheme();
  const isMobile = useMediaQuery(theme.breakpoints.down('md'));

  const [isEditOpen, setIsEditOpen] = useState(false);
  const [formData, setFormData] = useState({
    name: '',
    description: '',
  });
  const [userFormData, setUserFormData] = useState({
    email: '',
    name: '',
    password: '',
    role: 'org_user' as 'org_admin' | 'org_user',
  });
  const [resetPasswordDialog, setResetPasswordDialog] = useState<{
    isOpen: boolean;
    userId: string;
    userName: string;
    newPassword: string;
  }>({ isOpen: false, userId: '', userName: '', newPassword: '' });
  const [deleteDialog, setDeleteDialog] = useState<{
    isOpen: boolean;
    id: string;
    name: string;
    type: 'org' | 'user';
  }>({ isOpen: false, id: '', name: '', type: 'org' });

  // Fetch organization details
  const { data: organization, isLoading } = useQuery({
    queryKey: ['organization', id],
    queryFn: async () => {
      const response = await api.organizations.get(id!);
      return response.data;
    },
    enabled: !!id,
  });

  // Fetch organization users
  const { data: orgUsers = [] } = useQuery({
    queryKey: ['org-users', id],
    queryFn: async () => {
      const response = await api.organizations.users.list(id!);
      return response.data;
    },
    enabled: !!id,
  });

  const updateMutation = useMutation({
    mutationFn: (data: typeof formData) => api.organizations.update(id!, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['organization', id] });
      queryClient.invalidateQueries({ queryKey: ['organizations'] });
      setIsEditOpen(false);
      toast.success('Organization updated successfully');
    },
    onError: (error: any) => {
      toast.error(error.response?.data?.error || 'Failed to update organization');
    },
  });

  const deleteMutation = useMutation({
    mutationFn: () => api.organizations.delete(id!),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['organizations'] });
      toast.success('Organization deleted successfully');
      navigate('/organizations');
    },
    onError: (error: any) => {
      toast.error(error.response?.data?.error || 'Failed to delete organization');
    },
  });

  const toggleMutation = useMutation({
    mutationFn: () => api.organizations.toggle(id!),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['organization', id] });
      queryClient.invalidateQueries({ queryKey: ['organizations'] });
      toast.success('Organization status updated');
    },
    onError: (error: any) => {
      toast.error(error.response?.data?.error || 'Failed to update status');
    },
  });

  const createUserMutation = useMutation({
    mutationFn: (data: typeof userFormData) => api.organizations.users.create(id!, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['org-users', id] });
      setUserFormData({ email: '', name: '', password: '', role: 'org_user' });
      toast.success('User created successfully');
    },
    onError: (error: any) => {
      toast.error(error.response?.data?.error || 'Failed to create user');
    },
  });

  const resetPasswordMutation = useMutation({
    mutationFn: ({ userId, password }: { userId: string; password: string }) =>
      api.organizations.users.resetPassword(id!, userId, password),
    onSuccess: () => {
      toast.success('Password reset successfully');
    },
    onError: (error: any) => {
      toast.error(error.response?.data?.error || 'Failed to reset password');
    },
  });

  const deleteUserMutation = useMutation({
    mutationFn: (userId: string) => api.organizations.users.delete(id!, userId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['org-users', id] });
      toast.success('User deleted successfully');
    },
    onError: (error: any) => {
      toast.error(error.response?.data?.error || 'Failed to delete user');
    },
  });

  const handleEdit = () => {
    if (organization) {
      setFormData({
        name: organization.name,
        description: organization.description || '',
      });
      setIsEditOpen(true);
    }
  };

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    updateMutation.mutate(formData);
  };

  const handleDelete = () => {
    if (organization) {
      setDeleteDialog({ isOpen: true, id: organization.id, name: organization.name, type: 'org' });
    }
  };

  const confirmDelete = () => {
    if (deleteDialog.type === 'org') {
      deleteMutation.mutate();
    } else {
      deleteUserMutation.mutate(deleteDialog.id);
    }
    setDeleteDialog({ isOpen: false, id: '', name: '', type: 'org' });
  };

  const handleUserSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    createUserMutation.mutate(userFormData);
  };

  const handleResetPassword = (userId: string, userName: string) => {
    setResetPasswordDialog({ isOpen: true, userId, userName, newPassword: '' });
  };

  const confirmResetPassword = () => {
    if (resetPasswordDialog.newPassword.length < 8) {
      toast.error('Password must be at least 8 characters');
      return;
    }
    resetPasswordMutation.mutate({
      userId: resetPasswordDialog.userId,
      password: resetPasswordDialog.newPassword,
    });
    setResetPasswordDialog({ isOpen: false, userId: '', userName: '', newPassword: '' });
  };

  const handleDeleteUser = (userId: string, userName: string) => {
    setDeleteDialog({ isOpen: true, id: userId, name: userName, type: 'user' });
  };

  if (isLoading) {
    return (
      <Box sx={{ textAlign: 'center', py: 12 }}>
        <CircularProgress />
        <Typography variant="body2" color="text.secondary" sx={{ mt: 2 }}>
          Loading organization...
        </Typography>
      </Box>
    );
  }

  if (!organization) {
    return (
      <Box sx={{ textAlign: 'center', py: 12 }}>
        <Typography variant="h6" color="text.secondary">
          Organization not found
        </Typography>
        <Button onClick={() => navigate('/organizations')} sx={{ mt: 2 }}>
          Back to Organizations
        </Button>
      </Box>
    );
  }

  return (
    <Box sx={{ display: 'flex', flexDirection: 'column', gap: 3 }}>
      {/* Header */}
      <Box
        sx={{
          display: 'flex',
          flexDirection: { xs: 'column', md: 'row' },
          justifyContent: 'space-between',
          alignItems: { xs: 'stretch', md: 'flex-start' },
          gap: 2,
        }}
      >
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 2 }}>
          <IconButton onClick={() => navigate('/organizations')}>
            <ArrowLeft size={20} />
          </IconButton>
          <Box sx={{ flex: 1, minWidth: 0 }}>
            <Typography
              variant={isMobile ? 'h5' : 'h4'}
              sx={{ fontWeight: 700, wordBreak: 'break-word' }}
            >
              {organization.name}
            </Typography>
            <Typography variant="body2" color="text.secondary">
              Organization Management
            </Typography>
          </Box>
        </Box>
        <Box
          sx={{
            display: 'flex',
            flexDirection: { xs: 'column', sm: 'row' },
            alignItems: { xs: 'stretch', sm: 'center' },
            gap: 1,
          }}
        >
          <Chip
            label={organization.is_active ? 'Active' : 'Inactive'}
            color={organization.is_active ? 'success' : 'default'}
            variant="outlined"
            sx={{ alignSelf: { xs: 'flex-start', sm: 'center' } }}
          />
          <Button
            variant="outlined"
            startIcon={!isMobile && <Edit size={16} />}
            onClick={handleEdit}
            fullWidth={isMobile}
          >
            {isMobile ? 'Edit' : 'Edit'}
          </Button>
          <Button
            variant="outlined"
            startIcon={!isMobile && <Power size={16} />}
            onClick={() => toggleMutation.mutate()}
            fullWidth={isMobile}
          >
            {organization.is_active ? 'Deactivate' : 'Activate'}
          </Button>
          <Button
            variant="outlined"
            color="error"
            startIcon={!isMobile && <Trash2 size={16} />}
            onClick={handleDelete}
            fullWidth={isMobile}
          >
            {isMobile ? 'Delete' : 'Delete'}
          </Button>
        </Box>
      </Box>

      {/* Organization Details */}
      <Card>
        <CardContent sx={{ py: 3 }}>
          <Typography variant="h6" sx={{ mb: 2, fontWeight: 600 }}>
            Organization Details
          </Typography>
          <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', md: 'repeat(2, 1fr)' }, gap: 3 }}>
            <Box>
              <Typography variant="caption" color="text.secondary">
                Organization Name
              </Typography>
              <Typography variant="body1" sx={{ fontWeight: 500 }}>
                {organization.name}
              </Typography>
            </Box>
            <Box>
              <Typography variant="caption" color="text.secondary">
                Subdomain
              </Typography>
              <Typography variant="body1" sx={{ fontWeight: 500, fontFamily: 'monospace' }}>
                {organization.subdomain}
              </Typography>
            </Box>
            <Box>
              <Typography variant="caption" color="text.secondary" sx={{ display: 'block', mb: 1 }}>
                Full Domain
              </Typography>
              <Chip
                label={organization.full_domain}
                color="secondary"
                variant="outlined"
                size="small"
                sx={{ fontFamily: 'monospace', fontSize: '0.75rem', fontWeight: 500 }}
              />
            </Box>
            <Box>
              <Typography variant="caption" color="text.secondary">
                Created
              </Typography>
              <Typography variant="body1" sx={{ fontWeight: 500 }}>
                {formatRelativeTime(organization.created_at)}
              </Typography>
            </Box>
            {organization.description && (
              <Box sx={{ gridColumn: { xs: '1', md: '1 / -1' } }}>
                <Typography variant="caption" color="text.secondary">
                  Description
                </Typography>
                <Typography variant="body1">{organization.description}</Typography>
              </Box>
            )}
          </Box>
        </CardContent>
      </Card>

      {/* User Management */}
      <Card>
        <CardContent sx={{ py: 3 }}>
          <Typography variant="h6" sx={{ mb: 2, fontWeight: 600 }}>
            <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
              <Users size={20} />
              Users Management ({orgUsers.length})
            </Box>
          </Typography>

          {/* Create User Form */}
          <form onSubmit={handleUserSubmit}>
            <Paper variant="outlined" sx={{ p: 3, mb: 3 }}>
              <Typography variant="subtitle1" sx={{ fontWeight: 600, mb: 2 }}>
                Add New User
              </Typography>
              <Box
                sx={{
                  display: 'grid',
                  gridTemplateColumns: { xs: '1fr', sm: 'repeat(2, 1fr)' },
                  gap: 2,
                  mb: 2,
                }}
              >
                <TextField
                  fullWidth
                  label="Email"
                  type="email"
                  value={userFormData.email}
                  onChange={(e) => setUserFormData({ ...userFormData, email: e.target.value })}
                  required
                />
                <TextField
                  fullWidth
                  label="Name"
                  value={userFormData.name}
                  onChange={(e) => setUserFormData({ ...userFormData, name: e.target.value })}
                  required
                />
                <TextField
                  fullWidth
                  label="Password"
                  type="password"
                  value={userFormData.password}
                  onChange={(e) => setUserFormData({ ...userFormData, password: e.target.value })}
                  required
                />
                <FormControl fullWidth>
                  <InputLabel>Role</InputLabel>
                  <Select
                    value={userFormData.role}
                    label="Role"
                    onChange={(e) =>
                      setUserFormData({
                        ...userFormData,
                        role: e.target.value as 'org_admin' | 'org_user',
                      })
                    }
                  >
                    <MenuItem value="org_user">User</MenuItem>
                    <MenuItem value="org_admin">Admin</MenuItem>
                  </Select>
                </FormControl>
              </Box>
              <Button
                type="submit"
                variant="contained"
                startIcon={<UserPlus size={16} />}
                disabled={createUserMutation.isPending}
                sx={{ bgcolor: '#667eea', '&:hover': { bgcolor: '#5568d3' } }}
              >
                Create User
              </Button>
            </Paper>
          </form>

          {/* Users List */}
          {orgUsers.length === 0 ? (
            <Box sx={{ textAlign: 'center', py: 8 }}>
              <Typography variant="body2" color="text.secondary">
                No users yet. Add your first user above!
              </Typography>
            </Box>
          ) : (
            <Box sx={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
              {orgUsers.map((user: any) => (
                <Paper
                  key={user.id}
                  variant="outlined"
                  sx={{
                    p: 2,
                    '&:hover': { bgcolor: 'action.hover' },
                  }}
                >
                  <Box
                    sx={{
                      display: 'flex',
                      flexDirection: { xs: 'column', sm: 'row' },
                      alignItems: { xs: 'stretch', sm: 'center' },
                      justifyContent: 'space-between',
                      gap: 2,
                    }}
                  >
                    <Box sx={{ flex: 1, minWidth: 0 }}>
                      <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mb: 0.5 }}>
                        <Typography variant="body1" sx={{ fontWeight: 600 }}>
                          {user.name}
                        </Typography>
                        <Chip
                          label={user.role === 'org_admin' ? 'Admin' : 'User'}
                          color={user.role === 'org_admin' ? 'primary' : 'default'}
                          variant="outlined"
                          size="small"
                        />
                      </Box>
                      <Typography variant="body2" color="text.secondary" noWrap>
                        {user.email}
                      </Typography>
                      <Typography variant="caption" color="text.secondary">
                        Created {formatRelativeTime(user.created_at)}
                      </Typography>
                    </Box>
                    <Box sx={{ display: 'flex', gap: 1, alignSelf: { xs: 'flex-end', sm: 'auto' } }}>
                      <IconButton
                        size="small"
                        onClick={() => handleResetPassword(user.id, user.name)}
                        title="Reset Password"
                      >
                        <Key size={18} />
                      </IconButton>
                      <IconButton
                        size="small"
                        color="error"
                        onClick={() => handleDeleteUser(user.id, user.name)}
                        title="Delete User"
                      >
                        <Trash2 size={18} />
                      </IconButton>
                    </Box>
                  </Box>
                </Paper>
              ))}
            </Box>
          )}
        </CardContent>
      </Card>

      {/* Edit Organization Dialog */}
      <Dialog open={isEditOpen} onClose={() => setIsEditOpen(false)} maxWidth="sm" fullWidth fullScreen={isMobile}>
        <DialogTitle>Edit Organization</DialogTitle>
        <form onSubmit={handleSubmit}>
          <DialogContent>
            <Typography variant="body2" color="text.secondary" sx={{ mb: 3 }}>
              Update organization details
            </Typography>
            <Box sx={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
              <TextField
                fullWidth
                label="Organization Name"
                value={formData.name}
                onChange={(e) => setFormData({ ...formData, name: e.target.value })}
                required
              />
              <TextField
                fullWidth
                label="Description (optional)"
                value={formData.description}
                onChange={(e) => setFormData({ ...formData, description: e.target.value })}
                multiline
                rows={3}
              />
            </Box>
          </DialogContent>
          <DialogActions>
            <Button onClick={() => setIsEditOpen(false)}>Cancel</Button>
            <Button type="submit" variant="contained" disabled={updateMutation.isPending}>
              Update
            </Button>
          </DialogActions>
        </form>
      </Dialog>

      {/* Reset Password Dialog */}
      <Dialog
        open={resetPasswordDialog.isOpen}
        onClose={() =>
          setResetPasswordDialog({ isOpen: false, userId: '', userName: '', newPassword: '' })
        }
        maxWidth="xs"
        fullWidth
        fullScreen={isMobile}
      >
        <DialogTitle>Reset Password</DialogTitle>
        <DialogContent>
          <Typography variant="body2" color="text.secondary" sx={{ mb: 3 }}>
            Enter a new password for {resetPasswordDialog.userName}
          </Typography>
          <TextField
            fullWidth
            label="New Password"
            type="password"
            placeholder="Min 8 characters"
            value={resetPasswordDialog.newPassword}
            onChange={(e) =>
              setResetPasswordDialog({ ...resetPasswordDialog, newPassword: e.target.value })
            }
            onKeyDown={(e) => {
              if (e.key === 'Enter') {
                confirmResetPassword();
              }
            }}
          />
        </DialogContent>
        <DialogActions>
          <Button
            onClick={() =>
              setResetPasswordDialog({ isOpen: false, userId: '', userName: '', newPassword: '' })
            }
          >
            Cancel
          </Button>
          <Button variant="contained" onClick={confirmResetPassword}>
            Reset Password
          </Button>
        </DialogActions>
      </Dialog>

      {/* Delete Confirmation Dialog */}
      <Dialog
        open={deleteDialog.isOpen}
        onClose={() => setDeleteDialog({ isOpen: false, id: '', name: '', type: 'org' })}
        maxWidth="xs"
        fullWidth
        fullScreen={isMobile}
      >
        <DialogTitle>Confirm Deletion</DialogTitle>
        <DialogContent>
          <Typography variant="body2" color="text.secondary">
            Are you sure you want to delete {deleteDialog.type === 'org' ? 'organization' : 'user'}{' '}
            "{deleteDialog.name}"? This action cannot be undone.
          </Typography>
        </DialogContent>
        <DialogActions>
          <Button
            onClick={() => setDeleteDialog({ isOpen: false, id: '', name: '', type: 'org' })}
          >
            Cancel
          </Button>
          <Button variant="contained" color="error" onClick={confirmDelete}>
            Delete
          </Button>
        </DialogActions>
      </Dialog>
    </Box>
  );
}
