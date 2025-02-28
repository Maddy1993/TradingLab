import React, { useEffect, useState } from 'react';
import {
  Paper,
  Typography,
  Box,
  Grid,
  Divider,
  Chip,
  CircularProgress
} from '@mui/material';
import SignalCellularAltIcon from '@mui/icons-material/SignalCellularAlt';
import TrendingUpIcon from '@mui/icons-material/TrendingUp';
import TrendingDownIcon from '@mui/icons-material/TrendingDown';
import { format } from 'date-fns';

const SignalSummary = ({ signals, loading }) => {
  const [stats, setStats] = useState({
    total: 0,
    long: 0,
    short: 0,
    avgStopDistance: 0,
    latestSignal: null,
    latestSignalType: '',
    latestSignalDate: '',
    latestSignalPrice: 0,
  });

  useEffect(() => {
    if (signals && signals.length > 0) {
      // Calculate statistics
      const longSignals = signals.filter(s => s.signal_type === 'LONG');
      const shortSignals = signals.filter(s => s.signal_type === 'SHORT');

      // Calculate average stop distance (as percentage)
      const stopDistances = signals.map(s =>
          Math.abs((s.stoploss - s.entry_price) / s.entry_price * 100)
      );
      const avgStopDistance = stopDistances.reduce((sum, val) => sum + val, 0) / signals.length;

      // Get the latest signal
      const latestSignal = signals[signals.length - 1];
      let formattedDate = '';

      try {
        const dateObj = new Date(latestSignal.date);
        formattedDate = format(dateObj, 'MMM dd, yyyy HH:mm');
      } catch (error) {
        formattedDate = latestSignal.date;
      }

      setStats({
        total: signals.length,
        long: longSignals.length,
        short: shortSignals.length,
        avgStopDistance,
        latestSignal,
        latestSignalType: latestSignal.signal_type,
        latestSignalDate: formattedDate,
        latestSignalPrice: latestSignal.entry_price,
      });
    }
  }, [signals]);

  if (loading) {
    return (
        <Box sx={{ display: 'flex', justifyContent: 'center', p: 3 }}>
          <CircularProgress />
        </Box>
    );
  }

  if (!signals || signals.length === 0) {
    return (
        <Paper sx={{ p: 3, mb: 3 }}>
          <Typography variant="subtitle1" align="center">
            No trading signals available for the selected period.
          </Typography>
        </Paper>
    );
  }

  return (
      <Paper sx={{ p: 3, mb: 3 }}>
        <Box sx={{ display: 'flex', alignItems: 'center', mb: 2 }}>
          <SignalCellularAltIcon color="primary" sx={{ mr: 1 }} />
          <Typography variant="h6">Signal Summary</Typography>
        </Box>

        <Divider sx={{ mb: 3 }} />

        <Grid container spacing={3}>
          <Grid item xs={12} md={6}>
            <Box>
              <Typography variant="subtitle2" color="text.secondary">
                Total Signals
              </Typography>
              <Typography variant="h4">
                {stats.total}
              </Typography>
            </Box>

            <Box sx={{ display: 'flex', mt: 2, gap: 2 }}>
              <Box>
                <Typography variant="subtitle2" color="text.secondary">
                  Long Signals
                </Typography>
                <Box sx={{ display: 'flex', alignItems: 'center' }}>
                  <TrendingUpIcon color="success" sx={{ mr: 0.5 }} />
                  <Typography variant="h6" color="success.main">
                    {stats.long}
                  </Typography>
                </Box>
              </Box>

              <Box>
                <Typography variant="subtitle2" color="text.secondary">
                  Short Signals
                </Typography>
                <Box sx={{ display: 'flex', alignItems: 'center' }}>
                  <TrendingDownIcon color="error" sx={{ mr: 0.5 }} />
                  <Typography variant="h6" color="error.main">
                    {stats.short}
                  </Typography>
                </Box>
              </Box>
            </Box>

            <Box sx={{ mt: 2 }}>
              <Typography variant="subtitle2" color="text.secondary">
                Average Stop Distance
              </Typography>
              <Typography variant="h6">
                {stats.avgStopDistance.toFixed(2)}%
              </Typography>
            </Box>
          </Grid>

          <Grid item xs={12} md={6}>
            <Typography variant="subtitle2" color="text.secondary">
              Latest Signal
            </Typography>

            <Box sx={{ mt: 1, p: 2, bgcolor: 'background.paper', borderRadius: 1 }}>
              <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', mb: 1 }}>
                <Typography variant="body2" color="text.secondary">
                  {stats.latestSignalDate}
                </Typography>

                <Chip
                    label={stats.latestSignalType}
                    color={stats.latestSignalType === 'LONG' ? 'success' : 'error'}
                    size="small"
                    icon={stats.latestSignalType === 'LONG' ? <TrendingUpIcon /> : <TrendingDownIcon />}
                />
              </Box>

              <Typography variant="h6">
                Entry Price: ${stats.latestSignalPrice?.toFixed(2)}
              </Typography>

              {stats.latestSignal && (
                  <Typography variant="body1">
                    Stop Loss: ${stats.latestSignal.stoploss.toFixed(2)}
                  </Typography>
              )}

              {stats.latestSignal && (
                  <Typography variant="body2" color="text.secondary" sx={{ mt: 1 }}>
                    Risk: {Math.abs((stats.latestSignal.stoploss - stats.latestSignal.entry_price) / stats.latestSignal.entry_price * 100).toFixed(2)}%
                  </Typography>
              )}
            </Box>
          </Grid>
        </Grid>
      </Paper>
  );
};

export default SignalSummary;