import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { api } from '@/lib/api';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Activity, Globe, Download, Upload } from 'lucide-react';
import TunnelList from './TunnelList';
import TokenManager from './TokenManager';

function Dashboard() {
  const [activeTab, setActiveTab] = useState<'tunnels' | 'tokens'>('tunnels');

  const { data: stats } = useQuery({
    queryKey: ['stats'],
    queryFn: async () => {
      const response = await api.stats.get();
      return response.data;
    },
    refetchInterval: 5000, // Refresh every 5 seconds
  });

  const formatBytes = (bytes: number) => {
    if (bytes === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
  };

  return (
    <div className="min-h-screen bg-background">
      <div className="container mx-auto px-4 py-8">
        {/* Header */}
        <div className="mb-8">
          <h1 className="text-4xl font-bold mb-2">Grok Dashboard</h1>
          <p className="text-muted-foreground">
            Manage your tunnels and authentication tokens
          </p>
        </div>

        {/* Stats Cards */}
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6 mb-8">
          <Card>
            <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
              <CardTitle className="text-sm font-medium">
                Active Tunnels
              </CardTitle>
              <Globe className="h-4 w-4 text-muted-foreground" />
            </CardHeader>
            <CardContent>
              <div className="text-2xl font-bold">
                {stats?.active_tunnels || 0}
              </div>
              <p className="text-xs text-muted-foreground">
                of {stats?.total_tunnels || 0} total
              </p>
            </CardContent>
          </Card>

          <Card>
            <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
              <CardTitle className="text-sm font-medium">
                Total Requests
              </CardTitle>
              <Activity className="h-4 w-4 text-muted-foreground" />
            </CardHeader>
            <CardContent>
              <div className="text-2xl font-bold">
                {stats?.total_requests?.toLocaleString() || 0}
              </div>
              <p className="text-xs text-muted-foreground">All time</p>
            </CardContent>
          </Card>

          <Card>
            <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
              <CardTitle className="text-sm font-medium">
                Data Received
              </CardTitle>
              <Download className="h-4 w-4 text-muted-foreground" />
            </CardHeader>
            <CardContent>
              <div className="text-2xl font-bold">
                {formatBytes(stats?.total_bytes_in || 0)}
              </div>
              <p className="text-xs text-muted-foreground">Inbound traffic</p>
            </CardContent>
          </Card>

          <Card>
            <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
              <CardTitle className="text-sm font-medium">Data Sent</CardTitle>
              <Upload className="h-4 w-4 text-muted-foreground" />
            </CardHeader>
            <CardContent>
              <div className="text-2xl font-bold">
                {formatBytes(stats?.total_bytes_out || 0)}
              </div>
              <p className="text-xs text-muted-foreground">Outbound traffic</p>
            </CardContent>
          </Card>
        </div>

        {/* Tabs */}
        <div className="mb-6">
          <div className="border-b border-border">
            <div className="flex gap-4">
              <button
                onClick={() => setActiveTab('tunnels')}
                className={`px-4 py-2 border-b-2 transition-colors ${
                  activeTab === 'tunnels'
                    ? 'border-primary text-primary'
                    : 'border-transparent text-muted-foreground hover:text-foreground'
                }`}
              >
                Tunnels
              </button>
              <button
                onClick={() => setActiveTab('tokens')}
                className={`px-4 py-2 border-b-2 transition-colors ${
                  activeTab === 'tokens'
                    ? 'border-primary text-primary'
                    : 'border-transparent text-muted-foreground hover:text-foreground'
                }`}
              >
                Auth Tokens
              </button>
            </div>
          </div>
        </div>

        {/* Content */}
        <div>{activeTab === 'tunnels' ? <TunnelList /> : <TokenManager />}</div>
      </div>
    </div>
  );
}

export default Dashboard;
