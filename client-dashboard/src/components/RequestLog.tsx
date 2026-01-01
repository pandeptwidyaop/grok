import { useState, useMemo } from 'react';
import {
  Paper,
  Typography,
  Box,
  TextField,
  Select,
  MenuItem,
  FormControl,
  InputLabel,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Chip,
  IconButton,
  Tooltip,
  Switch,
  FormControlLabel,
} from '@mui/material';
import {
  Refresh,
  Clear,
  Search,
  Info,
} from '@mui/icons-material';
import { useRequestLog } from '@/hooks/useRequestLog';
import { formatBytes, formatDuration, formatTimestamp, getStatusColor } from '@/lib/utils';
import RequestDetail from './RequestDetail';

function RequestLog() {
  const {
    requests,
    loading,
    autoScroll,
    setAutoScroll,
    filterMethod,
    setFilterMethod,
    filterStatus,
    setFilterStatus,
    searchQuery,
    setSearchQuery,
    clear,
    refresh,
  } = useRequestLog();

  const [selectedRequest, setSelectedRequest] = useState<string | null>(null);

  // Filter requests based on criteria
  const filteredRequests = useMemo(() => {
    return requests.filter((req) => {
      // Method filter
      if (filterMethod !== 'ALL' && req.method !== filterMethod) {
        return false;
      }

      // Status filter
      if (filterStatus !== 'ALL') {
        const statusCode = req.status_code;
        if (filterStatus === '2xx' && (statusCode < 200 || statusCode >= 300)) return false;
        if (filterStatus === '3xx' && (statusCode < 300 || statusCode >= 400)) return false;
        if (filterStatus === '4xx' && (statusCode < 400 || statusCode >= 500)) return false;
        if (filterStatus === '5xx' && (statusCode < 500 || statusCode >= 600)) return false;
      }

      // Search query
      if (searchQuery) {
        const query = searchQuery.toLowerCase();
        return (
          req.path.toLowerCase().includes(query) ||
          req.method.toLowerCase().includes(query) ||
          req.remote_addr.toLowerCase().includes(query)
        );
      }

      return true;
    });
  }, [requests, filterMethod, filterStatus, searchQuery]);

  // Get unique methods for filter
  const uniqueMethods = useMemo(() => {
    const methods = new Set(requests.map((r) => r.method));
    return Array.from(methods).sort();
  }, [requests]);

  return (
    <Paper sx={{ p: 3 }}>
      {/* Header */}
      <Box sx={{ display: 'flex', alignItems: 'center', mb: 3 }}>
        <Typography variant="h6" sx={{ flexGrow: 1 }}>
          Request Log
        </Typography>
        <FormControlLabel
          control={<Switch checked={autoScroll} onChange={(e) => setAutoScroll(e.target.checked)} />}
          label="Auto-scroll"
        />
        <Tooltip title="Refresh">
          <IconButton onClick={refresh} disabled={loading}>
            <Refresh />
          </IconButton>
        </Tooltip>
        <Tooltip title="Clear all">
          <IconButton onClick={clear}>
            <Clear />
          </IconButton>
        </Tooltip>
      </Box>

      {/* Filters */}
      <Box sx={{ display: 'flex', gap: 2, mb: 3, flexWrap: 'wrap' }}>
        <TextField
          size="small"
          placeholder="Search path, method, IP..."
          value={searchQuery}
          onChange={(e) => setSearchQuery(e.target.value)}
          InputProps={{
            startAdornment: <Search sx={{ mr: 1, color: 'text.secondary' }} />,
          }}
          sx={{ minWidth: 250 }}
        />

        <FormControl size="small" sx={{ minWidth: 120 }}>
          <InputLabel>Method</InputLabel>
          <Select value={filterMethod} onChange={(e) => setFilterMethod(e.target.value)} label="Method">
            <MenuItem value="ALL">All Methods</MenuItem>
            {uniqueMethods.map((method) => (
              <MenuItem key={method} value={method}>
                {method}
              </MenuItem>
            ))}
          </Select>
        </FormControl>

        <FormControl size="small" sx={{ minWidth: 120 }}>
          <InputLabel>Status</InputLabel>
          <Select value={filterStatus} onChange={(e) => setFilterStatus(e.target.value)} label="Status">
            <MenuItem value="ALL">All Status</MenuItem>
            <MenuItem value="2xx">2xx Success</MenuItem>
            <MenuItem value="3xx">3xx Redirect</MenuItem>
            <MenuItem value="4xx">4xx Client Error</MenuItem>
            <MenuItem value="5xx">5xx Server Error</MenuItem>
          </Select>
        </FormControl>

        <Box sx={{ flexGrow: 1 }} />

        <Typography variant="body2" color="text.secondary" sx={{ display: 'flex', alignItems: 'center' }}>
          {filteredRequests.length} of {requests.length} requests
        </Typography>
      </Box>

      {/* Table */}
      <TableContainer sx={{ maxHeight: 600 }}>
        <Table stickyHeader size="small">
          <TableHead>
            <TableRow>
              <TableCell>Time</TableCell>
              <TableCell>Method</TableCell>
              <TableCell>Path</TableCell>
              <TableCell>Status</TableCell>
              <TableCell align="right">Duration</TableCell>
              <TableCell align="right">Bytes In</TableCell>
              <TableCell align="right">Bytes Out</TableCell>
              <TableCell align="center">Actions</TableCell>
            </TableRow>
          </TableHead>
          <TableBody>
            {filteredRequests.length === 0 ? (
              <TableRow>
                <TableCell colSpan={8} align="center">
                  <Typography variant="body2" color="text.secondary" sx={{ py: 4 }}>
                    {loading ? 'Loading requests...' : 'No requests yet'}
                  </Typography>
                </TableCell>
              </TableRow>
            ) : (
              filteredRequests.map((req) => (
                <TableRow
                  key={req.id}
                  hover
                  onClick={() => setSelectedRequest(req.id)}
                  sx={{ cursor: 'pointer' }}
                >
                  <TableCell>
                    <Typography variant="body2" sx={{ fontFamily: 'monospace', fontSize: '0.75rem' }}>
                      {formatTimestamp(req.timestamp)}
                    </Typography>
                  </TableCell>
                  <TableCell>
                    <Chip label={req.method} size="small" color="primary" variant="outlined" />
                  </TableCell>
                  <TableCell>
                    <Typography variant="body2" sx={{ fontFamily: 'monospace', fontSize: '0.875rem' }}>
                      {req.path.length > 50 ? req.path.substring(0, 50) + '...' : req.path}
                    </Typography>
                  </TableCell>
                  <TableCell>
                    <Chip
                      label={req.status_code || 'N/A'}
                      size="small"
                      color={getStatusColor(req.status_code) as any}
                    />
                  </TableCell>
                  <TableCell align="right">
                    <Typography variant="body2" sx={{ fontFamily: 'monospace' }}>
                      {formatDuration(req.duration_ms)}
                    </Typography>
                  </TableCell>
                  <TableCell align="right">
                    <Typography variant="body2">{formatBytes(req.bytes_in)}</Typography>
                  </TableCell>
                  <TableCell align="right">
                    <Typography variant="body2">{formatBytes(req.bytes_out)}</Typography>
                  </TableCell>
                  <TableCell align="center">
                    <Tooltip title="View details">
                      <IconButton
                        size="small"
                        onClick={(e) => {
                          e.stopPropagation(); // Prevent double trigger
                          setSelectedRequest(req.id);
                        }}
                      >
                        <Info fontSize="small" />
                      </IconButton>
                    </Tooltip>
                  </TableCell>
                </TableRow>
              ))
            )}
          </TableBody>
        </Table>
      </TableContainer>

      {/* Request Detail Dialog */}
      {selectedRequest && (
        <RequestDetail
          requestId={selectedRequest}
          open={Boolean(selectedRequest)}
          onClose={() => setSelectedRequest(null)}
        />
      )}
    </Paper>
  );
}

export default RequestLog;
