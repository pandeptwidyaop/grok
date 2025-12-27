import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useAuth } from '@/contexts/AuthContext';
import {
  Box,
  Button,
  Card,
  CardContent,
  TextField,
  Typography,
  Alert,
  CircularProgress,
  Container,
  Paper,
  IconButton,
  useMediaQuery,
  useTheme,
} from '@mui/material';
import { ArrowRight, ArrowLeft, Shield } from 'lucide-react';

export default function Login() {
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [otpCode, setOtpCode] = useState('');
  const [requires2FA, setRequires2FA] = useState(false);
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);
  const { login } = useAuth();
  const navigate = useNavigate();
  const theme = useTheme();
  const isMobile = useMediaQuery(theme.breakpoints.down('sm'));

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');
    setLoading(true);

    try {
      const result = await login(username, password, otpCode || undefined);

      // Check if 2FA is required
      if (result.requires_2fa) {
        setRequires2FA(true);
        setLoading(false);
        return;
      }

      // Successful login - navigate to dashboard first
      navigate('/');

      // Refresh after navigation to ensure clean state
      // Use setTimeout to allow navigation to complete first
      setTimeout(() => {
        window.location.reload();
      }, 100);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Login failed');
    } finally {
      setLoading(false);
    }
  };

  const handleBackToLogin = () => {
    setRequires2FA(false);
    setOtpCode('');
    setError('');
  };

  return (
    <Box
      sx={{
        minHeight: '100vh',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        background: 'linear-gradient(135deg, #667eea 0%, #764ba2 100%)',
        position: 'relative',
        overflow: 'hidden',
      }}
    >
      {/* Animated background elements (hidden on mobile) */}
      {!isMobile && (
        <>
          <Box
            sx={{
              position: 'absolute',
              top: 80,
              left: 80,
              width: 300,
              height: 300,
              background: 'rgba(255, 255, 255, 0.1)',
              borderRadius: '50%',
              filter: 'blur(60px)',
              animation: 'pulse 3s ease-in-out infinite',
            }}
          />
          <Box
            sx={{
              position: 'absolute',
              bottom: 80,
              right: 80,
              width: 400,
              height: 400,
              background: 'rgba(255, 255, 255, 0.1)',
              borderRadius: '50%',
              filter: 'blur(60px)',
              animation: 'pulse 3s ease-in-out infinite 1s',
            }}
          />
        </>
      )}

      <Container maxWidth="sm" sx={{ position: 'relative', zIndex: 10, px: 2 }}>
        {/* Logo/Brand */}
        <Box sx={{ textAlign: 'center', mb: { xs: 3, sm: 4 } }}>
          <Paper
            elevation={0}
            sx={{
              display: 'inline-flex',
              alignItems: 'center',
              justifyContent: 'center',
              width: { xs: 64, sm: 80 },
              height: { xs: 64, sm: 80 },
              borderRadius: 3,
              bgcolor: 'rgba(255, 255, 255, 0.15)',
              backdropFilter: 'blur(10px)',
              mb: 2,
              p: 1.5,
            }}
          >
            <img
              src="/favicon.svg"
              alt="Grok Logo"
              style={{ width: '100%', height: '100%' }}
            />
          </Paper>
          <Typography
            variant="h3"
            sx={{
              color: 'white',
              fontWeight: 700,
              mb: 1,
              fontSize: { xs: '1.75rem', sm: '2.25rem', md: '3rem' },
            }}
          >
            Welcome Back
          </Typography>
          <Typography
            variant="body1"
            sx={{
              color: 'rgba(255, 255, 255, 0.8)',
              fontSize: { xs: '0.9375rem', sm: '1rem' },
            }}
          >
            Sign in to access your Grok Dashboard
          </Typography>
        </Box>

        {/* Login Card */}
        <Card
          elevation={8}
          sx={{
            borderRadius: 3,
            backdropFilter: 'blur(10px)',
            bgcolor: 'rgba(255, 255, 255, 0.95)',
          }}
        >
          <CardContent sx={{ p: { xs: 2, sm: 3, md: 4 } }}>
            {!requires2FA ? (
              // Login Form
              <>
                <Typography variant="h4" sx={{ color: 'primary.main', fontWeight: 700, mb: 1 }}>
                  Sign In
                </Typography>
                <Typography variant="body2" color="text.secondary" sx={{ mb: 3 }}>
                  Enter your credentials to continue
                </Typography>

                <form onSubmit={handleSubmit}>
                  {error && (
                    <Alert severity="error" sx={{ mb: 3 }}>
                      {error}
                    </Alert>
                  )}

                  <TextField
                    fullWidth
                    label="Username"
                    type="text"
                    value={username}
                    onChange={(e) => setUsername(e.target.value)}
                    required
                    disabled={loading}
                    sx={{ mb: 3 }}
                    autoComplete="username"
                    autoFocus
                  />

                  <TextField
                    fullWidth
                    label="Password"
                    type="password"
                    value={password}
                    onChange={(e) => setPassword(e.target.value)}
                    required
                    disabled={loading}
                    sx={{ mb: 4 }}
                    autoComplete="current-password"
                  />

                  <Button
                    type="submit"
                    fullWidth
                    variant="contained"
                    size="large"
                    disabled={loading}
                    sx={{
                      height: 48,
                      fontWeight: 600,
                      boxShadow: 3,
                      '&:hover': {
                        boxShadow: 6,
                      },
                    }}
                  >
                    {loading ? (
                      <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                        <CircularProgress size={20} color="inherit" />
                        Verifying...
                      </Box>
                    ) : (
                      <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                        Continue
                        <ArrowRight size={18} />
                      </Box>
                    )}
                  </Button>
                </form>
              </>
            ) : (
              // 2FA Form
              <>
                <Box sx={{ display: 'flex', alignItems: 'center', mb: 2 }}>
                  <IconButton onClick={handleBackToLogin} sx={{ mr: 1 }}>
                    <ArrowLeft size={20} />
                  </IconButton>
                  <Box sx={{ flex: 1 }}>
                    <Typography variant="h4" sx={{ color: 'primary.main', fontWeight: 700 }}>
                      Two-Factor Authentication
                    </Typography>
                  </Box>
                </Box>

                <Box
                  sx={{
                    display: 'flex',
                    justifyContent: 'center',
                    mb: 3,
                  }}
                >
                  <Box
                    sx={{
                      width: 80,
                      height: 80,
                      borderRadius: '50%',
                      bgcolor: 'rgba(102, 126, 234, 0.1)',
                      display: 'flex',
                      alignItems: 'center',
                      justifyContent: 'center',
                    }}
                  >
                    <Shield size={40} color="#667eea" />
                  </Box>
                </Box>

                <Typography variant="body2" color="text.secondary" sx={{ mb: 1, textAlign: 'center' }}>
                  Enter the 6-digit verification code from your authenticator app
                </Typography>
                <Typography variant="caption" color="text.secondary" sx={{ mb: 3, display: 'block', textAlign: 'center' }}>
                  Logging in as: <strong>{username}</strong>
                </Typography>

                <form onSubmit={handleSubmit}>
                  {error && (
                    <Alert severity="error" sx={{ mb: 3 }}>
                      {error}
                    </Alert>
                  )}

                  <TextField
                    fullWidth
                    label="Verification Code"
                    type="text"
                    value={otpCode}
                    onChange={(e) => {
                      const value = e.target.value.replace(/\D/g, '');
                      if (value.length <= 6) {
                        setOtpCode(value);
                      }
                    }}
                    required
                    disabled={loading}
                    sx={{ mb: 4 }}
                    placeholder="000000"
                    autoComplete="one-time-code"
                    autoFocus
                    inputProps={{
                      maxLength: 6,
                      pattern: '[0-9]*',
                      inputMode: 'numeric',
                      style: { textAlign: 'center', fontSize: '24px', letterSpacing: '8px' },
                    }}
                  />

                  <Button
                    type="submit"
                    fullWidth
                    variant="contained"
                    size="large"
                    disabled={loading || otpCode.length !== 6}
                    sx={{
                      height: 48,
                      fontWeight: 600,
                      boxShadow: 3,
                      '&:hover': {
                        boxShadow: 6,
                      },
                    }}
                  >
                    {loading ? (
                      <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                        <CircularProgress size={20} color="inherit" />
                        Verifying...
                      </Box>
                    ) : (
                      <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                        Verify & Sign in
                        <ArrowRight size={18} />
                      </Box>
                    )}
                  </Button>
                </form>
              </>
            )}
          </CardContent>
        </Card>

        {/* Footer */}
        <Typography
          variant="body2"
          sx={{ textAlign: 'center', mt: 3, color: 'rgba(255, 255, 255, 0.6)' }}
        >
          Powered by Grok Tunnel
        </Typography>
      </Container>

      {/* CSS Animations */}
      <style>
        {`
          @keyframes pulse {
            0%, 100% {
              opacity: 0.6;
            }
            50% {
              opacity: 1;
            }
          }
        `}
      </style>
    </Box>
  );
}
