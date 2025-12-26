import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { api } from '@/lib/api';
import { useAuth } from '@/contexts/AuthContext';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Activity, Globe, Download, Upload, LogOut, User, Sparkles } from 'lucide-react';
import TunnelList from './TunnelList';
import TokenManager from './TokenManager';

function Dashboard() {
  const [activeTab, setActiveTab] = useState<'tunnels' | 'tokens'>('tunnels');
  const { user, logout } = useAuth();

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
    <div className="min-h-screen bg-gradient-to-br from-gray-50 via-white to-gray-50">
      {/* Gradient Header */}
      <div className="gradient-primary relative overflow-hidden">
        <div className="absolute inset-0 bg-grid-white/10"></div>
        <div className="container mx-auto px-4 py-12 relative">
          <div className="flex items-start justify-between">
            <div className="animate-fade-in">
              <div className="flex items-center gap-3 mb-3">
                <div className="w-12 h-12 rounded-xl bg-white/20 backdrop-blur-sm flex items-center justify-center">
                  <Sparkles className="h-6 w-6 text-white" />
                </div>
                <h1 className="text-4xl font-bold text-white">Grok Dashboard</h1>
              </div>
              <p className="text-white/90 text-lg">
                Manage your tunnels and authentication tokens
              </p>
            </div>
            <div className="flex items-center gap-4 animate-fade-in">
              <div className="flex items-center gap-2 text-white/90 bg-white/10 backdrop-blur-sm px-4 py-2 rounded-full">
                <User className="h-4 w-4" />
                <span className="font-medium">{user}</span>
              </div>
              <Button
                variant="outline"
                size="sm"
                onClick={logout}
                className="bg-white/10 hover:bg-white/20 text-white border-white/20 backdrop-blur-sm"
              >
                <LogOut className="h-4 w-4 mr-2" />
                Logout
              </Button>
            </div>
          </div>
        </div>
      </div>

      <div className="container mx-auto px-4 -mt-8">
        {/* Stats Cards */}
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6 mb-8">
          <Card className="border-0 shadow-lg hover:shadow-xl transition-shadow animate-slide-up">
            <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
              <CardTitle className="text-sm font-medium text-muted-foreground">
                Active Tunnels
              </CardTitle>
              <div className="w-10 h-10 rounded-lg bg-blue-500 flex items-center justify-center">
                <Globe className="h-5 w-5 text-white" />
              </div>
            </CardHeader>
            <CardContent>
              <div className="text-3xl font-bold text-blue-600">
                {stats?.active_tunnels || 0}
              </div>
              <p className="text-xs text-muted-foreground mt-1">
                of {stats?.total_tunnels || 0} total tunnels
              </p>
            </CardContent>
          </Card>

          <Card className="border-0 shadow-lg hover:shadow-xl transition-shadow animate-slide-up" style={{animationDelay: '0.1s'}}>
            <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
              <CardTitle className="text-sm font-medium text-muted-foreground">
                Total Requests
              </CardTitle>
              <div className="w-10 h-10 rounded-lg bg-blue-500 flex items-center justify-center">
                <Activity className="h-5 w-5 text-white" />
              </div>
            </CardHeader>
            <CardContent>
              <div className="text-3xl font-bold text-blue-600">
                {(stats?.total_requests ?? 0).toLocaleString()}
              </div>
              <p className="text-xs text-muted-foreground mt-1">All time requests</p>
            </CardContent>
          </Card>

          <Card className="border-0 shadow-lg hover:shadow-xl transition-shadow animate-slide-up" style={{animationDelay: '0.2s'}}>
            <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
              <CardTitle className="text-sm font-medium text-muted-foreground">
                Data Received
              </CardTitle>
              <div className="w-10 h-10 rounded-lg bg-blue-500 flex items-center justify-center">
                <Download className="h-5 w-5 text-white" />
              </div>
            </CardHeader>
            <CardContent>
              <div className="text-3xl font-bold text-blue-600">
                {formatBytes(stats?.total_bytes_in || 0)}
              </div>
              <p className="text-xs text-muted-foreground mt-1">Inbound traffic</p>
            </CardContent>
          </Card>

          <Card className="border-0 shadow-lg hover:shadow-xl transition-shadow animate-slide-up" style={{animationDelay: '0.3s'}}>
            <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
              <CardTitle className="text-sm font-medium text-muted-foreground">Data Sent</CardTitle>
              <div className="w-10 h-10 rounded-lg bg-blue-500 flex items-center justify-center">
                <Upload className="h-5 w-5 text-white" />
              </div>
            </CardHeader>
            <CardContent>
              <div className="text-3xl font-bold text-blue-600">
                {formatBytes(stats?.total_bytes_out || 0)}
              </div>
              <p className="text-xs text-muted-foreground mt-1">Outbound traffic</p>
            </CardContent>
          </Card>
        </div>

        {/* Modern Tabs */}
        <div className="mb-6">
          <div className="bg-white rounded-lg shadow-sm p-1 inline-flex gap-1">
            <button
              onClick={() => setActiveTab('tunnels')}
              className={`px-6 py-2.5 rounded-md font-medium text-sm transition-all ${
                activeTab === 'tunnels'
                  ? 'bg-blue-600 text-white shadow-md'
                  : 'text-gray-600 hover:text-gray-900 hover:bg-gray-50'
              }`}
            >
              Tunnels
            </button>
            <button
              onClick={() => setActiveTab('tokens')}
              className={`px-6 py-2.5 rounded-md font-medium text-sm transition-all ${
                activeTab === 'tokens'
                  ? 'bg-blue-600 text-white shadow-md'
                  : 'text-gray-600 hover:text-gray-900 hover:bg-gray-50'
              }`}
            >
              Auth Tokens
            </button>
          </div>
        </div>

        {/* Content */}
        <div className="pb-8">
          {activeTab === 'tunnels' ? <TunnelList /> : <TokenManager />}
        </div>
      </div>
    </div>
  );
}

export default Dashboard;
