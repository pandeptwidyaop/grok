import { useState, useEffect } from 'react';
import {
  Box,
  Card,
  CardContent,
  Typography,
  Accordion,
  AccordionSummary,
  AccordionDetails,
  Chip,
  Button,
  Alert,
  Stack,
  ToggleButtonGroup,
  ToggleButton,
  Tooltip,
  useMediaQuery,
  useTheme,
} from '@mui/material';
import {
  ChevronDown,
  Download,
  Key,
  Server,
  Globe,
  Network,
  Webhook,
  Copy,
  CheckCircle2,
  Terminal,
} from 'lucide-react';

interface OSInfo {
  os: 'linux' | 'darwin' | 'windows' | 'unknown';
  arch: 'amd64' | 'arm64' | '386' | 'arm' | 'unknown';
  downloadUrl: string;
  filename: string;
}

type ReleaseChannel = 'stable' | 'beta' | 'alpha';

function GettingStarted() {
  const [osInfo, setOsInfo] = useState<OSInfo>({
    os: 'unknown',
    arch: 'unknown',
    downloadUrl: '',
    filename: '',
  });
  const [copiedSteps, setCopiedSteps] = useState<{ [key: string]: boolean }>({});
  const [releaseChannel, setReleaseChannel] = useState<ReleaseChannel>('stable');
  const theme = useTheme();
  const isMobile = useMediaQuery(theme.breakpoints.down('sm'));

  // Server configuration
  const serverDomain = window.location.hostname;
  const grpcPort = '4443'; // gRPC port for client connections
  const apiPort = window.location.port || '4040'; // API port for dashboard
  const serverAddress = `${serverDomain}:${grpcPort}`;
  const isUsingTLS = window.location.protocol === 'https:';

  useEffect(() => {
    detectOS();
  }, []);

  useEffect(() => {
    updateDownloadURL();
  }, [releaseChannel, osInfo.os, osInfo.arch]);

  const detectOS = () => {
    const userAgent = window.navigator.userAgent.toLowerCase();
    const platform = window.navigator.platform.toLowerCase();

    let os: OSInfo['os'] = 'unknown';
    let arch: OSInfo['arch'] = 'amd64'; // default

    // Detect OS
    if (platform.includes('mac') || userAgent.includes('mac')) {
      os = 'darwin';
    } else if (platform.includes('win') || userAgent.includes('win')) {
      os = 'windows';
    } else if (platform.includes('linux') || userAgent.includes('linux')) {
      os = 'linux';
    }

    // Detect Architecture (basic detection)
    if (userAgent.includes('arm64') || userAgent.includes('aarch64')) {
      arch = 'arm64';
    } else if (userAgent.includes('x86_64') || userAgent.includes('x64')) {
      arch = 'amd64';
    } else if (userAgent.includes('i686') || userAgent.includes('i386')) {
      arch = '386';
    }

    const filename = os === 'windows'
      ? `grok-${os}-${arch}.exe`
      : `grok-${os}-${arch}`;

    setOsInfo({ os, arch, downloadUrl: '', filename });
  };

  const updateDownloadURL = () => {
    if (osInfo.os === 'unknown') return;

    const filename = osInfo.os === 'windows'
      ? `grok-${osInfo.os}-${osInfo.arch}.exe`
      : `grok-${osInfo.os}-${osInfo.arch}`;

    // Build download URL based on release channel
    let downloadUrl: string;
    if (releaseChannel === 'stable') {
      // Use latest stable release
      downloadUrl = `https://github.com/pandeptwidyaop/grok/releases/latest/download/${filename}`;
    } else {
      // Use tag for pre-releases (beta/alpha)
      // Format: v1.0.0-alpha.1, v1.0.0-beta.1
      downloadUrl = `https://github.com/pandeptwidyaop/grok/releases/download/latest-${releaseChannel}/${filename}`;
    }

    setOsInfo(prev => ({ ...prev, downloadUrl, filename }));
  };

  const handleCopy = (text: string, step: string) => {
    navigator.clipboard.writeText(text);
    setCopiedSteps({ ...copiedSteps, [step]: true });
    setTimeout(() => {
      setCopiedSteps({ ...copiedSteps, [step]: false });
    }, 2000);
  };

  const CodeBlock = ({ code, step }: { code: string; step: string }) => (
    <Box
      sx={{
        position: 'relative',
        bgcolor: '#1e1e1e',
        p: { xs: '8px 8px 44px 8px', sm: 2 },
        borderRadius: 1,
        my: 2,
        fontFamily: 'monospace',
        fontSize: { xs: '0.75rem', sm: '0.875rem' },
        color: '#d4d4d4',
        overflow: 'auto',
      }}
    >
      <Button
        size="small"
        startIcon={copiedSteps[step] ? <CheckCircle2 size={16} /> : <Copy size={16} />}
        onClick={() => handleCopy(code, step)}
        sx={{
          position: 'absolute',
          top: { xs: 'auto', sm: 8 },
          bottom: { xs: 4, sm: 'auto' },
          right: { xs: 4, sm: 8 },
          minWidth: 'auto',
          minHeight: 44,
          color: copiedSteps[step] ? '#4ade80' : '#9ca3af',
          fontSize: { xs: '0.75rem', sm: '0.875rem' },
          '&:hover': {
            bgcolor: 'rgba(255,255,255,0.1)',
          },
        }}
      >
        {copiedSteps[step] ? 'Copied!' : 'Copy'}
      </Button>
      <pre
        style={{
          margin: 0,
          whiteSpace: 'pre-wrap',
          wordBreak: 'break-word',
          paddingRight: isMobile ? 0 : '100px',
        }}
      >
        {code}
      </pre>
    </Box>
  );

  const steps = [
    {
      title: '1. Download Grok Client',
      icon: Download,
      content: (
        <Stack spacing={2}>
          <Alert severity="info" icon={<Terminal size={20} />}>
            Detected System: <strong>{osInfo.os}</strong> ({osInfo.arch})
          </Alert>

          <Typography variant="body2" color="text.secondary">
            Select release channel and download the Grok client binary:
          </Typography>

          {/* Release Channel Selector */}
          <Box>
            <Typography variant="subtitle2" sx={{ mb: 1 }}>
              Release Channel
            </Typography>
            <ToggleButtonGroup
              value={releaseChannel}
              exclusive
              onChange={(_, newChannel) => newChannel && setReleaseChannel(newChannel)}
              size="small"
              sx={{
                mb: 2,
                display: 'flex',
                width: { xs: '100%', sm: 'auto' },
                '& .MuiToggleButton-root': {
                  flex: { xs: 1, sm: 'initial' },
                  minHeight: 44,
                  px: { xs: 1.5, sm: 3 },
                  fontSize: { xs: '0.8125rem', sm: '0.875rem' },
                },
              }}
            >
              <Tooltip title="Stable production-ready releases">
                <ToggleButton value="stable">
                  <CheckCircle2 size={16} style={{ marginRight: isMobile ? 4 : 6 }} />
                  Stable
                </ToggleButton>
              </Tooltip>
              <Tooltip title="Beta testing releases (may have bugs)">
                <ToggleButton value="beta">
                  Beta
                </ToggleButton>
              </Tooltip>
              <Tooltip title="Alpha experimental releases (unstable)">
                <ToggleButton value="alpha">
                  Alpha
                </ToggleButton>
              </Tooltip>
            </ToggleButtonGroup>

            {releaseChannel !== 'stable' && (
              <Alert severity="warning" sx={{ mb: 2 }}>
                <Typography variant="body2" fontWeight={600}>
                  ⚠️ {releaseChannel === 'alpha' ? 'Alpha' : 'Beta'} Release Channel
                </Typography>
                <Typography variant="body2" color="text.secondary">
                  {releaseChannel === 'alpha'
                    ? 'Alpha releases are experimental and may be unstable. Use for testing only.'
                    : 'Beta releases are feature-complete but may contain bugs. Suitable for testing.'}
                </Typography>
              </Alert>
            )}
          </Box>

          <Box
            sx={{
              display: 'flex',
              flexDirection: { xs: 'column', sm: 'row' },
              gap: 2,
            }}
          >
            <Button
              variant="contained"
              startIcon={<Download size={18} />}
              href={osInfo.downloadUrl}
              disabled={osInfo.os === 'unknown' || !osInfo.downloadUrl}
              sx={{
                bgcolor: releaseChannel !== 'stable' ? '#f59e0b' : '#667eea',
                '&:hover': { bgcolor: releaseChannel !== 'stable' ? '#d97706' : '#5568d3' },
                minHeight: 44,
                width: { xs: '100%', sm: 'auto' },
              }}
            >
              Download {releaseChannel !== 'stable' ? releaseChannel.toUpperCase() : ''} for {osInfo.os} ({osInfo.arch})
            </Button>
            <Button
              variant="outlined"
              href={`https://github.com/pandeptwidyaop/grok/releases${releaseChannel === 'stable' ? '/latest' : ''}`}
              target="_blank"
              sx={{
                minHeight: 44,
                width: { xs: '100%', sm: 'auto' },
              }}
            >
              View All {releaseChannel !== 'stable' ? releaseChannel.charAt(0).toUpperCase() + releaseChannel.slice(1) : ''} Releases
            </Button>
          </Box>

          <Typography variant="body2" color="text.secondary" sx={{ mt: 2 }}>
            For Linux/macOS, make the binary executable:
          </Typography>
          <CodeBlock
            code={`chmod +x ${osInfo.filename}\nsudo mv ${osInfo.filename} /usr/local/bin/grok`}
            step="download"
          />

          <Typography variant="body2" color="text.secondary">
            Verify installation:
          </Typography>
          <CodeBlock code="grok --version" step="verify" />
        </Stack>
      ),
    },
    {
      title: '2. Create Authentication Token',
      icon: Key,
      content: (
        <Stack spacing={2}>
          <Typography variant="body2" color="text.secondary">
            Go to the <strong>Auth Tokens</strong> page in the dashboard and create a new token.
          </Typography>

          <Alert severity="warning">
            Save your token securely! It will only be shown once.
          </Alert>

          <Typography variant="body2" color="text.secondary" sx={{ mt: 2 }}>
            Your token will look like:
          </Typography>
          <CodeBlock code="grok_a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6" step="token-example" />

          <Button
            variant="contained"
            href="#/auth-tokens"
            sx={{ alignSelf: 'flex-start', bgcolor: '#667eea', '&:hover': { bgcolor: '#5568d3' } }}
          >
            Go to Auth Tokens
          </Button>
        </Stack>
      ),
    },
    {
      title: '3. Configure Grok Client',
      icon: Server,
      content: (
        <Stack spacing={2}>
          <Typography variant="body2" color="text.secondary">
            Set your Grok server host and register your authentication token:
          </Typography>

          {isUsingTLS && (
            <Alert severity="success" icon={<CheckCircle2 size={20} />}>
              <Typography variant="body2" fontWeight={600}>
                🔒 TLS Enabled Server
              </Typography>
              <Typography variant="body2" color="text.secondary">
                This server is using TLS/HTTPS. The <code>--tls</code> flag will be used for secure connections.
              </Typography>
            </Alert>
          )}

          <Typography variant="subtitle2" sx={{ mt: 2 }}>
            Step 1: Set Server Host {isUsingTLS && '(with TLS)'}
          </Typography>
          <CodeBlock
            code={`grok config set-server ${serverAddress}${isUsingTLS ? ' --tls' : ''}`}
            step="config-server"
          />
          {isUsingTLS && (
            <Typography variant="caption" color="text.secondary">
              The <code>--tls</code> flag enables secure TLS connection with system CA pool
            </Typography>
          )}

          <Typography variant="subtitle2" sx={{ mt: 2 }}>
            Step 2: Set Authentication Token
          </Typography>
          <CodeBlock
            code={`grok config set-token grok_your_token_here`}
            step="config-token"
          />

          <Alert severity="info">
            Configuration will be saved to <code>~/.grok/config.yaml</code>
          </Alert>

          <Typography variant="subtitle2" sx={{ mt: 2 }}>
            Verify Configuration
          </Typography>
          <CodeBlock
            code={`# View current config
cat ~/.grok/config.yaml`}
            step="config-verify"
          />

          <Typography variant="body2" color="text.secondary" sx={{ mt: 2 }}>
            Alternative: You can also use CLI flags for one-time use:
          </Typography>
          <CodeBlock
            code={`grok http 3000 --host ${serverAddress}${isUsingTLS ? ' --tls' : ''} --token your_token_here`}
            step="config-cli"
          />
        </Stack>
      ),
    },
    {
      title: '4. Create HTTP Tunnel',
      icon: Globe,
      content: (
        <Stack spacing={2}>
          <Typography variant="body2" color="text.secondary">
            Expose a local HTTP service to the internet:
          </Typography>

          <Typography variant="subtitle2" sx={{ mt: 2 }}>
            Basic HTTP Tunnel (Auto-generated subdomain)
          </Typography>
          <CodeBlock code="grok http 3000" step="http-basic" />

          <Typography variant="subtitle2" sx={{ mt: 2 }}>
            Custom Subdomain
          </Typography>
          <CodeBlock code="grok http 3000 --subdomain myapp" step="http-subdomain" />
          <Typography variant="caption" color="text.secondary">
            This will create: https://myapp.{serverDomain}
          </Typography>

          <Typography variant="subtitle2" sx={{ mt: 2 }}>
            Named Tunnel (with custom name)
          </Typography>
          <CodeBlock code="grok http 3000 --name my-api --subdomain api" step="http-named" />
          <Typography variant="caption" color="text.secondary">
            Use <code>--name</code> to give your tunnel a persistent name instead of random ID
          </Typography>

          <Typography variant="subtitle2" sx={{ mt: 2 }}>
            With Custom Host Header
          </Typography>
          <CodeBlock code="grok http localhost:8080 --subdomain backend" step="http-host" />

          <Alert severity="success" icon={<CheckCircle2 size={20} />}>
            Your tunnel will appear in the <strong>Tunnels</strong> page immediately!
          </Alert>
        </Stack>
      ),
    },
    {
      title: '5. Create TCP Tunnel',
      icon: Network,
      content: (
        <Stack spacing={2}>
          <Typography variant="body2" color="text.secondary">
            Expose any TCP service (SSH, database, custom protocols):
          </Typography>

          <Typography variant="subtitle2" sx={{ mt: 2 }}>
            SSH Server (Auto-generated name)
          </Typography>
          <CodeBlock code="grok tcp 22" step="tcp-ssh" />
          <Typography variant="caption" color="text.secondary">
            Connect via: ssh user@{serverDomain} -p [assigned-port]
          </Typography>

          <Typography variant="subtitle2" sx={{ mt: 2 }}>
            Named SSH Tunnel
          </Typography>
          <CodeBlock code="grok tcp 22 --name my-ssh" step="tcp-ssh-named" />
          <Typography variant="caption" color="text.secondary">
            Use <code>--name</code> to identify your tunnel easily in the dashboard
          </Typography>

          <Typography variant="subtitle2" sx={{ mt: 2 }}>
            Database (PostgreSQL)
          </Typography>
          <CodeBlock code="grok tcp 5432 --name postgres-db" step="tcp-postgres" />

          <Typography variant="subtitle2" sx={{ mt: 2 }}>
            Custom Service
          </Typography>
          <CodeBlock code="grok tcp 8080 --name custom-app" step="tcp-custom" />

          <Alert severity="info">
            TCP tunnels are assigned a random port. Check the <strong>Tunnels</strong> page for the public address.
          </Alert>
        </Stack>
      ),
    },
    {
      title: '6. Setup Webhooks',
      icon: Webhook,
      content: (
        <Stack spacing={2}>
          <Typography variant="body2" color="text.secondary">
            Configure webhooks to receive real-time notifications about tunnel events:
          </Typography>

          <Typography variant="subtitle2" sx={{ mt: 2 }}>
            Step 1: Create Webhook App
          </Typography>
          <Typography variant="body2" color="text.secondary">
            Go to the <strong>Webhooks</strong> page and create a new webhook application.
          </Typography>

          <Button
            variant="contained"
            href="#/webhooks"
            sx={{ alignSelf: 'flex-start', bgcolor: '#667eea', '&:hover': { bgcolor: '#5568d3' } }}
          >
            Go to Webhooks
          </Button>

          <Typography variant="subtitle2" sx={{ mt: 2 }}>
            Step 2: Configure Webhook URL
          </Typography>
          <CodeBlock code="https://your-app.com/api/webhooks/grok" step="webhook-url" />

          <Typography variant="subtitle2" sx={{ mt: 2 }}>
            Step 3: Select Events
          </Typography>
          <Box sx={{ display: 'flex', gap: 1, flexWrap: 'wrap' }}>
            <Chip label="tunnel.created" size="small" />
            <Chip label="tunnel.closed" size="small" />
            <Chip label="request.received" size="small" />
          </Box>

          <Typography variant="subtitle2" sx={{ mt: 2 }}>
            Example Webhook Payload
          </Typography>
          <CodeBlock
            code={`{
  "event": "tunnel.created",
  "timestamp": "2024-01-15T10:30:00Z",
  "data": {
    "tunnel_id": "123e4567-e89b-12d3-a456-426614174000",
    "subdomain": "myapp",
    "public_url": "https://myapp.${serverDomain}",
    "local_addr": "localhost:3000",
    "protocol": "http"
  }
}`}
            step="webhook-payload"
          />
        </Stack>
      ),
    },
  ];

  return (
    <Card elevation={2} sx={{ mb: 4 }}>
      <CardContent>
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 2, mb: 3 }}>
          <Box
            sx={{
              width: 48,
              height: 48,
              borderRadius: '12px',
              bgcolor: '#667eea',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
            }}
          >
            <Download size={24} color="white" />
          </Box>
          <Box>
            <Typography variant="h5" sx={{ fontWeight: 700, color: '#667eea' }}>
              Getting Started
            </Typography>
            <Typography variant="body2" color="text.secondary">
              Complete tutorial to set up and use Grok tunneling system
            </Typography>
          </Box>
        </Box>

        {/* Server Information */}
        <Alert
          severity={isUsingTLS ? "success" : "info"}
          icon={<Server size={20} />}
          sx={{ mb: 3 }}
        >
          <Typography variant="body2" fontWeight={600} sx={{ mb: 1 }}>
            Server Information
          </Typography>
          <Box sx={{ display: 'flex', flexDirection: 'column', gap: 0.5 }}>
            <Typography variant="body2" component="div">
              <strong>Domain:</strong> <code>{serverDomain}</code>
            </Typography>
            <Typography variant="body2" component="div">
              <strong>gRPC Port:</strong> <code>{grpcPort}</code> (Client Connection)
            </Typography>
            <Typography variant="body2" component="div">
              <strong>API Port:</strong> <code>{apiPort}</code> (Dashboard/API)
            </Typography>
            <Typography variant="body2" component="div">
              <strong>TLS/HTTPS:</strong> <code>{isUsingTLS ? '✓ Enabled (Secure)' : '✗ Disabled (Insecure)'}</code>
            </Typography>
            <Typography variant="body2" component="div" sx={{ mt: 1 }}>
              <strong>Server Address:</strong> <code style={{
                backgroundColor: isUsingTLS ? '#10b981' : '#667eea',
                color: 'white',
                padding: '2px 8px',
                borderRadius: '4px'
              }}>{serverAddress}</code>
            </Typography>
          </Box>
        </Alert>

        {steps.map((step, index) => {
          const IconComponent = step.icon;
          return (
            <Accordion
              key={index}
              sx={{
                mb: 1,
                '&:before': { display: 'none' },
                boxShadow: 'none',
                border: '1px solid',
                borderColor: 'divider',
              }}
            >
              <AccordionSummary
                expandIcon={<ChevronDown />}
                sx={{
                  '&:hover': {
                    bgcolor: 'action.hover',
                  },
                }}
              >
                <Box sx={{ display: 'flex', alignItems: 'center', gap: 2 }}>
                  <Box
                    sx={{
                      width: 32,
                      height: 32,
                      borderRadius: '8px',
                      bgcolor: '#667eea',
                      display: 'flex',
                      alignItems: 'center',
                      justifyContent: 'center',
                    }}
                  >
                    <IconComponent size={18} color="white" />
                  </Box>
                  <Typography fontWeight={600}>{step.title}</Typography>
                </Box>
              </AccordionSummary>
              <AccordionDetails>
                {step.content}
              </AccordionDetails>
            </Accordion>
          );
        })}

        <Box sx={{ mt: 3, display: 'flex', flexDirection: 'column', gap: 2 }}>
          <Alert severity="info" icon={<Globe size={20} />}>
            <Typography variant="body2" fontWeight={600} sx={{ mb: 1 }}>
              📚 Full Documentation
            </Typography>
            <Typography variant="body2" color="text.secondary" sx={{ mb: 1 }}>
              For complete documentation, advanced configuration, API reference, and more examples, visit:
            </Typography>
            <Button
              variant="outlined"
              size="small"
              href="https://github.com/pandeptwidyaop/grok"
              target="_blank"
              rel="noopener noreferrer"
              startIcon={<Globe size={16} />}
              sx={{ alignSelf: 'flex-start' }}
            >
              GitHub Repository
            </Button>
          </Alert>

          <Alert severity="success" icon={<CheckCircle2 size={20} />}>
            <Typography variant="body2" fontWeight={600}>
              💬 Need Help?
            </Typography>
            <Typography variant="body2" color="text.secondary">
              Found a bug or have questions? Open an issue on{' '}
              <a
                href="https://github.com/pandeptwidyaop/grok/issues"
                target="_blank"
                rel="noopener noreferrer"
                style={{ color: '#667eea', fontWeight: 600 }}
              >
                GitHub Issues
              </a>{' '}
              or check the{' '}
              <a
                href="https://github.com/pandeptwidyaop/grok/discussions"
                target="_blank"
                rel="noopener noreferrer"
                style={{ color: '#667eea', fontWeight: 600 }}
              >
                Discussions
              </a>{' '}
              for community support.
            </Typography>
          </Alert>
        </Box>
      </CardContent>
    </Card>
  );
}

export default GettingStarted;
