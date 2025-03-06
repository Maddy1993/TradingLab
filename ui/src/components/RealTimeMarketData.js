import React, { useState, useEffect } from 'react';
import { Typography, Paper, Box, Grid, Chip } from '@mui/material';
import TrendingUpIcon from '@mui/icons-material/TrendingUp';
import TrendingDownIcon from '@mui/icons-material/TrendingDown';
import { useMarketData } from '../services/websocketService';

const RealTimeMarketData = ({ ticker }) => {
  const marketData = useMarketData(ticker);
  const [priceChange, setPriceChange] = useState(0);
  const [lastPrice, setLastPrice] = useState(null);

  useEffect(() => {
    if (marketData && marketData.price) {
      // Calculate price change
      if (lastPrice !== null) {
        setPriceChange(marketData.price - lastPrice);
      }
      setLastPrice(marketData.price);
    }
  }, [marketData, lastPrice]);

  if (!marketData) {
    return (
        <Paper sx={{ p: 2, mb: 3 }}>
          <Typography variant="h6">Real-Time Market Data</Typography>
          <Typography variant="body2" color="text.secondary">
            Waiting for market data...
          </Typography>
        </Paper>
    );
  }

  // Format timestamp
  const timestamp = marketData.timestamp
      ? new Date(marketData.timestamp).toLocaleTimeString()
      : 'N/A';

  // Determine if price is up or down
  const priceIsUp = priceChange >= 0;
  const priceChangeAbs = Math.abs(priceChange);

  return (
      <Paper sx={{ p: 2, mb: 3 }}>
        <Typography variant="h6" gutterBottom>
          {ticker} Real-Time Market Data
        </Typography>

        <Grid container spacing={2}>
          <Grid item xs={6} md={3}>
            <Box>
              <Typography variant="body2" color="text.secondary">
                Price
              </Typography>
              <Typography variant="h4" sx={{ display: 'flex', alignItems: 'center' }}>
                ${marketData.price.toFixed(2)}
                {priceChange !== 0 && (
                    <Chip
                        size="small"
                        icon={priceIsUp ? <TrendingUpIcon /> : <TrendingDownIcon />}
                        label={`${priceIsUp ? '+' : '-'}$${priceChangeAbs.toFixed(2)}`}
                        color={priceIsUp ? "success" : "error"}
                        sx={{ ml: 1 }}
                    />
                )}
              </Typography>
            </Box>
          </Grid>

          <Grid item xs={6} md={3}>
            <Box>
              <Typography variant="body2" color="text.secondary">
                Last Update
              </Typography>
              <Typography variant="body1">
                {timestamp}
              </Typography>
            </Box>
          </Grid>

          <Grid item xs={6} md={3}>
            <Box>
              <Typography variant="body2" color="text.secondary">
                Volume
              </Typography>
              <Typography variant="body1">
                {marketData.volume ? marketData.volume.toLocaleString() : 'N/A'}
              </Typography>
            </Box>
          </Grid>

          <Grid item xs={6} md={3}>
            <Box>
              <Typography variant="body2" color="text.secondary">
                Source
              </Typography>
              <Typography variant="body1">
                {marketData.source || 'Real-time Feed'}
              </Typography>
            </Box>
          </Grid>
        </Grid>

        <Grid container spacing={2} sx={{ mt: 2 }}>
          <Grid item xs={3}>
            <Typography variant="body2" color="text.secondary">Open</Typography>
            <Typography variant="body1">${marketData.open?.toFixed(2) || 'N/A'}</Typography>
          </Grid>
          <Grid item xs={3}>
            <Typography variant="body2" color="text.secondary">High</Typography>
            <Typography variant="body1">${marketData.high?.toFixed(2) || 'N/A'}</Typography>
          </Grid>
          <Grid item xs={3}>
            <Typography variant="body2" color="text.secondary">Low</Typography>
            <Typography variant="body1">${marketData.low?.toFixed(2) || 'N/A'}</Typography>
          </Grid>
          <Grid item xs={3}>
            <Typography variant="body2" color="text.secondary">Close</Typography>
            <Typography variant="body1">${marketData.close?.toFixed(2) || 'N/A'}</Typography>
          </Grid>
        </Grid>
      </Paper>
  );
};

export default RealTimeMarketData;