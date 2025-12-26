import { Routes, Route, Navigate, useNavigate, useLocation } from 'react-router-dom';
import { useQueryClient } from '@tanstack/react-query';
import {
  Box,
  Drawer,
  List,
  ListItem,
  ListItemButton,
  ListItemIcon,
  ListItemText,
  Button,
  Chip,
  Typography,
  Paper,
  Divider,
} from '@mui/material';
import {
  LayoutDashboard,
  Globe,
  LogOut,
  User,
  Sparkles,
  Building2,
  Settings,
  Users as UsersIcon,
  Webhook,
  Key,
} from 'lucide-react';
import { useAuth } from '@/contexts/AuthContext';
import { useTunnelEvents } from '@/hooks/useSSE';
import Dashboard from './Dashboard';
import TunnelList from './TunnelList';
import TunnelDetail from './TunnelDetail';
import TokenManager from './TokenManager';
import { WebhookApps } from './WebhookApps';
import { WebhookAppDetailPage } from './WebhookAppDetailPage';
import OrganizationList from './OrganizationList';
import OrgUserManagement from './OrgUserManagement';

const DRAWER_WIDTH = 280;

function MainLayout() {
  const navigate = useNavigate();
  const location = useLocation();
  const { user, role, organizationName, organizationId, isSuperAdmin, isOrgAdmin, logout } =
    useAuth();
  const queryClient = useQueryClient();

  // Subscribe to real-time tunnel events via SSE
  useTunnelEvents((event) => {
    // Handle different event types efficiently
    if (event.type === 'tunnel_stats_updated') {
      // For stats updates, update cache directly (no refetch needed)
      queryClient.setQueryData(['tunnels'], (oldData: any) => {
        if (!oldData) return oldData;
        return oldData.map((tunnel: any) =>
          tunnel.id === event.data.tunnel_id
            ? { ...tunnel, ...event.data.tunnel }
            : tunnel
        );
      });
      // Force refetch stats (invalidateQueries doesn't work with staleTime: Infinity)
      queryClient.refetchQueries({ queryKey: ['stats'] });
    } else {
      // For connect/disconnect events, force refetch
      queryClient.refetchQueries({ queryKey: ['tunnels'] });
      queryClient.refetchQueries({ queryKey: ['stats'] });
    }
  });

  const getRoleBadge = () => {
    const roleColors = {
      super_admin: 'secondary',
      org_admin: 'primary',
      org_user: 'success',
    } as const;
    const roleLabels = {
      super_admin: 'Super Admin',
      org_admin: 'Org Admin',
      org_user: 'User',
    };
    if (!role) return null;
    return <Chip label={roleLabels[role]} color={roleColors[role]} size="small" />;
  };

  // Build navigation items based on role
  const navItems = [
    {
      id: 'dashboard',
      path: '/',
      label: 'Dashboard',
      icon: LayoutDashboard,
    },
    {
      id: 'tunnels',
      path: '/tunnels',
      label: 'Tunnels',
      icon: Globe,
    },
    {
      id: 'tokens',
      path: '/tokens',
      label: 'Auth Tokens',
      icon: Key,
    },
    {
      id: 'webhooks',
      path: '/webhooks',
      label: 'Webhooks',
      icon: Webhook,
    },
    // Admin menu items (conditional based on role)
    ...(isSuperAdmin
      ? [
          {
            id: 'organizations',
            path: '/organizations',
            label: 'Organizations',
            icon: Settings,
          },
        ]
      : []),
    ...(isOrgAdmin && !isSuperAdmin
      ? [
          {
            id: 'org-users',
            path: '/org-users',
            label: 'Manage Users',
            icon: UsersIcon,
          },
        ]
      : []),
  ];

  return (
    <Box sx={{ display: 'flex', minHeight: '100vh' }}>
      {/* Left Sidebar */}
      <Drawer
        variant="permanent"
        sx={{
          width: DRAWER_WIDTH,
          flexShrink: 0,
          '& .MuiDrawer-paper': {
            width: DRAWER_WIDTH,
            boxSizing: 'border-box',
            background: 'linear-gradient(180deg, #667eea 0%, #764ba2 100%)',
            color: 'white',
            borderRight: 'none',
          },
        }}
      >
        {/* Logo/Brand */}
        <Box sx={{ p: 3 }}>
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 2, mb: 1 }}>
            <Paper
              elevation={0}
              sx={{
                width: 40,
                height: 40,
                borderRadius: 2,
                bgcolor: 'rgba(255, 255, 255, 0.2)',
                backdropFilter: 'blur(10px)',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
              }}
            >
              <Sparkles size={20} color="white" />
            </Paper>
            <Typography variant="h6" sx={{ fontWeight: 700, color: 'white' }}>
              Grok
            </Typography>
          </Box>
          <Typography variant="caption" sx={{ color: 'rgba(255, 255, 255, 0.7)', display: 'block' }}>
            Tunneling System
          </Typography>
        </Box>

        <Divider sx={{ borderColor: 'rgba(255, 255, 255, 0.1)' }} />

        {/* Navigation Menu */}
        <List sx={{ flex: 1, px: 2, py: 2 }}>
          {navItems.map((item) => {
            const Icon = item.icon;
            // For /tunnels and /webhooks, match both exact and sub-routes
            const isActive = (item.path === '/tunnels' || item.path === '/webhooks')
              ? location.pathname.startsWith(item.path)
              : location.pathname === item.path;
            return (
              <ListItem key={item.id} disablePadding sx={{ mb: 1 }}>
                <ListItemButton
                  selected={isActive}
                  onClick={() => navigate(item.path)}
                  sx={{
                    borderRadius: 2,
                    color: isActive ? 'white' : 'rgba(255, 255, 255, 0.7)',
                    '&.Mui-selected': {
                      bgcolor: 'rgba(255, 255, 255, 0.15)',
                      '&:hover': {
                        bgcolor: 'rgba(255, 255, 255, 0.2)',
                      },
                    },
                    '&:hover': {
                      bgcolor: 'rgba(255, 255, 255, 0.1)',
                    },
                  }}
                >
                  <ListItemIcon sx={{ color: 'inherit', minWidth: 40 }}>
                    <Icon size={20} />
                  </ListItemIcon>
                  <ListItemText
                    primary={item.label}
                    primaryTypographyProps={{
                      fontSize: '0.95rem',
                      fontWeight: isActive ? 600 : 400,
                    }}
                  />
                </ListItemButton>
              </ListItem>
            );
          })}
        </List>

        <Divider sx={{ borderColor: 'rgba(255, 255, 255, 0.1)' }} />

        {/* User Info & Actions */}
        <Box sx={{ p: 2 }}>
          {/* User Info */}
          <Box
            sx={{
              bgcolor: 'rgba(255, 255, 255, 0.1)',
              backdropFilter: 'blur(10px)',
              borderRadius: 2,
              p: 2,
              mb: 2,
            }}
          >
            <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mb: 1 }}>
              <User size={16} />
              <Typography variant="body2" sx={{ fontWeight: 500, flex: 1, color: 'white' }}>
                {user}
              </Typography>
            </Box>
            {getRoleBadge()}
            {organizationName && (
              <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mt: 1.5 }}>
                <Building2 size={14} />
                <Typography variant="caption" sx={{ color: 'rgba(255, 255, 255, 0.9)' }}>
                  {organizationName}
                </Typography>
              </Box>
            )}
          </Box>

          {/* Logout Button */}
          <Button
            fullWidth
            variant="outlined"
            size="small"
            onClick={logout}
            sx={{
              color: 'white',
              borderColor: 'rgba(255, 255, 255, 0.3)',
              '&:hover': {
                borderColor: 'rgba(255, 255, 255, 0.5)',
                bgcolor: 'rgba(255, 255, 255, 0.1)',
              },
            }}
            startIcon={<LogOut size={16} />}
          >
            Logout
          </Button>
        </Box>
      </Drawer>

      {/* Main Content */}
      <Box
        component="main"
        sx={{
          flexGrow: 1,
          bgcolor: 'background.default',
          minHeight: '100vh',
          p: 4,
        }}
      >
        <Routes>
          <Route path="/" element={<Dashboard />} />
          <Route path="/tunnels/:id" element={<TunnelDetail />} />
          <Route path="/tunnels" element={<TunnelList />} />
          <Route path="/tokens" element={<TokenManager />} />
          <Route path="/webhooks/:id" element={<WebhookAppDetailPage />} />
          <Route path="/webhooks" element={<WebhookApps />} />
          {isSuperAdmin && (
            <Route path="/organizations" element={<OrganizationList />} />
          )}
          {isOrgAdmin && (
            <Route
              path="/org-users"
              element={<OrgUserManagement organizationId={organizationId || ''} />}
            />
          )}
          <Route path="*" element={<Navigate to="/" replace />} />
        </Routes>
      </Box>
    </Box>
  );
}

export default MainLayout;
