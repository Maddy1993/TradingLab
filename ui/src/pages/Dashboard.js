import React, { useState, useEffect, useCallback, Suspense } from 'react';
import {
  Typography,
  Grid,
  Paper,
  Box,
  Divider,
  Button,
  Alert,
  CircularProgress
} from '@mui/material';
import axios from 'axios';
import { useNavigate } from 'react-router-dom';
import TradingViewChart from '../components/TradingViewChart';
import SignalSummary from '../components/SignalSummary';
import Loading from '../components/Loading';
import RealTimeMarketData from '../components/RealTimeMarketData';
import RealTimeSignals from '../components/RealTimeSignals';

// Add global error handler for better error handling
axios.interceptors.response.use(
  response => response,
  error => {
    console.error('API Error:', error);
    if (error.response && error.response.status === 404) {
      // Handle not found errors
      console.warn('Resource not found:', error.config.url);
    }
    return Promise.reject(error);
  }
);

const Dashboard = () => {
  const navigate = useNavigate();
  const [ticker, setTicker] = useState(localStorage.getItem('selectedTicker') || 'SPY');

  // State for each data section
  const [timeRange, setTimeRange] = useState('15M');
  const [historicalData, setHistoricalData] = useState([]);
  const [historicalLoading, setHistoricalLoading] = useState(true);
  const [historicalError, setHistoricalError] = useState(null);

  const [signals, setSignals] = useState([]);
  const [signalsLoading, setSignalsLoading] = useState(true);
  const [signalsError, setSignalsError] = useState(null);

  // Listen for ticker changes from Navbar
  useEffect(() => {
    const handleTickerChange = (event) => {
      setTicker(event.detail);
    };

    window.addEventListener('tickerchange', handleTickerChange);
    return () => {
      window.removeEventListener('tickerchange', handleTickerChange);
    };
  }, []);

  // Fetch essential data (historical data and signals) for dashboard
  useEffect(() => {
    fetchEssentialData(timeRange);
  }, [ticker, timeRange, fetchEssentialData]);

  const fetchEssentialData = useCallback(async (range = timeRange) => {
    setHistoricalLoading(true);
    setSignalsLoading(true);
    setHistoricalError(null);
    setSignalsError(null);

    try {
      // Fetch historical data
      const historicalResponse = await axios.get(`/api/historical-data?ticker=${ticker}&days=30&interval=${range}`);
      setHistoricalData(historicalResponse.data);
    } catch (error) {
      console.error('Error fetching historical data:', error);
      setHistoricalError('Failed to load historical data.');
    } finally {
      setHistoricalLoading(false);
    }

    try {
      // Fetch signals data
      const signalsResponse = await axios.get(`/api/signals?ticker=${ticker}&days=30&strategy=RedCandle&interval=${range}`);
      setSignals(signalsResponse.data);
    } catch (error) {
      console.error('Error fetching signals data:', error);
      setSignalsError('Failed to load signals data.');
    } finally {
      setSignalsLoading(false);
    }
  }, [ticker, timeRange]);

  const handleRangeChange = (newRange) => {
    setTimeRange(newRange);
    fetchEssentialData(newRange);
  };

  return (
      <Box>
        <Typography variant="h4" gutterBottom>
          {ticker} Analysis Dashboard
        </Typography>

        <Typography variant="body1" color="text.secondary" paragraph>
          Real-time trading analysis and recommendations using the Red Candle strategy.
        </Typography>

        <Grid container spacing={3}>
          {/* Real-Time Market Data */}
          <Grid item xs={12}>
            <RealTimeMarketData ticker={ticker} />
          </Grid>

          {/* Real-Time Signals */}
          <Grid item xs={12}>
            <RealTimeSignals ticker={ticker} />
          </Grid>

          {/* Price Chart */}
          <Grid item xs={12}>
            {historicalLoading ? (
                <Loading message={`Loading price data for ${ticker}...`} />
            ) : historicalError ? (
                <Alert severity="error">{historicalError}</Alert>
            ) : !historicalData || historicalData.length === 0 ? (
                <Alert severity="info">No historical data available for {ticker}.</Alert>
            ) : (
                <Paper sx={{ p: 2 }}>
                  <Typography variant="h6" gutterBottom>
                    {ticker} Historical Price Chart
                  </Typography>
                  <Divider sx={{ mb: 2 }} />
                  <TradingViewChart
                      data={historicalData}
                      signals={signals}
                      initialRange={timeRange}
                      onRangeChange={handleRangeChange}
                  />
                </Paper>
            )}
          </Grid>

          {/* Signal Summary */}
          <Grid item xs={12}>
            {signalsLoading ? (
                <Loading message={`Loading signals for ${ticker}...`} />
            ) : signalsError ? (
                <Alert severity="error">{signalsError}</Alert>
            ) : (
                <SignalSummary signals={signals} loading={false} />
            )}
          </Grid>

          {/* Backtest Preview */}
          <Grid item xs={12} md={6}>
            <Paper sx={{ p: 2, height: '100%' }}>
              <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', mb: 2 }}>
                <Typography variant="h6">Backtest Results</Typography>
                <Button
                    variant="outlined"
                    size="small"
                    onClick={() => navigate('/backtest')}
                >
                  Run Backtests
                </Button>
              </Box>
              <Divider sx={{ mb: 2 }} />
              <Box sx={{
                display: 'flex',
                justifyContent: 'center',
                alignItems: 'center',
                height: '200px',
                backgroundColor: 'rgba(0,0,0,0.05)',
                borderRadius: 1
              }}>
                <Typography variant="body1" color="text.secondary">
                  Click "Run Backtests" to analyze {ticker} performance
                </Typography>
              </Box>
            </Paper>
          </Grid>

          {/* Recommendation Preview */}
          <Grid item xs={12} md={6}>
            <Paper sx={{ p: 2, height: '100%' }}>
              <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', mb: 2 }}>
                <Typography variant="h6">Options Recommendations</Typography>
                <Button
                    variant="outlined"
                    size="small"
                    onClick={() => navigate('/recommendations')}
                >
                  Get Recommendations
                </Button>
              </Box>
              <Divider sx={{ mb: 2 }} />
              <Box sx={{
                display: 'flex',
                justifyContent: 'center',
                alignItems: 'center',
                height: '200px',
                backgroundColor: 'rgba(0,0,0,0.05)',
                borderRadius: 1
              }}>
                <Typography variant="body1" color="text.secondary">
                  Click "Get Recommendations" to see options strategies for {ticker}
                </Typography>
              </Box>
            </Paper>
          </Grid>
        </Grid>
      </Box>
  );
};

export default Dashboard;