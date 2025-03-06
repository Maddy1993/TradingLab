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
          <Button 
            variant="contained" 
            color="primary" 
            sx={{ mt: 2 }}
            onClick={() => {
              this.setState({ hasError: false });
              window.location.href = '/';
            }}
          >
            Return to Dashboard
          </Button>
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
  const [isLoading, setIsLoading] = useState(true);

  // Add effect to ensure UI is ready
  useEffect(() => {
    // Short timeout to ensure the DOM is fully loaded
    const timer = setTimeout(() => {
      setIsLoading(false);
    }, 500);
    
    return () => clearTimeout(timer);
  }, []);

  const LoadingFallback = () => (
    <Box sx={{ 
      display: 'flex',
      flexDirection: 'column',
      justifyContent: 'center',
      alignItems: 'center',
      height: '100vh'
    }}>
      <CircularProgress size={60} />
      <Typography variant="h6" sx={{ mt: 2 }}>
        Loading TradingLab...
      </Typography>
    </Box>
  );

  if (isLoading) {
    return (
      <ThemeProvider theme={theme}>
        <CssBaseline />
        <LoadingFallback />
      </ThemeProvider>
    );
  }

  return (
    <ThemeProvider theme={theme}>
      <CssBaseline />
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
              <Suspense fallback={<LoadingFallback />}>
                <Routes>
                  <Route path="/" element={<Dashboard />} />
                  <Route path="/dashboard" element={<Dashboard />} />
                  <Route path="/historical" element={<Historical />} />
                  <Route path="/signals" element={<Signals />} />
                  <Route path="/backtest" element={<Backtest />} />
                  <Route path="/recommendations" element={<Recommendations />} />
                  {/* Add a catch-all route that redirects to Dashboard */}
                  <Route path="*" element={<Dashboard />} />
                </Routes>
              </Suspense>
            </Box>
          </Box>
        </Router>
      </ErrorBoundary>
    </ThemeProvider>
  );
}

export default App;