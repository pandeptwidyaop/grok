import { useState, useEffect } from 'react';
import {
  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
  Button,
  Tabs,
  Tab,
  Box,
  Typography,
  Paper,
  Table,
  TableBody,
  TableCell,
  TableRow,
  IconButton,
  Tooltip,
  Chip,
} from '@mui/material';
import { ContentCopy, Close } from '@mui/icons-material';
import { toast } from 'sonner';
import api from '@/services/api';
import type { RequestLog } from '@/types';
import { formatBytes, formatDuration, formatFullTimestamp, getStatusColor } from '@/lib/utils';

interface RequestDetailProps {
  requestId: string;
  open: boolean;
  onClose: () => void;
}

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

function RequestDetail({ requestId, open, onClose }: RequestDetailProps) {
  const [tabValue, setTabValue] = useState(0);
  const [request, setRequest] = useState<RequestLog | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    if (open && requestId) {
      loadRequestDetail();
    }
  }, [open, requestId]);

  const loadRequestDetail = async () => {
    try {
      setLoading(true);
      const data = await api.requests.get(requestId);
      setRequest(data);
    } catch (error) {
      console.error('Failed to load request detail:', error);
      toast.error('Failed to load request details');
    } finally {
      setLoading(false);
    }
  };

  const copyToClipboard = (text: string) => {
    navigator.clipboard.writeText(text);
    toast.success('Copied to clipboard');
  };

  if (!request && !loading) {
    return null;
  }

  return (
    <Dialog open={open} onClose={onClose} maxWidth="md" fullWidth>
      <DialogTitle>
        <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
          <Typography variant="h6">Request Details</Typography>
          <IconButton onClick={onClose} size="small">
            <Close />
          </IconButton>
        </Box>
      </DialogTitle>

      <DialogContent dividers>
        {loading ? (
          <Typography>Loading...</Typography>
        ) : request ? (
          <>
            {/* Request Summary */}
            <Box sx={{ mb: 3 }}>
              <Typography variant="subtitle2" color="text.secondary" gutterBottom>
                Request Summary
              </Typography>
              <Box sx={{ display: 'flex', gap: 1, flexWrap: 'wrap', mb: 2 }}>
                <Chip label={request.method} color="primary" size="small" />
                <Chip
                  label={`Status: ${request.status_code || 'N/A'}`}
                  color={getStatusColor(request.status_code) as any}
                  size="small"
                />
                <Chip label={`Duration: ${formatDuration(request.duration_ms)}`} size="small" />
                <Chip label={`In: ${formatBytes(request.bytes_in)}`} size="small" />
                <Chip label={`Out: ${formatBytes(request.bytes_out)}`} size="small" />
              </Box>
              <Typography variant="body2" sx={{ fontFamily: 'monospace', mb: 1 }}>
                {request.path}
              </Typography>
              <Typography variant="caption" color="text.secondary">
                {formatFullTimestamp(request.timestamp)} • {request.remote_addr}
              </Typography>
            </Box>

            {/* Tabs */}
            <Box sx={{ borderBottom: 1, borderColor: 'divider' }}>
              <Tabs value={tabValue} onChange={(_, v) => setTabValue(v)}>
                <Tab label="Request Headers" />
                <Tab label="Request Body" />
                <Tab label="Response Headers" />
                <Tab label="Response Body" />
                <Tab label="Timing" />
              </Tabs>
            </Box>

            {/* Tab Panels */}
            <TabPanel value={tabValue} index={0}>
              <HeadersTable headers={request.request_headers} onCopy={copyToClipboard} />
            </TabPanel>

            <TabPanel value={tabValue} index={1}>
              <BodyViewer body={request.request_body} onCopy={copyToClipboard} />
            </TabPanel>

            <TabPanel value={tabValue} index={2}>
              <HeadersTable headers={request.response_headers} onCopy={copyToClipboard} />
            </TabPanel>

            <TabPanel value={tabValue} index={3}>
              <BodyViewer body={request.response_body} onCopy={copyToClipboard} />
            </TabPanel>

            <TabPanel value={tabValue} index={4}>
              <TimingInfo request={request} />
            </TabPanel>
          </>
        ) : (
          <Typography>Request not found</Typography>
        )}
      </DialogContent>

      <DialogActions>
        <Button onClick={onClose}>Close</Button>
      </DialogActions>
    </Dialog>
  );
}

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
    <TableBody>
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
                      <ContentCopy fontSize="small" />
                    </IconButton>
                  </Tooltip>
                </Box>
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </TableBody>
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
            <ContentCopy fontSize="small" />
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

function TimingInfo({ request }: { request: RequestLog }) {
  return (
    <Table size="small">
      <TableBody>
        <TableRow>
          <TableCell sx={{ fontWeight: 'bold' }}>Timestamp</TableCell>
          <TableCell>{formatFullTimestamp(request.timestamp)}</TableCell>
        </TableRow>
        <TableRow>
          <TableCell sx={{ fontWeight: 'bold' }}>Duration</TableCell>
          <TableCell>{formatDuration(request.duration_ms)}</TableCell>
        </TableRow>
        <TableRow>
          <TableCell sx={{ fontWeight: 'bold' }}>Protocol</TableCell>
          <TableCell>{request.protocol.toUpperCase()}</TableCell>
        </TableRow>
        <TableRow>
          <TableCell sx={{ fontWeight: 'bold' }}>Remote Address</TableCell>
          <TableCell>{request.remote_addr}</TableCell>
        </TableRow>
        <TableRow>
          <TableCell sx={{ fontWeight: 'bold' }}>Request ID</TableCell>
          <TableCell sx={{ fontFamily: 'monospace', fontSize: '0.75rem' }}>{request.id}</TableCell>
        </TableRow>
        {request.error && (
          <TableRow>
            <TableCell sx={{ fontWeight: 'bold' }}>Error</TableCell>
            <TableCell>
              <Typography color="error" variant="body2">
                {request.error}
              </Typography>
            </TableCell>
          </TableRow>
        )}
      </TableBody>
    </Table>
  );
}

export default RequestDetail;
