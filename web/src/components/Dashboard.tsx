import { useQuery } from '@tanstack/react-query';
import {
  Box,
  Card,
  CardContent,
  Typography,
  Avatar,
} from '@mui/material';
import {
  Activity,
  Globe,
  Download,
  Upload,
} from 'lucide-react';
import { api } from '@/lib/api';
import GettingStarted from './GettingStarted';

function Dashboard() {
  const { data: stats } = useQuery({
    queryKey: ['stats'],
    queryFn: async () => {
      const response = await api.stats.get();
      return response.data;
    },
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

  return (
    <Box>
      <Box sx={{ mb: 4 }}>
        <Typography variant="h4" sx={{ fontWeight: 700, color: '#667eea', mb: 1 }}>
          Dashboard
        </Typography>
        <Typography variant="body1" color="text.secondary">
          Overview of your tunneling system
        </Typography>
      </Box>

      {/* Stats Cards */}
      <Box
        sx={{
          display: 'grid',
          gridTemplateColumns: {
            xs: '1fr',
            sm: 'repeat(2, 1fr)',
            md: 'repeat(3, 1fr)',
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

      {/* Getting Started Tutorial */}
      <GettingStarted />
    </Box>
  );
}

export default Dashboard;
