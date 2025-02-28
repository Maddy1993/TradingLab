import React, { useState, useEffect } from 'react';
import {
  Typography,
  Grid,
  Paper,
  Box,
  Card,
  CardContent,
  Divider,
  Button,
  CircularProgress
} from '@mui/material';
import axios from 'axios';
import { useNavigate } from 'react-router-dom';
import CandlestickChart from '../components/CandlestickChart';
import SignalSummary from '../components/SignalSummary';
import BacktestCard from '../components/BacktestCard';
import RecommendationCard from '../components/RecommendationCard';
import Loading from '../components/Loading';

const Dashboard = () => {
  const navigate = useNavigate();
  const [loading, setLoading] = useState(true);
  const [ticker, setTicker] = useState(localStorage.getItem('selectedTicker') || 'SPY');
  const [historicalData, setHistoricalData] = useState([]);
  const [signals, setSignals] = useState([]);
  const [backtestResults, setBacktestResults] = useState({});
  const [recommendations, setRecommendations] = useState([]);
  const [error, setError] = useState(null);

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

  // Fetch all data needed for dashboard
  useEffect(() => {
    const fetchDashboardData = async () => {
      setLoading(true);
      setError(null);

      try {
        // Fetch data in parallel
        const [historicalResponse, signalsResponse, backtestResponse, recommendationsResponse] = await Promise.all([
          axios.get(`/api/historical-data?ticker=${ticker}&days=30`),
          axios.get(`/api/signals?ticker=${ticker}&days=30&strategy=RedCandle`),
          axios.get(`/api/backtest?ticker=${ticker}&days=30&strategy=RedCandle`),
          axios.get(`/api/recommendations?ticker=${ticker}&days=30&strategy=RedCandle`)
        ]);

        setHistoricalData(historicalResponse.data);
        setSignals(signalsResponse.data);
        setBacktestResults(backtestResponse.data);
        setRecommendations(recommendationsResponse.data);
      } catch (error) {
        console.error('Error fetching dashboard data:', error);
        setError('Failed to load dashboard data. Please try again later.');
      } finally {
        setLoading(false);
      }
    };

    fetchDashboardData();
  }, [ticker]);

  if (loading) {
    return <Loading message={`Loading dashboard for ${ticker}...`} />;
  }

  if (error) {
    return (
        <Box sx={{ p: 3 }}>
          <Typography color="error" variant="h6">
            {error}
          </Typography>
          <Button
              variant="contained"
              sx={{ mt: 2 }}
              onClick={() => window.location.reload()}
          >
            Retry
          </Button>
        </Box>
    );
  }

  // Get sample items for preview
  const bestBacktest = backtestResults && Object.entries(backtestResults).length > 0
      ? Object.entries(backtestResults).reduce((best, [key, current]) => {
        return !best || (current.profit_factor > best[1].profit_factor) ? [key, current] : best;
      }, null)
      : null;

  const latestRecommendation = recommendations && recommendations.length > 0
      ? recommendations[recommendations.length - 1]
      : null;

  return (
      <Box>
        <Typography variant="h4" gutterBottom>
          {ticker} Analysis Dashboard
        </Typography>

        <Typography variant="body1" color="text.secondary" paragraph>
          Real-time trading analysis and recommendations for {ticker} using the Red Candle strategy.
        </Typography>

        <Grid container spacing={3}>
          {/* Price Chart */}
          <Grid item xs={12}>
            <Paper sx={{ p: 2 }}>
              <Typography variant="h6" gutterBottom>
                {ticker} Price Chart
              </Typography>
              <Divider sx={{ mb: 2 }} />
              <CandlestickChart data={historicalData} signals={signals} />
            </Paper>
          </Grid>

          {/* Signal Summary */}
          <Grid item xs={12}>
            <SignalSummary signals={signals} loading={false} />
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
                  View All
                </Button>
              </Box>
              <Divider sx={{ mb: 2 }} />

              {bestBacktest ? (
                  <BacktestCard
                      title={bestBacktest[0]}
                      result={bestBacktest[1]}
                      onClick={() => navigate('/backtest')}
                  />
              ) : (
                  <Typography variant="body2" color="text.secondary" align="center">
                    No backtest results available
                  </Typography>
              )}
            </Paper>
          </Grid>

          {/* Recommendation Preview */}
          <Grid item xs={12} md={6}>
            <Paper sx={{ p: 2, height: '100%' }}>
              <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', mb: 2 }}>
                <Typography variant="h6">Latest Recommendation</Typography>
                <Button
                    variant="outlined"
                    size="small"
                    onClick={() => navigate('/recommendations')}
                >
                  View All
                </Button>
              </Box>
              <Divider sx={{ mb: 2 }} />

              {latestRecommendation ? (
                  <RecommendationCard recommendation={latestRecommendation} />
              ) : (
                  <Typography variant="body2" color="text.secondary" align="center">
                    No recommendations available
                  </Typography>
              )}
            </Paper>
          </Grid>
        </Grid>
      </Box>
  );
};

export default Dashboard;