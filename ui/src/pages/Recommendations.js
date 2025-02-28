import React, { useState, useEffect } from 'react';
import {
  Typography,
  Box,
  Paper,
  Grid,
  FormControl,
  InputLabel,
  Select,
  MenuItem,
  Button,
  Divider,
  Alert,
  TextField,
  Chip
} from '@mui/material';
import FilterListIcon from '@mui/icons-material/FilterList';
import CallMadeIcon from '@mui/icons-material/CallMade';
import CallReceivedIcon from '@mui/icons-material/CallReceived';
import axios from 'axios';
import { format } from 'date-fns';
import RecommendationCard from '../components/RecommendationCard';
import Loading from '../components/Loading';

const Recommendations = () => {
  const [loading, setLoading] = useState(false);
  const [ticker, setTicker] = useState(localStorage.getItem('selectedTicker') || 'SPY');
  const [days, setDays] = useState(30);
  const [strategy, setStrategy] = useState('RedCandle');
  const [recommendations, setRecommendations] = useState([]);
  const [filteredRecommendations, setFilteredRecommendations] = useState([]);
  const [error, setError] = useState(null);
  const [filter, setFilter] = useState('ALL');

  // Listen for ticker changes
  useEffect(() => {
    const handleTickerChange = (event) => {
      setTicker(event.detail);
    };

    window.addEventListener('tickerchange', handleTickerChange);
    return () => {
      window.removeEventListener('tickerchange', handleTickerChange);
    };
  }, []);

  // Fetch data when component mounts
  useEffect(() => {
    fetchRecommendations();
  }, [ticker]);

  // Apply filter when recommendations or filter changes
  useEffect(() => {
    applyFilter();
  }, [recommendations, filter]);

  const fetchRecommendations = async () => {
    setLoading(true);
    setError(null);

    try {
      const response = await axios.get(`/api/recommendations?ticker=${ticker}&days=${days}&strategy=${strategy}`);
      setRecommendations(response.data);
    } catch (error) {
      console.error('Error fetching recommendations:', error);
      setError('Failed to fetch recommendations. Please try again later.');
    } finally {
      setLoading(false);
    }
  };

  const applyFilter = () => {
    if (filter === 'ALL') {
      setFilteredRecommendations(recommendations);
    } else if (filter === 'LONG') {
      setFilteredRecommendations(recommendations.filter(rec => rec.signal_type === 'LONG'));
    } else if (filter === 'SHORT') {
      setFilteredRecommendations(recommendations.filter(rec => rec.signal_type === 'SHORT'));
    } else if (filter === 'CALL') {
      setFilteredRecommendations(recommendations.filter(rec => rec.option_type === 'CALL'));
    } else if (filter === 'PUT') {
      setFilteredRecommendations(recommendations.filter(rec => rec.option_type === 'PUT'));
    }
  };

  const handleDaysChange = (event) => {
    setDays(event.target.value);
  };

  const handleStrategyChange = (event) => {
    setStrategy(event.target.value);
  };

  const handleFilterChange = (event) => {
    setFilter(event.target.value);
  };

  // Calculate statistics
  const stats = {
    total: recommendations.length,
    long: recommendations.filter(rec => rec.signal_type === 'LONG').length,
    short: recommendations.filter(rec => rec.signal_type === 'SHORT').length,
    call: recommendations.filter(rec => rec.option_type === 'CALL').length,
    put: recommendations.filter(rec => rec.option_type === 'PUT').length
  };

  return (
      <Box>
        <Typography variant="h4" gutterBottom>
          Options Recommendations
        </Typography>

        <Paper sx={{ p: 3, mb: 3 }}>
          <Grid container spacing={2} alignItems="center">
            <Grid item xs={12} sm={6} md={3}>
              <Typography variant="h6" gutterBottom>
                {ticker} Recommendations
              </Typography>
            </Grid>
            <Grid item xs={12} sm={6} md={3}>
              <FormControl fullWidth>
                <InputLabel>Days</InputLabel>
                <Select
                    value={days}
                    label="Days"
                    onChange={handleDaysChange}
                >
                  <MenuItem value={10}>10 days</MenuItem>
                  <MenuItem value={30}>30 days</MenuItem>
                  <MenuItem value={60}>60 days</MenuItem>
                  <MenuItem value={90}>90 days</MenuItem>
                </Select>
              </FormControl>
            </Grid>
            <Grid item xs={12} sm={6} md={3}>
              <FormControl fullWidth>
                <InputLabel>Strategy</InputLabel>
                <Select
                    value={strategy}
                    label="Strategy"
                    onChange={handleStrategyChange}
                >
                  <MenuItem value="RedCandle">Red Candle Theory</MenuItem>
                </Select>
              </FormControl>
            </Grid>
            <Grid item xs={12} sm={6} md={3}>
              <Button
                  variant="contained"
                  fullWidth
                  onClick={fetchRecommendations}
              >
                Generate
              </Button>
            </Grid>
          </Grid>
        </Paper>

        {error && (
            <Alert severity="error" sx={{ mb: 3 }}>
              {error}
            </Alert>
        )}

        {loading ? (
            <Loading message={`Generating recommendations for ${ticker}...`} />
        ) : (
            <>
              {recommendations.length > 0 ? (
                  <>
                    <Paper sx={{ p: 3, mb: 3 }}>
                      <Grid container spacing={2} alignItems="center">
                        <Grid item xs={12} md={8}>
                          <Box sx={{ display: 'flex', gap: 2, flexWrap: 'wrap' }}>
                            <Box>
                              <Typography variant="body2" color="text.secondary">
                                Total Recommendations:
                              </Typography>
                              <Typography variant="h6">
                                {stats.total}
                              </Typography>
                            </Box>
                            <Divider orientation="vertical" flexItem />
                            <Box sx={{ display: 'flex', gap: 2 }}>
                              <Box>
                                <Typography variant="body2" color="text.secondary">
                                  Long Signals:
                                </Typography>
                                <Box sx={{ display: 'flex', alignItems: 'center' }}>
                                  <CallMadeIcon color="success" sx={{ mr: 0.5 }} fontSize="small" />
                                  <Typography variant="body1">
                                    {stats.long}
                                  </Typography>
                                </Box>
                              </Box>
                              <Box>
                                <Typography variant="body2" color="text.secondary">
                                  Short Signals:
                                </Typography>
                                <Box sx={{ display: 'flex', alignItems: 'center' }}>
                                  <CallReceivedIcon color="error" sx={{ mr: 0.5 }} fontSize="small" />
                                  <Typography variant="body1">
                                    {stats.short}
                                  </Typography>
                                </Box>
                              </Box>
                            </Box>
                            <Divider orientation="vertical" flexItem />
                            <Box sx={{ display: 'flex', gap: 2 }}>
                              <Box>
                                <Typography variant="body2" color="text.secondary">
                                  CALL Options:
                                </Typography>
                                <Typography variant="body1">
                                  {stats.call}
                                </Typography>
                              </Box>
                              <Box>
                                <Typography variant="body2" color="text.secondary">
                                  PUT Options:
                                </Typography>
                                <Typography variant="body1">
                                  {stats.put}
                                </Typography>
                              </Box>
                            </Box>
                          </Box>
                        </Grid>
                        <Grid item xs={12} md={4}>
                          <FormControl fullWidth>
                            <InputLabel>Filter</InputLabel>
                            <Select
                                value={filter}
                                label="Filter"
                                onChange={handleFilterChange}
                                startAdornment={<FilterListIcon sx={{ mr: 1 }} />}
                            >
                              <MenuItem value="ALL">All Recommendations</MenuItem>
                              <MenuItem value="LONG">Long Signals Only</MenuItem>
                              <MenuItem value="SHORT">Short Signals Only</MenuItem>
                              <MenuItem value="CALL">CALL Options Only</MenuItem>
                              <MenuItem value="PUT">PUT Options Only</MenuItem>
                            </Select>
                          </FormControl>
                        </Grid>
                      </Grid>
                    </Paper>

                    {filteredRecommendations.length > 0 ? (
                        filteredRecommendations.map((recommendation, index) => (
                            <RecommendationCard key={index} recommendation={recommendation} />
                        ))
                    ) : (
                        <Alert severity="info">
                          No recommendations match the selected filter.
                        </Alert>
                    )}
                  </>
              ) : (
                  <Alert severity="info">
                    No recommendations available for the selected ticker and time period.
                  </Alert>
              )}
            </>
        )}
      </Box>
  );
};

export default Recommendations;