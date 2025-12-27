import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import {
  Box,
  Card,
  CardContent,
  Typography,
  Button,
  TextField,
  Alert,
  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
  CircularProgress,
  Chip,
  Divider,
  Stack,
} from '@mui/material';
import { Shield, ShieldOff, Key } from 'lucide-react';
import { QRCodeSVG } from 'qrcode.react';
import { api } from '@/lib/api';

export default function TwoFASettings() {
  const queryClient = useQueryClient();
  const [enableDialogOpen, setEnableDialogOpen] = useState(false);
  const [disableDialogOpen, setDisableDialogOpen] = useState(false);
  const [verifyDialogOpen, setVerifyDialogOpen] = useState(false);
  const [otpCode, setOtpCode] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [qrUrl, setQrUrl] = useState('');
  const [secret, setSecret] = useState('');

  // Get 2FA status
  const { data: status, isLoading } = useQuery({
    queryKey: ['2fa-status'],
    queryFn: async () => {
      const response = await api.twoFA.getStatus();
      return response.data;
    },
  });

  // Setup 2FA mutation
  const setupMutation = useMutation({
    mutationFn: async () => {
      const response = await api.twoFA.setup();
      return response.data;
    },
    onSuccess: (data) => {
      setQrUrl(data.qr_url);
      setSecret(data.secret);
      setEnableDialogOpen(false);
      setVerifyDialogOpen(true);
      setError('');
    },
    onError: (err: any) => {
      setError(err.response?.data?.error || 'Failed to setup 2FA');
    },
  });

  // Verify 2FA mutation
  const verifyMutation = useMutation({
    mutationFn: async (code: string) => {
      await api.twoFA.verify({ code });
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['2fa-status'] });
      setVerifyDialogOpen(false);
      setOtpCode('');
      setSecret('');
      setQrUrl('');
      setError('');
    },
    onError: (err: any) => {
      setError(err.response?.data?.error || 'Invalid OTP code');
    },
  });

  // Disable 2FA mutation
  const disableMutation = useMutation({
    mutationFn: async (password: string) => {
      await api.twoFA.disable({ password });
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['2fa-status'] });
      setDisableDialogOpen(false);
      setPassword('');
      setError('');
    },
    onError: (err: any) => {
      setError(err.response?.data?.error || 'Failed to disable 2FA');
    },
  });

  const handleEnableClick = () => {
    setError('');
    setEnableDialogOpen(true);
  };

  const handleSetup = () => {
    setupMutation.mutate();
  };

  const handleVerify = () => {
    if (otpCode.length !== 6) {
      setError('Please enter a 6-digit code');
      return;
    }
    verifyMutation.mutate(otpCode);
  };

  const handleDisableClick = () => {
    setError('');
    setDisableDialogOpen(true);
  };

  const handleDisable = () => {
    if (!password) {
      setError('Password is required');
      return;
    }
    disableMutation.mutate(password);
  };

  if (isLoading) {
    return (
      <Box sx={{ display: 'flex', justifyContent: 'center', p: 4 }}>
        <CircularProgress />
      </Box>
    );
  }

  return (
    <Box>
      <Card
        sx={{
          background: 'linear-gradient(135deg, rgba(102, 126, 234, 0.1) 0%, rgba(118, 75, 162, 0.1) 100%)',
          border: '1px solid rgba(102, 126, 234, 0.2)',
        }}
      >
        <CardContent sx={{ p: 3 }}>
          <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', mb: 3 }}>
            <Box sx={{ display: 'flex', alignItems: 'center', gap: 2 }}>
              {status?.enabled ? (
                <Shield size={32} color="#4caf50" />
              ) : (
                <ShieldOff size={32} color="#9e9e9e" />
              )}
              <Box>
                <Typography variant="h5" sx={{ fontWeight: 700 }}>
                  Two-Factor Authentication
                </Typography>
                <Typography variant="body2" color="text.secondary">
                  Add an extra layer of security to your account
                </Typography>
              </Box>
            </Box>
            <Chip
              label={status?.enabled ? 'Enabled' : 'Disabled'}
              color={status?.enabled ? 'success' : 'default'}
              sx={{ fontWeight: 600 }}
            />
          </Box>

          <Divider sx={{ my: 3 }} />

          <Typography variant="body1" sx={{ mb: 3 }}>
            {status?.enabled
              ? 'Two-factor authentication is currently enabled on your account. You\'ll need to enter a code from your authenticator app when signing in.'
              : 'Protect your account with two-factor authentication. You\'ll need to enter a code from your authenticator app each time you sign in.'}
          </Typography>

          {status?.enabled ? (
            <Button
              variant="outlined"
              color="error"
              startIcon={<ShieldOff size={18} />}
              onClick={handleDisableClick}
            >
              Disable 2FA
            </Button>
          ) : (
            <Button
              variant="contained"
              color="primary"
              startIcon={<Shield size={18} />}
              onClick={handleEnableClick}
            >
              Enable 2FA
            </Button>
          )}
        </CardContent>
      </Card>

      {/* Enable 2FA Dialog */}
      <Dialog open={enableDialogOpen} onClose={() => setEnableDialogOpen(false)} maxWidth="sm" fullWidth>
        <DialogTitle>Enable Two-Factor Authentication</DialogTitle>
        <DialogContent>
          <Typography variant="body2" sx={{ mb: 2 }}>
            We'll generate a QR code that you can scan with your authenticator app (Google Authenticator, Authy, etc.).
          </Typography>
          {error && (
            <Alert severity="error" sx={{ mb: 2 }}>
              {error}
            </Alert>
          )}
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setEnableDialogOpen(false)}>Cancel</Button>
          <Button
            onClick={handleSetup}
            variant="contained"
            disabled={setupMutation.isPending}
          >
            {setupMutation.isPending ? <CircularProgress size={20} /> : 'Continue'}
          </Button>
        </DialogActions>
      </Dialog>

      {/* Verify 2FA Dialog */}
      <Dialog open={verifyDialogOpen} onClose={() => setVerifyDialogOpen(false)} maxWidth="sm" fullWidth>
        <DialogTitle>Scan QR Code</DialogTitle>
        <DialogContent>
          <Stack spacing={3}>
            <Typography variant="body2">
              Scan this QR code with your authenticator app:
            </Typography>

            {qrUrl && (
              <Box
                sx={{
                  display: 'flex',
                  justifyContent: 'center',
                  p: 2,
                  bgcolor: 'white',
                  borderRadius: 2,
                }}
              >
                <QRCodeSVG value={qrUrl} size={256} level="H" />
              </Box>
            )}

            {secret && (
              <Alert severity="info" icon={<Key size={18} />}>
                <Typography variant="caption" sx={{ fontWeight: 600, display: 'block', mb: 0.5 }}>
                  Manual Entry Key:
                </Typography>
                <Typography
                  variant="body2"
                  sx={{
                    fontFamily: 'monospace',
                    wordBreak: 'break-all',
                    bgcolor: 'rgba(0,0,0,0.05)',
                    p: 1,
                    borderRadius: 1,
                  }}
                >
                  {secret}
                </Typography>
              </Alert>
            )}

            <TextField
              fullWidth
              label="Verification Code"
              value={otpCode}
              onChange={(e) => {
                const value = e.target.value.replace(/\D/g, '');
                if (value.length <= 6) {
                  setOtpCode(value);
                }
              }}
              placeholder="Enter 6-digit code"
              inputProps={{
                maxLength: 6,
                pattern: '[0-9]*',
                inputMode: 'numeric',
              }}
              helperText="Enter the 6-digit code from your authenticator app"
            />

            {error && <Alert severity="error">{error}</Alert>}
          </Stack>
        </DialogContent>
        <DialogActions>
          <Button
            onClick={() => {
              setVerifyDialogOpen(false);
              setOtpCode('');
              setError('');
            }}
          >
            Cancel
          </Button>
          <Button
            onClick={handleVerify}
            variant="contained"
            disabled={verifyMutation.isPending || otpCode.length !== 6}
          >
            {verifyMutation.isPending ? <CircularProgress size={20} /> : 'Verify & Enable'}
          </Button>
        </DialogActions>
      </Dialog>

      {/* Disable 2FA Dialog */}
      <Dialog open={disableDialogOpen} onClose={() => setDisableDialogOpen(false)} maxWidth="sm" fullWidth>
        <DialogTitle>Disable Two-Factor Authentication</DialogTitle>
        <DialogContent>
          <Typography variant="body2" sx={{ mb: 2 }}>
            Please enter your password to confirm disabling 2FA.
          </Typography>
          <TextField
            fullWidth
            type="password"
            label="Password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            autoComplete="current-password"
          />
          {error && (
            <Alert severity="error" sx={{ mt: 2 }}>
              {error}
            </Alert>
          )}
        </DialogContent>
        <DialogActions>
          <Button
            onClick={() => {
              setDisableDialogOpen(false);
              setPassword('');
              setError('');
            }}
          >
            Cancel
          </Button>
          <Button
            onClick={handleDisable}
            variant="contained"
            color="error"
            disabled={disableMutation.isPending}
          >
            {disableMutation.isPending ? <CircularProgress size={20} /> : 'Disable 2FA'}
          </Button>
        </DialogActions>
      </Dialog>
    </Box>
  );
}
