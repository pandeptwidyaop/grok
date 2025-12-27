import { Routes, Route, Navigate, useNavigate, useLocation } from 'react-router-dom';
import { useQueryClient, useQuery } from '@tanstack/react-query';
import { useState } from 'react';
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
  Alert,
  IconButton,
  Collapse,
  AppBar,
  Toolbar,
  useMediaQuery,
  useTheme,
} from '@mui/material';
import {
  LayoutDashboard,
  Globe,
  LogOut,
  User,
  Building2,
  Settings as SettingsIcon,
  Users as UsersIcon,
  Webhook,
  Key,
  Download,
  ChevronUp,
  ChevronDown,
  Github,
  Menu,
} from 'lucide-react';
import { useAuth } from '@/contexts/AuthContext';
import { useTunnelEvents } from '@/hooks/useSSE';
import { api } from '@/lib/api';
import Dashboard from './Dashboard';
import TunnelList from './TunnelList';
import TunnelDetail from './TunnelDetail';
import TokenManager from './TokenManager';
import { WebhookApps } from './WebhookApps';
import { WebhookAppDetailPage } from './WebhookAppDetailPage';
import OrganizationList from './OrganizationList';
import OrganizationDetail from './OrganizationDetail';
import OrgUserManagement from './OrgUserManagement';
import Settings from './Settings';

const DRAWER_WIDTH = 280;

