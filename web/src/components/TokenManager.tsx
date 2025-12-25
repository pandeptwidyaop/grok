import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { api, type AuthToken } from '@/lib/api';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Key, Trash2, Copy, Check } from 'lucide-react';

function TokenManager() {
  const [newTokenName, setNewTokenName] = useState('');
  const [createdToken, setCreatedToken] = useState<AuthToken | null>(null);
  const [copiedId, setCopiedId] = useState<string | null>(null);
  const queryClient = useQueryClient();

  const { data: tokens, isLoading } = useQuery({
    queryKey: ['tokens'],
    queryFn: async () => {
      const response = await api.tokens.list();
      return response.data;
    },
  });

  const createMutation = useMutation({
    mutationFn: (name: string) => api.tokens.create(name, ['tunnel:create', 'tunnel:list']),
    onSuccess: (response) => {
      setCreatedToken(response.data);
      setNewTokenName('');
      queryClient.invalidateQueries({ queryKey: ['tokens'] });
    },
  });

  const deleteMutation = useMutation({
    mutationFn: (id: string) => api.tokens.delete(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['tokens'] });
    },
  });

  const handleCreate = () => {
    if (newTokenName.trim()) {
      createMutation.mutate(newTokenName);
    }
  };

  const handleCopy = async (token: string, id: string) => {
    await navigator.clipboard.writeText(token);
    setCopiedId(id);
    setTimeout(() => setCopiedId(null), 2000);
  };

  const formatDate = (date: string) => {
    return new Date(date).toLocaleString();
  };

  if (isLoading) {
    return (
      <Card>
        <CardHeader>
          <CardTitle>Authentication Tokens</CardTitle>
          <CardDescription>Loading tokens...</CardDescription>
        </CardHeader>
      </Card>
    );
  }

  return (
    <div className="space-y-6">
      <Card>
        <CardHeader>
          <CardTitle>Create New Token</CardTitle>
          <CardDescription>
            Generate a new authentication token for the Grok client
          </CardDescription>
        </CardHeader>
        <CardContent>
          <div className="flex gap-4">
            <input
              type="text"
              placeholder="Token name (e.g., laptop, server)"
              value={newTokenName}
              onChange={(e) => setNewTokenName(e.target.value)}
              onKeyPress={(e) => e.key === 'Enter' && handleCreate()}
              className="flex-1 px-3 py-2 border border-input rounded-md bg-background"
            />
            <Button onClick={handleCreate} disabled={!newTokenName.trim()}>
              Create Token
            </Button>
          </div>
        </CardContent>
      </Card>

      {createdToken && createdToken.token && (
        <Card className="border-green-500 bg-green-50 dark:bg-green-950">
          <CardHeader>
            <CardTitle className="text-green-700 dark:text-green-300">
              Token Created Successfully!
            </CardTitle>
            <CardDescription className="text-green-600 dark:text-green-400">
              Make sure to copy your token now. You won't be able to see it again!
            </CardDescription>
          </CardHeader>
          <CardContent>
            <div className="flex items-center gap-4 p-4 bg-white dark:bg-gray-900 rounded-lg border">
              <code className="flex-1 font-mono text-sm break-all">
                {createdToken.token}
              </code>
              <Button
                size="sm"
                variant="outline"
                onClick={() => handleCopy(createdToken.token!, createdToken.id)}
              >
                {copiedId === createdToken.id ? (
                  <>
                    <Check className="h-4 w-4 mr-2" />
                    Copied
                  </>
                ) : (
                  <>
                    <Copy className="h-4 w-4 mr-2" />
                    Copy
                  </>
                )}
              </Button>
            </div>
            <p className="text-sm text-muted-foreground mt-4">
              Use this token with: <code className="bg-muted px-2 py-1 rounded">grok config set-token {createdToken.token}</code>
            </p>
          </CardContent>
        </Card>
      )}

      <Card>
        <CardHeader>
          <CardTitle>Authentication Tokens</CardTitle>
          <CardDescription>
            {tokens?.length || 0} token{tokens?.length !== 1 ? 's' : ''} configured
          </CardDescription>
        </CardHeader>
        <CardContent>
          {!tokens || tokens.length === 0 ? (
            <div className="text-center py-8 text-muted-foreground">
              <Key className="h-16 w-16 mx-auto mb-4 opacity-50" />
              <p className="text-lg mb-2">No tokens created</p>
              <p className="text-sm">
                Create a token above to start using the Grok client
              </p>
            </div>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Name</TableHead>
                  <TableHead>Scopes</TableHead>
                  <TableHead>Last Used</TableHead>
                  <TableHead>Created</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead className="text-right">Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {tokens.map((token: AuthToken) => (
                  <TableRow key={token.id}>
                    <TableCell className="font-medium">{token.name}</TableCell>
                    <TableCell>
                      <div className="flex gap-1">
                        {token.scopes && token.scopes.length > 0 ? (
                          token.scopes.map((scope) => (
                            <Badge key={scope} variant="secondary" className="text-xs">
                              {scope}
                            </Badge>
                          ))
                        ) : (
                          <span className="text-sm text-muted-foreground">No scopes</span>
                        )}
                      </div>
                    </TableCell>
                    <TableCell className="text-sm text-muted-foreground">
                      {token.last_used_at ? formatDate(token.last_used_at) : 'Never'}
                    </TableCell>
                    <TableCell className="text-sm text-muted-foreground">
                      {formatDate(token.created_at)}
                    </TableCell>
                    <TableCell>
                      {token.is_active ? (
                        <Badge variant="default">Active</Badge>
                      ) : (
                        <Badge variant="secondary">Inactive</Badge>
                      )}
                    </TableCell>
                    <TableCell className="text-right">
                      <Button
                        size="sm"
                        variant="destructive"
                        onClick={() => {
                          if (confirm('Are you sure you want to delete this token?')) {
                            deleteMutation.mutate(token.id);
                          }
                        }}
                      >
                        <Trash2 className="h-4 w-4" />
                      </Button>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>
    </div>
  );
}

export default TokenManager;
