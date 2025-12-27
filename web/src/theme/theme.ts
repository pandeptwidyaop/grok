import { createTheme } from '@mui/material/styles';

// Grok blue tech theme matching the original design
export const theme = createTheme({
  palette: {
    mode: 'light',
    primary: {
      main: '#2563eb', // Blue-600
      light: '#3b82f6', // Blue-500
      dark: '#1d4ed8', // Blue-700
      contrastText: '#ffffff',
    },
    secondary: {
      main: '#7c3aed', // Purple-600
      light: '#8b5cf6', // Purple-500
      dark: '#6d28d9', // Purple-700
      contrastText: '#ffffff',
    },
    success: {
      main: '#10b981', // Green-500
      light: '#34d399', // Green-400
      dark: '#059669', // Green-600
    },
    error: {
      main: '#ef4444', // Red-500
      light: '#f87171', // Red-400
      dark: '#dc2626', // Red-600
    },
    warning: {
      main: '#f59e0b', // Amber-500
      light: '#fbbf24', // Amber-400
      dark: '#d97706', // Amber-600
    },
    info: {
      main: '#3b82f6', // Blue-500
      light: '#60a5fa', // Blue-400
      dark: '#2563eb', // Blue-600
    },
    background: {
      default: '#f9fafb', // Gray-50
      paper: '#ffffff',
    },
    text: {
      primary: '#111827', // Gray-900
      secondary: '#6b7280', // Gray-500
    },
  },
  typography: {
    fontFamily: [
      '-apple-system',
      'BlinkMacSystemFont',
      '"Segoe UI"',
      'Roboto',
      '"Helvetica Neue"',
      'Arial',
      'sans-serif',
    ].join(','),
    h1: {
      fontSize: '1.75rem', // Mobile: 28px
      fontWeight: 700,
      lineHeight: 1.2,
      '@media (min-width:600px)': {
        fontSize: '2rem', // Tablet: 32px
      },
      '@media (min-width:900px)': {
        fontSize: '2.25rem', // Desktop: 36px
      },
    },
    h2: {
      fontSize: '1.5rem', // Mobile: 24px
      fontWeight: 700,
      lineHeight: 1.3,
      '@media (min-width:600px)': {
        fontSize: '1.75rem', // Tablet: 28px
      },
      '@media (min-width:900px)': {
        fontSize: '1.875rem', // Desktop: 30px
      },
    },
    h3: {
      fontSize: '1.25rem', // Mobile: 20px
      fontWeight: 600,
      lineHeight: 1.4,
      '@media (min-width:600px)': {
        fontSize: '1.375rem', // Tablet: 22px
      },
      '@media (min-width:900px)': {
        fontSize: '1.5rem', // Desktop: 24px
      },
    },
    h4: {
      fontSize: '1.125rem', // Mobile: 18px
      fontWeight: 600,
      lineHeight: 1.4,
      '@media (min-width:600px)': {
        fontSize: '1.1875rem', // Tablet: 19px
      },
      '@media (min-width:900px)': {
        fontSize: '1.25rem', // Desktop: 20px
      },
    },
    h5: {
      fontSize: '1rem', // Mobile: 16px
      fontWeight: 600,
      lineHeight: 1.5,
      '@media (min-width:600px)': {
        fontSize: '1.0625rem', // Tablet: 17px
      },
      '@media (min-width:900px)': {
        fontSize: '1.125rem', // Desktop: 18px
      },
    },
    h6: {
      fontSize: '0.9375rem', // Mobile: 15px
      fontWeight: 600,
      lineHeight: 1.5,
      '@media (min-width:600px)': {
        fontSize: '0.96875rem', // Tablet: 15.5px
      },
      '@media (min-width:900px)': {
        fontSize: '1rem', // Desktop: 16px
      },
    },
    body1: {
      fontSize: '0.9375rem', // Mobile: 15px
      '@media (min-width:600px)': {
        fontSize: '0.96875rem', // Tablet: 15.5px
      },
      '@media (min-width:900px)': {
        fontSize: '1rem', // Desktop: 16px
      },
    },
    body2: {
      fontSize: '0.875rem', // Mobile: 14px
      '@media (min-width:600px)': {
        fontSize: '0.90625rem', // Tablet: 14.5px
      },
      '@media (min-width:900px)': {
        fontSize: '0.9375rem', // Desktop: 15px
      },
    },
    button: {
      textTransform: 'none', // No uppercase for buttons
      fontWeight: 500,
      fontSize: '0.9375rem', // Mobile: 15px
      '@media (min-width:600px)': {
        fontSize: '0.96875rem', // Tablet: 15.5px
      },
      '@media (min-width:900px)': {
        fontSize: '1rem', // Desktop: 16px
      },
    },
  },
  shape: {
    borderRadius: 8,
  },
  components: {
    MuiButton: {
      styleOverrides: {
        root: {
          borderRadius: 8,
          padding: '8px 16px',
          fontWeight: 500,
        },
        contained: {
          boxShadow: 'none',
          '&:hover': {
            boxShadow: 'none',
          },
        },
      },
    },
    MuiCard: {
      styleOverrides: {
        root: {
          borderRadius: 12,
          boxShadow: '0 1px 3px 0 rgb(0 0 0 / 0.1), 0 1px 2px -1px rgb(0 0 0 / 0.1)',
        },
      },
    },
    MuiChip: {
      styleOverrides: {
        root: {
          borderRadius: 16,
          fontWeight: 500,
        },
      },
    },
    MuiTextField: {
      defaultProps: {
        variant: 'outlined',
      },
    },
    MuiPaper: {
      styleOverrides: {
        root: {
          backgroundImage: 'none',
        },
        elevation1: {
          boxShadow: '0 1px 3px 0 rgb(0 0 0 / 0.1), 0 1px 2px -1px rgb(0 0 0 / 0.1)',
        },
      },
    },
  },
});

export default theme;
