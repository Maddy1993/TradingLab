import React, { useState, useEffect } from 'react';
import {
  Typography,
  Box,
  Paper,
  Grid,
  TextField,
  Button,
  Divider,
  FormControl,
  InputLabel,
  Select,
  MenuItem,
  Alert,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  TablePagination
} from '@mui/material';
import axios from 'axios';
import { format } from 'date-fns';
import TradingViewChart from '../components/TradingViewChart';
import Loading from '../components/Loading';

const Historical = () => {
  const [loading, setLoading] = useState(false);
  const [ticker, setTicker] = useState(localStorage.getItem('selectedTicker') || 'SPY');
  const [days, setDays] = useState(30);
  const [historicalData, setHistoricalData] = useState([]);
  const [error, setError] = useState(null);
  const [page, setPage] = useState(0);
  const [rowsPerPage, setRowsPerPage] = useState(10);

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

  // Fetch data when ticker or days change
  useEffect(() => {
    fetchHistoricalData();
  }, [ticker]);

  const fetchHistoricalData = async () => {
    setLoading(true);
    setError(null);

    try {
      const response = await axios.get(`/api/historical-data?ticker=${ticker}&days=${days}`);
      setHistoricalData(response.data);
    } catch (error) {
      console.error('Error fetching historical data:', error);
      setError('Failed to load historical data. Please try again later.');
    } finally {
      setLoading(false);
    }
  };

  const handleDaysChange = (event) => {
    setDays(event.target.value);
  };

  const handleChangePage = (event, newPage) => {
    setPage(newPage);
  };

  const handleChangeRowsPerPage = (event) => {
    setRowsPerPage(parseInt(event.target.value, 10));
    setPage(0);
  };

  const formattedData = historicalData.map(candle => {
    // Format date
    let formattedDate;
    try {
      const dateObj = new Date(candle.date);
      formattedDate = format(dateObj, 'yyyy-MM-dd HH:mm:ss');
    } catch (error) {
      formattedDate = candle.date;
    }

    return {
      ...candle,
      formattedDate
    };
  });

  // Calculate stats
  const stats = React.useMemo(() => {
    if (!historicalData || historicalData.length === 0) {
      return {
        high: 0,
        low: 0,
        avgVolume: 0,
        avgRange: 0
      };
    }

    const high = Math.max(...historicalData.map(c => c.high));
    const low = Math.min(...historicalData.map(c => c.low));
    const avgVolume = historicalData.reduce((sum, c) => sum + c.volume, 0) / historicalData.length;
    const avgRange = historicalData.reduce((sum, c) => sum + (c.high - c.low), 0) / historicalData.length;

    return {
      high: high.toFixed(2),
      low: low.toFixed(2),
      avgVolume: Math.round(avgVolume).toLocaleString(),
      avgRange: avgRange.toFixed(2)
    };
  }, [historicalData]);

  return (
      <Box>
        <Typography variant="h4" gutterBottom>
          Historical Data
        </Typography>

        <Paper sx={{ p: 3, mb: 3 }}>
          <Grid container spacing={2} alignItems="center">
            <Grid item xs={12} sm={6} md={4}>
              <Typography variant="h6" gutterBottom>
                {ticker} Historical Prices
              </Typography>
            </Grid>
            <Grid item xs={12} sm={6} md={4}>
              <FormControl fullWidth>
                <InputLabel>Days</InputLabel>
                <Select
                    value={days}
                    label="Days"
                    onChange={handleDaysChange}
                >
                  <MenuItem value={5}>5 days</MenuItem>
                  <MenuItem value={10}>10 days</MenuItem>
                  <MenuItem value={30}>30 days</MenuItem>
                  <MenuItem value={60}>60 days</MenuItem>
                  <MenuItem value={90}>90 days</MenuItem>
                </Select>
              </FormControl>
            </Grid>
            <Grid item xs={12} sm={12} md={4}>
              <Button
                  variant="contained"
                  fullWidth
                  onClick={fetchHistoricalData}
              >
                Fetch Data
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
            <Loading message={`Loading historical data for ${ticker}...`} />
        ) : (
            <>
              {historicalData.length > 0 && (
                  <>
                    <Grid container spacing={3} sx={{ mb: 3 }}>
                      <Grid item xs={6} sm={3}>
                        <Paper sx={{ p: 2, textAlign: 'center' }}>
                          <Typography variant="body2" color="text.secondary">Highest Price</Typography>
                          <Typography variant="h6">${stats.high}</Typography>
                        </Paper>
                      </Grid>
                      <Grid item xs={6} sm={3}>
                        <Paper sx={{ p: 2, textAlign: 'center' }}>
                          <Typography variant="body2" color="text.secondary">Lowest Price</Typography>
                          <Typography variant="h6">${stats.low}</Typography>
                        </Paper>
                      </Grid>
                      <Grid item xs={6} sm={3}>
                        <Paper sx={{ p: 2, textAlign: 'center' }}>
                          <Typography variant="body2" color="text.secondary">Avg. Volume</Typography>
                          <Typography variant="h6">{stats.avgVolume}</Typography>
                        </Paper>
                      </Grid>
                      <Grid item xs={6} sm={3}>
                        <Paper sx={{ p: 2, textAlign: 'center' }}>
                          <Typography variant="body2" color="text.secondary">Avg. Range</Typography>
                          <Typography variant="h6">${stats.avgRange}</Typography>
                        </Paper>
                      </Grid>
                    </Grid>

                    <Paper sx={{ mb: 3 }}>
                      <TradingViewChart data={historicalData} />
                    </Paper>

                    <Paper>
                      <TableContainer>
                        <Table>
                          <TableHead>
                            <TableRow>
                              <TableCell>Date & Time</TableCell>
                              <TableCell align="right">Open</TableCell>
                              <TableCell align="right">High</TableCell>
                              <TableCell align="right">Low</TableCell>
                              <TableCell align="right">Close</TableCell>
                              <TableCell align="right">Volume</TableCell>
                            </TableRow>
                          </TableHead>
                          <TableBody>
                            {formattedData
                            .slice(page * rowsPerPage, page * rowsPerPage + rowsPerPage)
                            .map((row, index) => (
                                <TableRow key={index}>
                                  <TableCell>{row.formattedDate}</TableCell>
                                  <TableCell align="right">${row.open.toFixed(2)}</TableCell>
                                  <TableCell align="right">${row.high.toFixed(2)}</TableCell>
                                  <TableCell align="right">${row.low.toFixed(2)}</TableCell>
                                  <TableCell align="right">${row.close.toFixed(2)}</TableCell>
                                  <TableCell align="right">{row.volume.toLocaleString()}</TableCell>
                                </TableRow>
                            ))}
                          </TableBody>
                        </Table>
                        <TablePagination
                            rowsPerPageOptions={[10, 25, 50, 100]}
                            component="div"
                            count={formattedData.length}
                            rowsPerPage={rowsPerPage}
                            page={page}
                            onPageChange={handleChangePage}
                            onRowsPerPageChange={handleChangeRowsPerPage}
                        />
                      </TableContainer>
                    </Paper>
                  </>
              )}

              {historicalData.length === 0 && !loading && (
                  <Alert severity="info">
                    No historical data available for the selected ticker and time period.
                  </Alert>
              )}
            </>
        )}
      </Box>
  );
};

export default Historical;