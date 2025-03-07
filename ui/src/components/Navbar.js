import React, { useState, useEffect } from 'react';
import { AppBar, Toolbar, Typography, IconButton, Box, MenuItem, FormControl, Select, Divider } from '@mui/material';
import MenuIcon from '@mui/icons-material/Menu';
import axios from 'axios';
import StatusIndicator from './StatusIndicator';
import { useSystemStatus, STATUS_LEVELS } from '../hooks/useSystemStatus';

const Navbar = ({ drawerWidth }) => {
  const [mobileOpen, setMobileOpen] = useState(false);
  const [selectedTicker, setSelectedTicker] = useState('SPY');
  const [tickers, setTickers] = useState(['SPY', 'AAPL', 'MSFT', 'GOOGL', 'AMZN']);
  const { status, isTradingDisabled } = useSystemStatus();

  useEffect(() => {
    const fetchTickers = async () => {
      try {
        const response = await axios.get('/api/tickers');
        if (response.data && Array.isArray(response.data)) {
          setTickers(response.data);
        }
      } catch (error) {
        console.error('Error fetching tickers:', error);
      }
    };

    fetchTickers();
  }, []);

  useEffect(() => {
    // Load selected ticker from localStorage on component mount
    const savedTicker = localStorage.getItem('selectedTicker');
    if (savedTicker) {
      setSelectedTicker(savedTicker);
    }
  }, []);

  const handleDrawerToggle = () => {
    setMobileOpen(!mobileOpen);
  };

  const handleTickerChange = (event) => {
    const newTicker = event.target.value;
    setSelectedTicker(newTicker);
    // Store in localStorage to make it available to other components
    localStorage.setItem('selectedTicker', newTicker);
    // Trigger an event to notify other components of the ticker change
    window.dispatchEvent(new CustomEvent('tickerchange', { detail: newTicker }));
  };

  return (
      <AppBar
          position="fixed"
          sx={{
            width: { sm: `calc(100% - ${drawerWidth}px)` },
            ml: { sm: `${drawerWidth}px` },
          }}
      >
        <Toolbar>
          <IconButton
              color="inherit"
              aria-label="open drawer"
              edge="start"
              onClick={handleDrawerToggle}
              sx={{ mr: 2, display: { sm: 'none' } }}
          >
            <MenuIcon />
          </IconButton>
          <Typography variant="h6" noWrap component="div" sx={{ flexGrow: 1 }}>
            TradingLab Dashboard
          </Typography>

          {/* Status indicator */}
          <StatusIndicator />
          
          <Divider orientation="vertical" flexItem sx={{ mx: 2, display: { xs: 'none', sm: 'block' } }} />

          <Box sx={{ display: 'flex', alignItems: 'center' }}>
            <Typography variant="body1" sx={{ mr: 2 }}>
              Ticker:
            </Typography>
            <FormControl size="small" sx={{ minWidth: 120 }}>
              <Select
                  value={selectedTicker}
                  onChange={handleTickerChange}
                  displayEmpty
                  inputProps={{ 'aria-label': 'Select Ticker' }}
                  disabled={isTradingDisabled()}
              >
                {tickers.map((ticker) => (
                    <MenuItem key={ticker} value={ticker}>
                      {ticker}
                    </MenuItem>
                ))}
              </Select>
            </FormControl>
          </Box>
        </Toolbar>
      </AppBar>
  );
};

export default Navbar;