import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { useParams, useNavigate } from 'react-router-dom';
import {
  Box,
  Card,
  CardContent,
  Button,
  IconButton,
  Typography,
  Tabs,
  Tab,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Chip,
  CircularProgress,
  Paper,
  Tooltip,
  Alert,
} from '@mui/material';
import { ArrowLeft, Copy, CheckCircle2, XCircle } from 'lucide-react';
import { toast } from 'sonner';
import { api } from '@/lib/api';
import type { WebhookEventDetail } from '@/lib/api';
import { formatBytes } from '@/lib/utils';

interface TabPanelProps {
  children?: React.ReactNode;
  index: number;
  value: number;
}

function TabPanel({ children, value, index }: TabPanelProps) {
  return (
    <div role="tabpanel" hidden={value !== index}>
      {value === index && <Box sx={{ py: 2 }}>{children}</Box>}
    </div>
  );
}

export function WebhookEventDetail() {
  const { appId, eventId } = useParams<{ appId: string; eventId: string }>();
  const navigate = useNavigate();
  const [tabValue, setTabValue] = useState(0);

  const { data: event, isLoading, error } = useQuery({
    queryKey: ['webhook-event-detail', appId, eventId],
    queryFn: async () => {
      if (!appId || !eventId) throw new Error('Missing parameters');
      const response = await api.webhooks.getEventDetail(appId, eventId);
      return response.data;
    },
    enabled: !!appId && !!eventId,
  });

  const copyToClipboard = (text: string) => {
    navigator.clipboard.writeText(text);
    toast.success('Copied to clipboard');
  };

  const formatTimestamp = (timestamp: string) => {
    return new Date(timestamp).toLocaleString();
  };

  if (isLoading) {
    return (
      <Box sx={{ display: 'flex', justifyContent: 'center', py: 8 }}>
        <CircularProgress />
      </Box>
    );
  }

  if (error || !event) {
    return (
      <Box>
        <Alert severity="error">Failed to load event details</Alert>
        <Button onClick={() => navigate(`/webhooks/${appId}`)} sx={{ mt: 2 }}>
          Back to Webhook App
        </Button>
      </Box>
    );
  }

  const getStatusColor = (status: number) => {
    if (status >= 200 && status < 300) return 'success';
    if (status >= 400) return 'error';
    return 'warning';
  };

  return (
    <Box>
      {/* Header */}
      <Box sx={{ display: 'flex', alignItems: 'center', gap: 2, mb: 4 }}>
        <IconButton onClick={() => navigate(`/webhooks/${appId}`)}>
          <ArrowLeft size={20} />
        </IconButton>
        <Box>
          <Typography variant="h4" sx={{ fontWeight: 700 }}>
            Webhook Event Details
          </Typography>
          <Typography variant="body2" color="text.secondary">
            {formatTimestamp(event.created_at)}
          </Typography>
        </Box>
      </Box>

      {/* Summary Card */}
      <Card sx={{ mb: 3 }}>
        <CardContent>
          <Typography variant="h6" sx={{ mb: 2 }}>
            Request Summary
          </Typography>
          <Box sx={{ display: 'flex', gap: 1, flexWrap: 'wrap', mb: 2 }}>
            <Chip label={event.method} color="primary" size="small" />
            <Chip
              label={`Status: ${event.status_code || 'N/A'}`}
              color={event.status_code ? getStatusColor(event.status_code) : 'default'}
              size="small"
            />
            <Chip label={`Duration: ${event.duration_ms}ms`} size="small" />
            <Chip label={`In: ${formatBytes(event.bytes_in)}`} size="small" />
            <Chip label={`Out: ${formatBytes(event.bytes_out)}`} size="small" />
            <Chip
              label={`Tunnels: ${event.success_count}/${event.tunnel_count}`}
              color={event.success_count > 0 ? 'success' : 'error'}
              size="small"
            />
          </Box>
          <Typography variant="body2" sx={{ fontFamily: 'monospace', mb: 1 }}>
            {event.request_path}
          </Typography>
          {event.client_ip && (
            <Typography variant="caption" color="text.secondary">
              Client IP: {event.client_ip}
            </Typography>
          )}
          {event.body_truncated && (
            <Alert severity="warning" sx={{ mt: 2 }}>
              Request or response body was truncated due to size limits (max 100KB)
            </Alert>
          )}
        </CardContent>
      </Card>

      {/* Per-Tunnel Latency Breakdown */}
      {event.tunnel_responses && event.tunnel_responses.length > 0 && (() => {
        // Calculate Grok processing overhead
        const maxTunnelLatency = Math.max(...event.tunnel_responses.map(tr => tr.duration_ms));
        const grokOverhead = event.duration_ms - maxTunnelLatency;

        return (
          <Card sx={{ mb: 3 }}>
            <CardContent>
              <Typography variant="h6" sx={{ mb: 2 }}>
                Per-Tunnel Latency Breakdown
              </Typography>

              {/* Total Latency Visualization */}
              <Box sx={{ mb: 3, p: 2, bgcolor: 'background.default', borderRadius: 1 }}>
                <Typography variant="body2" color="text.secondary" sx={{ mb: 1 }}>
                  Total Event Duration: <strong>{event.duration_ms}ms</strong>
                </Typography>
                <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                  <Typography variant="caption" color="text.secondary">
                    Internet
                  </Typography>
                  <Box sx={{ flex: 1, height: 24, display: 'flex', border: '1px solid', borderColor: 'divider', borderRadius: 0.5, overflow: 'hidden' }}>
                    {grokOverhead > 0 && (
                      <Box
                        sx={{
                          width: `${(grokOverhead / event.duration_ms) * 100}%`,
                          bgcolor: '#94a3b8',
                          display: 'flex',
                          alignItems: 'center',
                          justifyContent: 'center',
                          minWidth: 40,
                        }}
                      >
                        <Typography variant="caption" sx={{ color: 'white', fontSize: '0.65rem' }}>
                          Grok {grokOverhead}ms
                        </Typography>
                      </Box>
                    )}
                    <Box
                      sx={{
                        width: `${(maxTunnelLatency / event.duration_ms) * 100}%`,
                        bgcolor: '#667eea',
                        display: 'flex',
                        alignItems: 'center',
                        justifyContent: 'center',
                      }}
                    >
                      <Typography variant="caption" sx={{ color: 'white', fontSize: '0.65rem' }}>
                        Tunnel {maxTunnelLatency}ms
                      </Typography>
                    </Box>
                  </Box>
                  <Typography variant="caption" color="text.secondary">
                    Response
                  </Typography>
                </Box>
              </Box>

              <TableContainer component={Paper} variant="outlined">
                <Table>
                  <TableHead>
                    <TableRow>
                      <TableCell>Tunnel</TableCell>
                      <TableCell>Status</TableCell>
                      <TableCell>Latency Breakdown</TableCell>
                      <TableCell>Total</TableCell>
                      <TableCell>Success</TableCell>
                      <TableCell>Error</TableCell>
                    </TableRow>
                  </TableHead>
                  <TableBody>
                    {event.tunnel_responses.map((tr) => {
                      const tunnelLatency = tr.duration_ms;
                      const totalWithOverhead = grokOverhead + tunnelLatency;

                      return (
                        <TableRow key={tr.id}>
                          <TableCell>
                            <Typography variant="body2" fontWeight={500}>
                              {tr.tunnel_subdomain}
                            </Typography>
                          </TableCell>
                          <TableCell>
                            <Chip
                              label={tr.status_code || 'Failed'}
                              color={
                                tr.status_code >= 200 && tr.status_code < 300
                                  ? 'success'
                                  : tr.status_code >= 400
                                  ? 'error'
                                  : 'default'
                              }
                              size="small"
                            />
                          </TableCell>
                          <TableCell>
                            <Box sx={{ minWidth: 200 }}>
                              <Box sx={{ display: 'flex', height: 20, border: '1px solid', borderColor: 'divider', borderRadius: 0.5, overflow: 'hidden' }}>
                                {grokOverhead > 0 && (
                                  <Tooltip title={`Grok overhead: ${grokOverhead}ms`}>
                                    <Box
                                      sx={{
                                        width: `${(grokOverhead / totalWithOverhead) * 100}%`,
                                        bgcolor: '#94a3b8',
                                      }}
                                    />
                                  </Tooltip>
                                )}
                                <Tooltip title={`Tunnel processing: ${tunnelLatency}ms`}>
                                  <Box
                                    sx={{
                                      width: `${(tunnelLatency / totalWithOverhead) * 100}%`,
                                      bgcolor: '#667eea',
                                    }}
                                  />
                                </Tooltip>
                              </Box>
                              <Typography variant="caption" color="text.secondary" sx={{ mt: 0.5, display: 'block' }}>
                                Grok: {grokOverhead}ms → Tunnel: {tunnelLatency}ms
                              </Typography>
                            </Box>
                          </TableCell>
                          <TableCell>
                            <Typography variant="body2" fontWeight={500}>
                              {totalWithOverhead}ms
                            </Typography>
                          </TableCell>
                          <TableCell>
                            {tr.success ? (
                              <CheckCircle2 size={16} color="#10b981" />
                            ) : (
                              <XCircle size={16} color="#ef4444" />
                            )}
                          </TableCell>
                          <TableCell>
                            <Typography variant="caption" color="error">
                              {tr.error_message || '-'}
                            </Typography>
                          </TableCell>
                        </TableRow>
                      );
                    })}
                  </TableBody>
                </Table>
              </TableContainer>
            </CardContent>
          </Card>
        );
      })()}

      {/* Tabs for Headers/Body */}
      <Card>
        <CardContent>
          <Box sx={{ borderBottom: 1, borderColor: 'divider' }}>
            <Tabs value={tabValue} onChange={(_, v) => setTabValue(v)}>
              <Tab label="Request Headers" />
              <Tab label="Request Body" />
              <Tab label="Response Headers" />
              <Tab label="Response Body" />
            </Tabs>
          </Box>

          <TabPanel value={tabValue} index={0}>
            <HeadersTable
              headers={event.request_headers_parsed}
              onCopy={copyToClipboard}
            />
          </TabPanel>

          <TabPanel value={tabValue} index={1}>
            <BodyViewer body={event.request_body} onCopy={copyToClipboard} />
          </TabPanel>

          <TabPanel value={tabValue} index={2}>
            <HeadersTable
              headers={event.response_headers_parsed}
              onCopy={copyToClipboard}
            />
          </TabPanel>

          <TabPanel value={tabValue} index={3}>
            <BodyViewer body={event.response_body} onCopy={copyToClipboard} />
          </TabPanel>
        </CardContent>
      </Card>
    </Box>
  );
}

