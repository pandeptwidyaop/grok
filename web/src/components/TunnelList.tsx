import { useQuery } from '@tanstack/react-query';
import { api, type Tunnel } from '@/lib/api';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import { Badge } from '@/components/ui/badge';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Globe, Activity, ArrowUpDown } from 'lucide-react';

function TunnelList() {
  const { data: tunnels, isLoading } = useQuery({
    queryKey: ['tunnels'],
    queryFn: async () => {
      const response = await api.tunnels.list();
      return response.data;
    },
    refetchInterval: 3000, // Refresh every 3 seconds
  });

  const formatBytes = (bytes: number) => {
    if (bytes === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
  };

  const formatDate = (date: string) => {
    if (!date) return 'N/A';
    const d = new Date(date);
    return isNaN(d.getTime()) ? 'N/A' : d.toLocaleString();
  };

  const getStatusBadge = (status: string) => {
    if (!status) return <Badge variant="outline">Unknown</Badge>;

    switch (status.toLowerCase()) {
      case 'active':
        return <Badge variant="default">Active</Badge>;
      case 'inactive':
        return <Badge variant="secondary">Inactive</Badge>;
      default:
        return <Badge variant="outline">{status}</Badge>;
    }
  };

  const getTypeBadge = (type: string) => {
    if (!type) return <Badge>Unknown</Badge>;

    const colors: Record<string, string> = {
      http: 'bg-blue-500',
      https: 'bg-green-500',
      tcp: 'bg-purple-500',
    };
    return (
      <Badge className={colors[type.toLowerCase()] || ''}>
        {type.toUpperCase()}
      </Badge>
    );
  };

  if (isLoading) {
    return (
      <Card>
        <CardHeader>
          <CardTitle>Active Tunnels</CardTitle>
          <CardDescription>Loading tunnels...</CardDescription>
        </CardHeader>
      </Card>
    );
  }

  if (!tunnels || tunnels.length === 0) {
    return (
      <Card>
        <CardHeader>
          <CardTitle>Active Tunnels</CardTitle>
          <CardDescription>
            No active tunnels. Start a tunnel with the Grok client.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <div className="text-center py-8 text-muted-foreground">
            <Globe className="h-16 w-16 mx-auto mb-4 opacity-50" />
            <p className="text-lg mb-2">No tunnels running</p>
            <p className="text-sm">
              Run <code className="bg-muted px-2 py-1 rounded">grok http 3000</code>{' '}
              to create your first tunnel
            </p>
          </div>
        </CardContent>
      </Card>
    );
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>Active Tunnels</CardTitle>
        <CardDescription>
          {tunnels.length} tunnel{tunnels.length !== 1 ? 's' : ''} currently active
        </CardDescription>
      </CardHeader>
      <CardContent>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Type</TableHead>
              <TableHead>Public URL</TableHead>
              <TableHead>Local Address</TableHead>
              <TableHead>Status</TableHead>
              <TableHead className="text-right">Requests</TableHead>
              <TableHead className="text-right">Data In/Out</TableHead>
              <TableHead>Connected</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {tunnels.map((tunnel: Tunnel) => (
              <TableRow key={tunnel.id}>
                <TableCell>{getTypeBadge(tunnel.tunnel_type)}</TableCell>
                <TableCell>
                  <a
                    href={tunnel.public_url}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="text-primary hover:underline flex items-center gap-2"
                  >
                    {tunnel.public_url}
                    <Globe className="h-4 w-4" />
                  </a>
                </TableCell>
                <TableCell>
                  <code className="text-sm">{tunnel.local_addr}</code>
                </TableCell>
                <TableCell>{getStatusBadge(tunnel.status)}</TableCell>
                <TableCell className="text-right">
                  <div className="flex items-center justify-end gap-2">
                    <Activity className="h-4 w-4 text-muted-foreground" />
                    {(tunnel.requests_count ?? 0).toLocaleString()}
                  </div>
                </TableCell>
                <TableCell className="text-right">
                  <div className="flex flex-col items-end text-sm">
                    <div className="flex items-center gap-1">
                      <ArrowUpDown className="h-3 w-3 text-muted-foreground" />
                      {formatBytes(tunnel.bytes_in ?? 0)} / {formatBytes(tunnel.bytes_out ?? 0)}
                    </div>
                  </div>
                </TableCell>
                <TableCell className="text-sm text-muted-foreground">
                  {formatDate(tunnel.connected_at)}
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </CardContent>
    </Card>
  );
}

export default TunnelList;
