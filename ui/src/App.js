import React, { Suspense, lazy, useState, useEffect } from 'react';
import { BrowserRouter as Router, Route, Routes } from 'react-router-dom';
import { ThemeProvider, createTheme } from '@mui/material/styles';
import CssBaseline from '@mui/material/CssBaseline';
import Box from '@mui/material/Box';
import { CircularProgress, Alert, Typography, Button } from '@mui/material';

// Components
import Navbar from './components/Navbar';
import Sidebar from './components/Sidebar';
import SystemStatusBanner from './components/SystemStatusBanner';

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
    warning: {
      main: '#ff9800',
    },
    error: {
      main: '#f44336',
    },
    success: {
      main: '#4caf50',
    },
    info: {
      main: '#2196f3',
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
    MuiAlert: {
      styleOverrides: {
        root: {
          borderRadius: 4,
        },
        standardWarning: {
          backgroundColor: 'rgba(255, 152, 0, 0.1)',
          color: '#ff9800',
        },
        standardError: {
          backgroundColor: 'rgba(244, 67, 54, 0.1)',
          color: '#f44336',
        },
        standardInfo: {
          backgroundColor: 'rgba(33, 150, 243, 0.1)',
          color: '#2196f3',
        },
        standardSuccess: {
          backgroundColor: 'rgba(76, 175, 80, 0.1)',
          color: '#4caf50',
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
              {/* Add error boundaries around individual components */}
              <ErrorBoundary>
                <Navbar drawerWidth={drawerWidth} />
              </ErrorBoundary>
              <ErrorBoundary>
                <Sidebar drawerWidth={drawerWidth} />
              </ErrorBoundary>
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
                {/* System status banner at the top of the content area */}
                <ErrorBoundary>
                  <SystemStatusBanner />
                </ErrorBoundary>
                
                <Routes>
                  <Route path="/" element={
                    <ErrorBoundary>
                      <Dashboard />
                    </ErrorBoundary>
                  } />
                  <Route path="/dashboard" element={
                    <ErrorBoundary>
                      <Dashboard />
                    </ErrorBoundary>
                  } />
                  <Route path="/historical" element={
                    <ErrorBoundary>
                      <Suspense fallback={<Loading />}>
                        <Historical />
                      </Suspense>
                    </ErrorBoundary>
                  } />
                  <Route path="/signals" element={
                    <ErrorBoundary>
                      <Suspense fallback={<Loading />}>
                        <Signals />
                      </Suspense>
                    </ErrorBoundary>
                  } />
                  <Route path="/backtest" element={
                    <ErrorBoundary>
                      <Suspense fallback={<Loading />}>
                        <Backtest />
                      </Suspense>
                    </ErrorBoundary>
                  } />
                  <Route path="/recommendations" element={
                    <ErrorBoundary>
                      <Suspense fallback={<Loading />}>
                        <Recommendations />
                      </Suspense>
                    </ErrorBoundary>
                  } />
                  <Route path="*" element={
                    <ErrorBoundary>
                      <Dashboard />
                    </ErrorBoundary>
                  } />
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