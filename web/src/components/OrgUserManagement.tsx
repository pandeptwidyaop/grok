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
import { UserPlus, Trash2, Key } from 'lucide-react';
import { toast } from 'sonner';
import { api } from '@/lib/api';
import { useAuth } from '@/contexts/AuthContext';

interface OrgUserManagementProps {
  organizationId: string;
}

export default function OrgUserManagement({ organizationId }: OrgUserManagementProps) {
  const queryClient = useQueryClient();
  const { organizationName } = useAuth();

  const [isCreateOpen, setIsCreateOpen] = useState(false);
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
  }>({ isOpen: false, id: '', name: '' });

  const { data: users = [], isLoading } = useQuery({
    queryKey: ['org-users', organizationId],
    queryFn: async () => {
      const response = await api.organizations.users.list(organizationId);
      return response.data;
    },
    enabled: !!organizationId,
  });

  const createUserMutation = useMutation({
    mutationFn: (data: typeof userFormData) =>
      api.organizations.users.create(organizationId, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['org-users', organizationId] });
      setIsCreateOpen(false);
      resetUserForm();
      toast.success('User created successfully');
    },
    onError: (error: any) => {
      toast.error(error.response?.data?.error || 'Failed to create user');
    },
  });

  const resetPasswordMutation = useMutation({
    mutationFn: ({ userId, password }: { userId: string; password: string }) =>
      api.organizations.users.resetPassword(organizationId, userId, password),
    onSuccess: () => {
      toast.success('Password reset successfully');
    },
    onError: (error: any) => {
      toast.error(error.response?.data?.error || 'Failed to reset password');
    },
  });

  const deleteUserMutation = useMutation({
    mutationFn: (userId: string) => api.organizations.users.delete(organizationId, userId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['org-users', organizationId] });
      toast.success('User deleted successfully');
    },
    onError: (error: any) => {
      toast.error(error.response?.data?.error || 'Failed to delete user');
    },
  });

  const resetUserForm = () => {
    setUserFormData({
      email: '',
      name: '',
      password: '',
      role: 'org_user',
    });
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
    setDeleteDialog({ isOpen: true, id: userId, name: userName });
  };

  const confirmDelete = () => {
    deleteUserMutation.mutate(deleteDialog.id);
    setDeleteDialog({ isOpen: false, id: '', name: '' });
  };

  if (isLoading) {
    return (
      <Card>
        <CardContent sx={{ py: 8 }}>
          <Box sx={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 2 }}>
            <CircularProgress />
            <Typography variant="body2" color="text.secondary">
              Loading users...
            </Typography>
          </Box>
        </CardContent>
      </Card>
    );
  }

  return (
    <Box sx={{ display: 'flex', flexDirection: 'column', gap: 3 }}>
      {/* Header */}
      <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
        <Box>
          <Typography variant="h4" sx={{ fontWeight: 700, mb: 0.5 }}>
            Manage Users
          </Typography>
          <Typography variant="body2" color="text.secondary">
            {organizationName}
          </Typography>
        </Box>
        <Button
          variant="contained"
          startIcon={<UserPlus size={16} />}
          onClick={() => setIsCreateOpen(true)}
          sx={{ bgcolor: '#667eea', '&:hover': { bgcolor: '#5568d3' } }}
        >
          Add User
        </Button>
      </Box>

      {/* Users List */}
      <Card>
        <CardContent sx={{ py: 4 }}>
          <Typography variant="h6" gutterBottom sx={{ fontWeight: 600, mb: 3 }}>
            Users ({users.length})
          </Typography>

          {users.length === 0 ? (
            <Box sx={{ textAlign: 'center', py: 8 }}>
              <Typography variant="body2" color="text.secondary">
                No users found
              </Typography>
            </Box>
          ) : (
            <Box sx={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
              {users.map((user: any) => (
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
        </CardContent>
      </Card>

      {/* Create User Dialog */}
      <Dialog open={isCreateOpen} onClose={() => setIsCreateOpen(false)} maxWidth="sm" fullWidth>
        <DialogTitle>Create New User</DialogTitle>
        <form onSubmit={handleUserSubmit}>
          <DialogContent>
            <Typography variant="body2" color="text.secondary" sx={{ mb: 3 }}>
              Add a new user to {organizationName}
            </Typography>
            <Box
              sx={{
                display: 'grid',
                gridTemplateColumns: { xs: '1fr', sm: 'repeat(2, 1fr)' },
                gap: 2,
              }}
            >
              <TextField
                fullWidth
                label="Email"
                type="email"
                required
                value={userFormData.email}
                onChange={(e) => setUserFormData({ ...userFormData, email: e.target.value })}
              />
              <TextField
                fullWidth
                label="Name"
                required
                value={userFormData.name}
                onChange={(e) => setUserFormData({ ...userFormData, name: e.target.value })}
              />
              <TextField
                fullWidth
                label="Password"
                type="password"
                required
                value={userFormData.password}
                onChange={(e) => setUserFormData({ ...userFormData, password: e.target.value })}
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
          </DialogContent>
          <DialogActions>
            <Button
              onClick={() => {
                setIsCreateOpen(false);
                resetUserForm();
              }}
            >
              Cancel
            </Button>
            <Button type="submit" variant="contained">
              Create User
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
        onClose={() => setDeleteDialog({ isOpen: false, id: '', name: '' })}
        maxWidth="xs"
        fullWidth
      >
        <DialogTitle>Confirm Deletion</DialogTitle>
        <DialogContent>
          <Typography variant="body2" color="text.secondary">
            Are you sure you want to delete user "{deleteDialog.name}"? This action cannot be
            undone.
          </Typography>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setDeleteDialog({ isOpen: false, id: '', name: '' })}>
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