function MainLayout() {
  const navigate = useNavigate();
  const location = useLocation();
  const { user, role, organizationName, organizationId, isSuperAdmin, isOrgAdmin, logout } =
    useAuth();
  const queryClient = useQueryClient();
  const theme = useTheme();
  const isMobile = useMediaQuery(theme.breakpoints.down('md'));
  const [mobileOpen, setMobileOpen] = useState(false);
  const [showUpdateDetails, setShowUpdateDetails] = useState(false);

  const handleDrawerToggle = () => {
    setMobileOpen(!mobileOpen);
  };

  // Fetch version info
  const { data: versionInfo } = useQuery({
    queryKey: ['version'],
    queryFn: async () => {
      const response = await api.version.getVersion();
      return response.data;
    },
    staleTime: Infinity, // Version doesn't change during runtime
  });

  // Check for updates (every 6 hours)
  const { data: updateInfo } = useQuery({
    queryKey: ['update-check'],
    queryFn: async () => {
      const response = await api.version.checkUpdates();
      return response.data;
    },
    refetchInterval: 6 * 60 * 60 * 1000, // 6 hours
    staleTime: 6 * 60 * 60 * 1000,
  });

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
    {
      id: 'settings',
      path: '/settings',
      label: 'Settings',
      icon: SettingsIcon,
    },
    // Admin menu items (conditional based on role)
    ...(isSuperAdmin
      ? [
          {
            id: 'organizations',
            path: '/organizations',
            label: 'Organizations',
            icon: Building2,
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

  // Drawer content (reused for both mobile and desktop)
  const drawerContent = (
    <Box sx={{ height: '100%', display: 'flex', flexDirection: 'column' }}>
        {/* Logo/Brand */}
        <Box sx={{ p: 3 }}>
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 2, mb: 1 }}>
            <Paper
              elevation={0}
              sx={{
                width: 48,
                height: 48,
                borderRadius: 2,
                bgcolor: 'rgba(255, 255, 255, 0.15)',
                backdropFilter: 'blur(10px)',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                p: 1,
              }}
            >
              <img
                src="/favicon.svg"
                alt="Grok Logo"
                style={{ width: '100%', height: '100%' }}
              />
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
                  onClick={() => {
                    navigate(item.path);
                    if (isMobile) {
                      setMobileOpen(false);
                    }
                  }}
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

          {/* Version & Update Notification */}
          <Box sx={{ mt: 2 }}>
            {updateInfo?.update_available && (
              <Alert
                severity="info"
                sx={{
                  mb: 1.5,
                  bgcolor: 'rgba(33, 150, 243, 0.1)',
                  color: 'white',
                  border: '1px solid rgba(33, 150, 243, 0.3)',
                  '& .MuiAlert-icon': {
                    color: '#42a5f5',
                  },
                }}
                action={
                  <IconButton
                    size="small"
                    onClick={() => setShowUpdateDetails(!showUpdateDetails)}
                    sx={{ color: 'white' }}
                  >
                    {showUpdateDetails ? <ChevronUp size={16} /> : <ChevronDown size={16} />}
                  </IconButton>
                }
              >
                <Typography variant="caption" sx={{ fontWeight: 600 }}>
                  Update Available: {updateInfo.latest_version}
                </Typography>
              </Alert>
            )}

            <Collapse in={showUpdateDetails && updateInfo?.update_available}>
              <Box
                sx={{
                  bgcolor: 'rgba(255, 255, 255, 0.05)',
                  borderRadius: 1,
                  p: 1.5,
                  mb: 1.5,
                }}
              >
                <Typography variant="caption" sx={{ color: 'rgba(255, 255, 255, 0.9)', display: 'block', mb: 1 }}>
                  New version {updateInfo?.latest_version} is available!
                </Typography>
                <Button
                  fullWidth
                  size="small"
                  variant="contained"
                  component="a"
                  href={updateInfo?.release_url || ''}
                  target="_blank"
                  rel="noopener noreferrer"
                  sx={{
                    bgcolor: '#42a5f5',
                    '&:hover': { bgcolor: '#1e88e5' },
                  }}
                  startIcon={<Download size={14} />}
                >
                  Download Update
                </Button>
              </Box>
            </Collapse>

            <Box sx={{ textAlign: 'center' }}>
              <Typography
                variant="caption"
                sx={{
                  color: 'rgba(255, 255, 255, 0.5)',
                  display: 'block',
                  mb: 0.5,
                }}
              >
                v{versionInfo?.version || 'dev'}
              </Typography>
              <Box
                component="a"
                href="https://github.com/pandeptwidyaop/grok"
                target="_blank"
                rel="noopener noreferrer"
                sx={{
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                  gap: 0.5,
                  color: 'rgba(255, 255, 255, 0.4)',
                  textDecoration: 'none',
                  '&:hover': {
                    color: 'rgba(255, 255, 255, 0.7)',
                  },
                }}
              >
                <Github size={12} />
                <Typography
                  variant="caption"
                  sx={{
                    color: 'inherit',
                  }}
                >
                  GitHub
                </Typography>
              </Box>
            </Box>
          </Box>
        </Box>
    </Box>
  );

  return (
    <Box sx={{ display: 'flex', minHeight: '100vh' }}>
      {/* Mobile App Bar */}
      {isMobile && (
        <AppBar
          position="fixed"
          sx={{
            zIndex: theme.zIndex.drawer + 1,
            background: 'linear-gradient(90deg, #667eea 0%, #764ba2 100%)',
          }}
        >
          <Toolbar>
            <IconButton
              color="inherit"
              aria-label="open drawer"
              edge="start"
              onClick={handleDrawerToggle}
              sx={{ mr: 2 }}
            >
              <Menu size={24} />
            </IconButton>
            <Typography variant="h6" noWrap component="div" sx={{ flexGrow: 1 }}>
              Grok
            </Typography>
          </Toolbar>
        </AppBar>
      )}

      {/* Left Sidebar - Responsive Drawer */}
      <Drawer
        variant={isMobile ? 'temporary' : 'permanent'}
        open={isMobile ? mobileOpen : true}
        onClose={handleDrawerToggle}
        ModalProps={{
          keepMounted: true, // Better mobile performance
        }}
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
        {drawerContent}
      </Drawer>

      {/* Main Content */}
      <Box
        component="main"
        sx={{
          flexGrow: 1,
          bgcolor: 'background.default',
          minHeight: '100vh',
          p: { xs: 2, sm: 3, md: 4 },
          mt: { xs: 8, md: 0 }, // Account for mobile app bar
        }}
      >
        <Routes>
          <Route path="/" element={<Dashboard />} />
          <Route path="/tunnels/:id" element={<TunnelDetail />} />
          <Route path="/tunnels" element={<TunnelList />} />
          <Route path="/tokens" element={<TokenManager />} />
          <Route path="/webhooks/:id" element={<WebhookAppDetailPage />} />
          <Route path="/webhooks" element={<WebhookApps />} />
          <Route path="/settings" element={<Settings />} />
          {isSuperAdmin && (
            <>
              <Route path="/organizations" element={<OrganizationList />} />
              <Route path="/organizations/:id" element={<OrganizationDetail />} />
            </>
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
