import React, { Suspense, lazy, useState, useEffect } from 'react';
import { BrowserRouter as Router, Route, Routes } from 'react-router-dom';
import { ThemeProvider, createTheme } from '@mui/material/styles';
import CssBaseline from '@mui/material/CssBaseline';
import Box from '@mui/material/Box';
import { CircularProgress, Alert, Typography, Button } from '@mui/material';

// Components
import Navbar from './components/Navbar';
import Sidebar from './components/Sidebar';

// Pages - Using eager loading for Dashboard to ensure it loads quickly
import Dashboard from './pages/Dashboard';

// Lazy load other less critical pages
const Historical = lazy(() => import('./pages/Historical'));
const Signals = lazy(() => import('./pages/Signals'));
const Backtest = lazy(() => import('./pages/Backtest'));
const Recommendations = lazy(() => import('./pages/Recommendations'));

// Error boundary component
class ErrorBoundary extends React.Component {
  constructor(props) {
    super(props);
    this.state = { hasError: false, error: null };
  }

  static getDerivedStateFromError(error) {
    return { hasError: true, error };
  }

  componentDidCatch(error, errorInfo) {
    console.error('React Error Boundary caught an error:', error, errorInfo);
  }

  render() {
    if (this.state.hasError) {
      return (
        <Box 
          sx={{ 
            display: 'flex', 
            flexDirection: 'column',
            alignItems: 'center', 
            justifyContent: 'center',
            p: 3,
            height: '100vh'
          }}
        >
          <Alert severity="error" sx={{ mb: 2, width: '100%', maxWidth: 600 }}>
            Something went wrong loading the application.
          </Alert>
          <Typography variant="body1">
            Please try refreshing the page. If the problem persists, contact support.
          </Typography>
          {/* Button is imported separately to avoid initialization issues */}
          <Box sx={{ mt: 2 }}>
            <a href="/" style={{ 
              backgroundColor: '#3f51b5', 
              color: 'white',
              padding: '8px 16px',
              borderRadius: '4px',
              textDecoration: 'none',
              display: 'inline-block'
            }}>
              Return to Dashboard
            </a>
          </Box>
        </Box>
      );
    }

    return this.props.children;
  }
}

// Create a theme
const theme = createTheme({
  palette: {
    mode: 'dark',
    primary: {
      main: '#3f51b5',
    },
    secondary: {
      main: '#f50057',
    },
    background: {
      default: '#121212',
      paper: '#1e1e1e',
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
  },
  components: {
    MuiAppBar: {
      defaultProps: {
        elevation: 0,
      },
      styleOverrides: {
        root: {
          backgroundColor: '#1e1e1e',
          borderBottom: '1px solid rgba(255, 255, 255, 0.12)',
        },
      },
    },
  },
});

// Sidebar width
const drawerWidth = 240;

function App() {
  // Simplified loading approach to avoid dependency issues
  const [isReady, setIsReady] = useState(false);

  // Use simple approach to ensure DOM is fully loaded
  useEffect(() => {
    const timer = setTimeout(() => setIsReady(true), 100);
    return () => clearTimeout(timer);
  }, []);

  // Simple loading component
  const Loading = () => (
    <Box 
      sx={{ 
        display: 'flex', 
        flexDirection: 'column',
        alignItems: 'center', 
        justifyContent: 'center',
        p: 3,
        height: '100vh'
      }}
    >
      <Typography variant="h4">Loading TradingLab...</Typography>
    </Box>
  );

  // Simplified app to avoid potential initialization issues
  return (
    <ThemeProvider theme={theme}>
      <CssBaseline />
      {!isReady ? <Loading /> : (
        <ErrorBoundary>
          <Router>
            <Box sx={{ display: 'flex' }}>
              <Navbar drawerWidth={drawerWidth} />
              <Sidebar drawerWidth={drawerWidth} />
              <Box
                component="main"
                sx={{
                  flexGrow: 1,
                  p: 3,
                  width: { sm: `calc(100% - ${drawerWidth}px)` },
                  ml: { sm: `${drawerWidth}px` },
                  mt: '64px',
                }}
              >
                <Routes>
                  <Route path="/" element={<Dashboard />} />
                  <Route path="/dashboard" element={<Dashboard />} />
                  <Route path="/historical" element={<Historical />} />
                  <Route path="/signals" element={<Signals />} />
                  <Route path="/backtest" element={<Backtest />} />
                  <Route path="/recommendations" element={<Recommendations />} />
                  <Route path="*" element={<Dashboard />} />
                </Routes>
              </Box>
            </Box>
          </Router>
        </ErrorBoundary>
      )}
    </ThemeProvider>
  );
}

export default App;