// Helper components
function HeadersTable({
  headers,
  onCopy,
}: {
  headers?: Record<string, string[]>;
  onCopy: (text: string) => void;
}) {
  if (!headers || Object.keys(headers).length === 0) {
    return (
      <Typography variant="body2" color="text.secondary">
        No headers
      </Typography>
    );
  }

  return (
    <Table size="small">
      <TableBody>
        {Object.entries(headers).map(([key, values]) => (
          <TableRow key={key}>
            <TableCell sx={{ fontWeight: 'bold', width: '30%' }}>{key}</TableCell>
            <TableCell>
              <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
                <Typography variant="body2">{values.join(', ')}</Typography>
                <Tooltip title="Copy">
                  <IconButton size="small" onClick={() => onCopy(values.join(', '))}>
                    <Copy size={16} />
                  </IconButton>
                </Tooltip>
              </Box>
            </TableCell>
          </TableRow>
        ))}
      </TableBody>
    </Table>
  );
}

function BodyViewer({ body, onCopy }: { body?: string; onCopy: (text: string) => void }) {
  if (!body) {
    return (
      <Typography variant="body2" color="text.secondary">
        No body content
      </Typography>
    );
  }

  // Try to format JSON
  let formattedBody = body;
  try {
    const parsed = JSON.parse(body);
    formattedBody = JSON.stringify(parsed, null, 2);
  } catch {
    // Not JSON, use as-is
  }

  return (
    <Paper variant="outlined" sx={{ p: 2, position: 'relative' }}>
      <Box sx={{ position: 'absolute', top: 8, right: 8 }}>
        <Tooltip title="Copy">
          <IconButton size="small" onClick={() => onCopy(body)}>
            <Copy size={16} />
          </IconButton>
        </Tooltip>
      </Box>
      <Typography
        component="pre"
        sx={{
          fontFamily: 'monospace',
          fontSize: '0.75rem',
          whiteSpace: 'pre-wrap',
          wordBreak: 'break-word',
          maxHeight: 400,
          overflow: 'auto',
          mt: 2,
        }}
      >
        {formattedBody}
      </Typography>
    </Paper>
  );
}
