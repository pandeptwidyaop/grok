import { Paper, Typography, Grid, Box } from '@mui/material';
import {
  LineChart,
  Line,
  AreaChart,
  Area,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  Legend,
  ResponsiveContainer,
} from 'recharts';
import { useMetrics } from '@/hooks/useMetrics';
import { formatBytes, formatDuration } from '@/lib/utils';

function PerformanceCharts() {
  const { metrics, history } = useMetrics();

  if (!metrics) {
    return (
      <Paper sx={{ p: 3 }}>
        <Typography variant="h6" gutterBottom>
          Performance Metrics
        </Typography>
        <Typography variant="body2" color="text.secondary">
          Loading metrics...
        </Typography>
      </Paper>
    );
  }

  // Prepare data for charts (reverse history to show oldest first)
  const chartData = [...history].reverse().map((snapshot, index) => ({
    index,
    time: new Date(snapshot.timestamp).toLocaleTimeString(),
    requestRate: snapshot.request_rate,
    avgLatency: snapshot.avg_latency_ms,
    p50Latency: snapshot.p50_latency_ms,
    p95Latency: snapshot.p95_latency_ms,
    p99Latency: snapshot.p99_latency_ms,
    bytesIn: snapshot.bytes_in,
    bytesOut: snapshot.bytes_out,
  }));

  // Take last 30 data points for better visibility
  const recentData = chartData.slice(-30);

  return (
    <Paper sx={{ p: 3 }}>
      <Typography variant="h6" gutterBottom>
        Performance Metrics
      </Typography>

      {/* Current Stats */}
      <Grid container spacing={2} sx={{ mb: 4 }}>
        <Grid item xs={6} md={3}>
          <Box>
            <Typography variant="body2" color="text.secondary">
              Total Requests
            </Typography>
            <Typography variant="h5">
              {metrics.total_requests.toLocaleString()}
            </Typography>
          </Box>
        </Grid>

        <Grid item xs={6} md={3}>
          <Box>
            <Typography variant="body2" color="text.secondary">
              Request Rate
            </Typography>
            <Typography variant="h5">
              {metrics.request_rate.toFixed(2)}/s
            </Typography>
          </Box>
        </Grid>

        <Grid item xs={6} md={3}>
          <Box>
            <Typography variant="body2" color="text.secondary">
              Avg Latency
            </Typography>
            <Typography variant="h5">
              {formatDuration(metrics.avg_latency_ms)}
            </Typography>
          </Box>
        </Grid>

        <Grid item xs={6} md={3}>
          <Box>
            <Typography variant="body2" color="text.secondary">
              Errors
            </Typography>
            <Typography variant="h5" color={metrics.error_count > 0 ? 'error' : 'text.primary'}>
              {metrics.error_count}
            </Typography>
          </Box>
        </Grid>
      </Grid>

      {/* Charts */}
      <Grid container spacing={3}>
        {/* Request Rate Chart */}
        <Grid item xs={12} md={4}>
          <Typography variant="subtitle2" gutterBottom>
            Request Rate (req/s)
          </Typography>
          <ResponsiveContainer width="100%" height={200}>
            <LineChart data={recentData}>
              <CartesianGrid strokeDasharray="3 3" />
              <XAxis dataKey="time" hide />
              <YAxis />
              <Tooltip />
              <Line
                type="monotone"
                dataKey="requestRate"
                stroke="#2196f3"
                strokeWidth={2}
                dot={false}
              />
            </LineChart>
          </ResponsiveContainer>
        </Grid>

        {/* Latency Chart */}
        <Grid item xs={12} md={4}>
          <Typography variant="subtitle2" gutterBottom>
            Latency (ms)
          </Typography>
          <ResponsiveContainer width="100%" height={200}>
            <AreaChart data={recentData}>
              <CartesianGrid strokeDasharray="3 3" />
              <XAxis dataKey="time" hide />
              <YAxis />
              <Tooltip formatter={(value) => formatDuration(value as number)} />
              <Legend />
              <Area
                type="monotone"
                dataKey="p99Latency"
                stackId="1"
                stroke="#f44336"
                fill="#f44336"
                fillOpacity={0.3}
                name="P99"
              />
              <Area
                type="monotone"
                dataKey="p95Latency"
                stackId="2"
                stroke="#ff9800"
                fill="#ff9800"
                fillOpacity={0.3}
                name="P95"
              />
              <Area
                type="monotone"
                dataKey="p50Latency"
                stackId="3"
                stroke="#4caf50"
                fill="#4caf50"
                fillOpacity={0.3}
                name="P50"
              />
            </AreaChart>
          </ResponsiveContainer>
        </Grid>

        {/* Throughput Chart */}
        <Grid item xs={12} md={4}>
          <Typography variant="subtitle2" gutterBottom>
            Throughput (bytes)
          </Typography>
          <ResponsiveContainer width="100%" height={200}>
            <AreaChart data={recentData}>
              <CartesianGrid strokeDasharray="3 3" />
              <XAxis dataKey="time" hide />
              <YAxis tickFormatter={(value) => formatBytes(value)} />
              <Tooltip formatter={(value) => formatBytes(value as number)} />
              <Legend />
              <Area
                type="monotone"
                dataKey="bytesIn"
                stackId="1"
                stroke="#2196f3"
                fill="#2196f3"
                fillOpacity={0.6}
                name="Bytes In"
              />
              <Area
                type="monotone"
                dataKey="bytesOut"
                stackId="1"
                stroke="#4caf50"
                fill="#4caf50"
                fillOpacity={0.6}
                name="Bytes Out"
              />
            </AreaChart>
          </ResponsiveContainer>
        </Grid>
      </Grid>
    </Paper>
  );
}

export default PerformanceCharts;
