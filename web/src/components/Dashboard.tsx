import { useState } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import {
  Box,
  Drawer,
  List,
  ListItem,
  ListItemButton,
  ListItemIcon,
  ListItemText,
  Card,
  CardContent,
  Button,
  Chip,
  Typography,
  Paper,
  Avatar,
  Divider,
} from '@mui/material';
import {
  Activity,
  Globe,
  Download,
  Upload,
  LogOut,
  User,
  Sparkles,
  Building2,
  Settings,
  Users as UsersIcon,
  Webhook,
  Key,
} from 'lucide-react';
import { api } from '@/lib/api';
import { useAuth } from '@/contexts/AuthContext';
import { useTunnelEvents } from '@/hooks/useSSE';
import TunnelList from './TunnelList';
import TokenManager from './TokenManager';
import { WebhookApps } from './WebhookApps';
import OrganizationList from './OrganizationList';
import OrgUserManagement from './OrgUserManagement';

const DRAWER_WIDTH = 280;

function Dashboard() {
  const [activeTab, setActiveTab] = useState<
    'tunnels' | 'tokens' | 'webhooks' | 'organizations' | 'org-users'
  >('tunnels');
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
      // Also update stats summary
      queryClient.invalidateQueries({ queryKey: ['stats'] });
    } else {
      // For connect/disconnect events, do full refetch
      queryClient.invalidateQueries({ queryKey: ['tunnels'] });
      queryClient.invalidateQueries({ queryKey: ['stats'] });
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

  const { data: stats } = useQuery({
    queryKey: ['stats'],
    queryFn: async () => {
      const response = await api.stats.get();
      return response.data;
    },
    // Real-time updates via SSE - no need for polling
  });

  const formatBytes = (bytes: number) => {
    if (bytes === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
  };

  const statsCards = [
    {
      title: 'Active Tunnels',
      value: stats?.active_tunnels || 0,
      subtitle: `of ${stats?.total_tunnels || 0} total tunnels`,
      icon: Globe,
      color: '#667eea',
    },
    {
      title: 'Total Requests',
      value: (stats?.total_requests ?? 0).toLocaleString(),
      subtitle: 'All time requests',
      icon: Activity,
      color: '#667eea',
    },
    {
      title: 'Data Received',
      value: formatBytes(stats?.total_bytes_in || 0),
      subtitle: 'Inbound traffic',
      icon: Download,
      color: '#667eea',
    },
    {
      title: 'Data Sent',
      value: formatBytes(stats?.total_bytes_out || 0),
      subtitle: 'Outbound traffic',
      icon: Upload,
      color: '#667eea',
    },
  ];

  // Build navigation items based on role
  const navItems = [
    {
      id: 'tunnels' as const,
      label: 'Tunnels',
      icon: Globe,
    },
    {
      id: 'tokens' as const,
      label: 'Auth Tokens',
      icon: Key,
    },
    {
      id: 'webhooks' as const,
      label: 'Webhooks',
      icon: Webhook,
    },
    // Admin menu items (conditional based on role)
    ...(isSuperAdmin
      ? [
          {
            id: 'organizations' as const,
            label: 'Organizations',
            icon: Settings,
          },
        ]
      : []),
    ...(isOrgAdmin && !isSuperAdmin
      ? [
          {
            id: 'org-users' as const,
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
            const isActive = activeTab === item.id;
            return (
              <ListItem key={item.id} disablePadding sx={{ mb: 1 }}>
                <ListItemButton
                  selected={isActive}
                  onClick={() => setActiveTab(item.id)}
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
        }}
      >
        <Box sx={{ p: 4 }}>
          {/* Stats Cards */}
          <Box
            sx={{
              display: 'grid',
              gridTemplateColumns: {
                xs: '1fr',
                sm: 'repeat(2, 1fr)',
                lg: 'repeat(4, 1fr)',
              },
              gap: 3,
              mb: 4,
            }}
          >
            {statsCards.map((card, index) => {
              const IconComponent = card.icon;
              return (
                <Card
                  key={index}
                  elevation={2}
                  sx={{
                    transition: 'box-shadow 0.3s',
                    '&:hover': {
                      boxShadow: 6,
                    },
                  }}
                >
                  <CardContent>
                    <Box
                      sx={{
                        display: 'flex',
                        alignItems: 'center',
                        justifyContent: 'space-between',
                        mb: 2,
                      }}
                    >
                      <Typography variant="body2" color="text.secondary" fontWeight={500}>
                        {card.title}
                      </Typography>
                      <Avatar
                        sx={{
                          width: 40,
                          height: 40,
                          bgcolor: card.color,
                        }}
                      >
                        <IconComponent size={20} color="white" />
                      </Avatar>
                    </Box>
                    <Typography variant="h4" sx={{ fontWeight: 700, color: card.color, mb: 0.5 }}>
                      {card.value}
                    </Typography>
                    <Typography variant="caption" color="text.secondary">
                      {card.subtitle}
                    </Typography>
                  </CardContent>
                </Card>
              );
            })}
          </Box>

          {/* Content based on active tab */}
          <Box>
            {activeTab === 'tunnels' ? (
              <TunnelList />
            ) : activeTab === 'tokens' ? (
              <TokenManager />
            ) : activeTab === 'webhooks' ? (
              <WebhookApps />
            ) : activeTab === 'organizations' ? (
              <OrganizationList />
            ) : activeTab === 'org-users' ? (
              <OrgUserManagement organizationId={organizationId || ''} />
            ) : null}
          </Box>
        </Box>
      </Box>
    </Box>
  );
}

export default Dashboard;
