import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
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
} from '@mui/material';
import { Building2, Plus, Edit, Trash2, Power, Users, UserPlus, Key } from 'lucide-react';
import { toast } from 'sonner';
import { api, type Organization } from '@/lib/api';

export default function OrganizationList() {
  const queryClient = useQueryClient();
  const [isCreateOpen, setIsCreateOpen] = useState(false);
  const [editingOrg, setEditingOrg] = useState<Organization | null>(null);
  const [managingUsersOrg, setManagingUsersOrg] = useState<Organization | null>(null);
  const [formData, setFormData] = useState({
    name: '',
    subdomain: '',
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

  const updateMutation = useMutation({
    mutationFn: ({ id, data }: { id: string; data: Partial<typeof formData> }) =>
      api.organizations.update(id, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['organizations'] });
      setEditingOrg(null);
      resetForm();
      toast.success('Organization updated successfully');
    },
    onError: (error: any) => {
      toast.error(error.response?.data?.error || 'Failed to update organization');
    },
  });

  const deleteMutation = useMutation({
    mutationFn: (id: string) => api.organizations.delete(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['organizations'] });
      toast.success('Organization deleted successfully');
    },
    onError: (error: any) => {
      toast.error(error.response?.data?.error || 'Failed to delete organization');
    },
  });

  const toggleMutation = useMutation({
    mutationFn: (id: string) => api.organizations.toggle(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['organizations'] });
      toast.success('Organization status updated');
    },
    onError: (error: any) => {
      toast.error(error.response?.data?.error || 'Failed to update status');
    },
  });

  // User management queries and mutations
  const { data: orgUsers = [] } = useQuery({
    queryKey: ['org-users', managingUsersOrg?.id],
    queryFn: async () => {
      if (!managingUsersOrg) return [];
      const response = await api.organizations.users.list(managingUsersOrg.id);
      return response.data;
    },
    enabled: !!managingUsersOrg,
  });

  const createUserMutation = useMutation({
    mutationFn: (data: typeof userFormData) =>
      api.organizations.users.create(managingUsersOrg!.id, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['org-users', managingUsersOrg?.id] });
      setUserFormData({ email: '', name: '', password: '', role: 'org_user' });
      toast.success('User created successfully');
    },
    onError: (error: any) => {
      toast.error(error.response?.data?.error || 'Failed to create user');
    },
  });

  const resetPasswordMutation = useMutation({
    mutationFn: ({ userId, password }: { userId: string; password: string }) =>
      api.organizations.users.resetPassword(managingUsersOrg!.id, userId, password),
    onSuccess: () => {
      toast.success('Password reset successfully');
    },
    onError: (error: any) => {
      toast.error(error.response?.data?.error || 'Failed to reset password');
    },
  });

  const deleteUserMutation = useMutation({
    mutationFn: (userId: string) => api.organizations.users.delete(managingUsersOrg!.id, userId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['org-users', managingUsersOrg?.id] });
      toast.success('User deleted successfully');
    },
    onError: (error: any) => {
      toast.error(error.response?.data?.error || 'Failed to delete user');
    },
  });

  const resetForm = () => {
    setFormData({ name: '', subdomain: '', description: '' });
  };

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (editingOrg) {
      updateMutation.mutate({ id: editingOrg.id, data: formData });
    } else {
      createMutation.mutate(formData);
    }
  };

  const handleEdit = (org: Organization) => {
    setEditingOrg(org);
    setFormData({
      name: org.name,
      subdomain: org.subdomain,
      description: org.description || '',
    });
  };

  const handleDelete = (id: string, name: string) => {
    setDeleteDialog({ isOpen: true, id, name, type: 'org' });
  };

  const confirmDelete = () => {
    if (deleteDialog.type === 'org') {
      deleteMutation.mutate(deleteDialog.id);
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

      {/* Organizations Grid */}
      {isLoading ? (
        <Box sx={{ textAlign: 'center', py: 12 }}>
          <CircularProgress />
          <Typography variant="body2" color="text.secondary" sx={{ mt: 2 }}>
            Loading organizations...
          </Typography>
        </Box>
      ) : organizations.length === 0 ? (
        <Card>
          <CardContent sx={{ py: 12, textAlign: 'center' }}>
            <Building2 size={48} style={{ color: '#9e9e9e', opacity: 0.5, margin: '0 auto 16px' }} />
            <Typography variant="body2" color="text.secondary">
              No organizations yet. Create your first one!
            </Typography>
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
          {organizations.map((org) => (
            <Card
              key={org.id}
              sx={{
                transition: 'box-shadow 0.3s',
                '&:hover': { boxShadow: 6 },
              }}
            >
              <CardContent>
                <Box sx={{ mb: 2 }}>
                  <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mb: 1 }}>
                    <Building2 size={20} style={{ color: '#667eea' }} />
                    <Typography variant="h6" sx={{ fontWeight: 600 }}>
                      {org.name}
                    </Typography>
                  </Box>
                  <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, flexWrap: 'wrap' }}>
                    <Chip
                      label={org.full_domain}
                      size="small"
                      sx={{ fontFamily: 'monospace', fontSize: '0.75rem' }}
                    />
                    <Chip
                      label={org.is_active ? 'Active' : 'Inactive'}
                      color={org.is_active ? 'success' : 'default'}
                      size="small"
                    />
                  </Box>
                </Box>

                {org.description && (
                  <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
                    {org.description}
                  </Typography>
                )}

                <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mb: 2 }}>
                  <Users size={12} />
                  <Typography variant="caption" color="text.secondary">
                    Created {new Date(org.created_at).toLocaleDateString()}
                  </Typography>
                </Box>

                <Box sx={{ display: 'flex', gap: 1 }}>
                  <Button
                    size="small"
                    variant="outlined"
                    onClick={() => setManagingUsersOrg(org)}
                    sx={{ flex: 1 }}
                    startIcon={<UserPlus size={12} />}
                  >
                    Users
                  </Button>
                  <Button
                    size="small"
                    variant="outlined"
                    onClick={() => handleEdit(org)}
                    sx={{ flex: 1 }}
                    startIcon={<Edit size={12} />}
                  >
                    Edit
                  </Button>
                  <IconButton
                    size="small"
                    onClick={() => toggleMutation.mutate(org.id)}
                    sx={{ border: 1, borderColor: 'divider' }}
                  >
                    <Power size={12} />
                  </IconButton>
                  <IconButton
                    size="small"
                    color="error"
                    onClick={() => handleDelete(org.id, org.name)}
                    sx={{ border: 1, borderColor: 'divider' }}
                  >
                    <Trash2 size={12} />
                  </IconButton>
                </Box>
              </CardContent>
            </Card>
          ))}
        </Box>
      )}

      {/* Create/Edit Organization Dialog */}
      <Dialog
        open={isCreateOpen || !!editingOrg}
        onClose={() => {
          setIsCreateOpen(false);
          setEditingOrg(null);
          resetForm();
        }}
        maxWidth="sm"
        fullWidth
      >
        <DialogTitle>{editingOrg ? 'Edit Organization' : 'Create Organization'}</DialogTitle>
        <form onSubmit={handleSubmit}>
          <DialogContent>
            <Typography variant="body2" color="text.secondary" sx={{ mb: 3 }}>
              {editingOrg
                ? 'Update organization details'
                : 'Add a new organization to the system'}
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
                disabled={!!editingOrg}
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
                setEditingOrg(null);
                resetForm();
              }}
            >
              Cancel
            </Button>
            <Button
              type="submit"
              variant="contained"
              disabled={createMutation.isPending || updateMutation.isPending}
            >
              {editingOrg ? 'Update' : 'Create'}
            </Button>
          </DialogActions>
        </form>
      </Dialog>

      {/* User Management Dialog */}
      <Dialog
        open={!!managingUsersOrg}
        onClose={() => setManagingUsersOrg(null)}
        maxWidth="md"
        fullWidth
      >
        <DialogTitle>Manage Users - {managingUsersOrg?.name}</DialogTitle>
        <DialogContent>
          <Typography variant="body2" color="text.secondary" sx={{ mb: 3 }}>
            Add and manage users for {managingUsersOrg?.full_domain}
          </Typography>

          {/* Create User Form */}
          <form onSubmit={handleUserSubmit}>
            <Box sx={{ borderBottom: 1, borderColor: 'divider', pb: 3, mb: 3 }}>
              <Typography variant="h6" sx={{ fontWeight: 600, mb: 2 }}>
                Create New User
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
              >
                Create User
              </Button>
            </Box>
          </form>

          {/* Users List */}
          <Box>
            <Typography variant="h6" sx={{ fontWeight: 600, mb: 2 }}>
              Existing Users ({orgUsers.length})
            </Typography>
            {orgUsers.length === 0 ? (
              <Typography variant="body2" color="text.secondary">
                No users yet
              </Typography>
            ) : (
              <Box sx={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
                {orgUsers.map((user: any) => (
                  <Box
                    key={user.id}
                    sx={{
                      display: 'flex',
                      flexDirection: { xs: 'column', sm: 'row' },
                      alignItems: { xs: 'stretch', sm: 'center' },
                      justifyContent: 'space-between',
                      gap: 2,
                      p: 2,
                      border: 1,
                      borderColor: 'divider',
                      borderRadius: 2,
                      '&:hover': { bgcolor: 'action.hover' },
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
                          size="small"
                        />
                      </Box>
                      <Typography variant="body2" color="text.secondary" noWrap>
                        {user.email}
                      </Typography>
                      <Typography variant="caption" color="text.secondary">
                        Created {new Date(user.created_at).toLocaleDateString()}
                      </Typography>
                    </Box>
                    <Box sx={{ display: 'flex', gap: 1, alignSelf: { xs: 'flex-end', sm: 'auto' } }}>
                      <IconButton
                        size="small"
                        onClick={() => handleResetPassword(user.id, user.name)}
                        title="Reset Password"
                        sx={{ border: 1, borderColor: 'divider' }}
                      >
                        <Key size={16} />
                      </IconButton>
                      <IconButton
                        size="small"
                        color="error"
                        onClick={() => handleDeleteUser(user.id, user.name)}
                        title="Delete User"
                        sx={{ border: 1, borderColor: 'divider' }}
                      >
                        <Trash2 size={16} />
                      </IconButton>
                    </Box>
                  </Box>
                ))}
              </Box>
            )}
          </Box>
        </DialogContent>
      </Dialog>

      {/* Reset Password Dialog */}
      <Dialog
        open={resetPasswordDialog.isOpen}
        onClose={() =>
          setResetPasswordDialog({ isOpen: false, userId: '', userName: '', newPassword: '' })
        }
        maxWidth="xs"
        fullWidth
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
