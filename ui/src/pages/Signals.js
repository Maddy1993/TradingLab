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
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  TablePagination,
  Chip
} from '@mui/material';
import TrendingUpIcon from '@mui/icons-material/TrendingUp';
import TrendingDownIcon from '@mui/icons-material/TrendingDown';
import axios from 'axios';
import { format } from 'date-fns';
import CandlestickChart from '../components/CandlestickChart';
import SignalSummary from '../components/SignalSummary';
import Loading from '../components/Loading';

const Signals = () => {
  const [loading, setLoading] = useState(false);
  const [ticker, setTicker] = useState(localStorage.getItem('selectedTicker') || 'SPY');
  const [days, setDays] = useState(30);
  const [strategy, setStrategy] = useState('RedCandle');
  const [historicalData, setHistoricalData] = useState([]);
  const [signals, setSignals] = useState([]);
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
    fetchData();
  }, [ticker]);

  const fetchData = async () => {
    setLoading(true);
    setError(null);

    try {
      // Fetch data in parallel
      const [historicalResponse, signalsResponse] = await Promise.all([
        axios.get(`/api/historical-data?ticker=${ticker}&days=${days}`),
        axios.get(`/api/signals?ticker=${ticker}&days=${days}&strategy=${strategy}`)
      ]);

      setHistoricalData(historicalResponse.data);
      setSignals(signalsResponse.data);
    } catch (error) {
      console.error('Error fetching data:', error);
      setError('Failed to load data. Please try again later.');
    } finally {
      setLoading(false);
    }
  };

  const handleDaysChange = (event) => {
    setDays(event.target.value);
  };

  const handleStrategyChange = (event) => {
    setStrategy(event.target.value);
  };

  const handleChangePage = (event, newPage) => {
    setPage(newPage);
  };

  const handleChangeRowsPerPage = (event) => {
    setRowsPerPage(parseInt(event.target.value, 10));
    setPage(0);
  };

  // Format signals for table display
  const formattedSignals = signals.map(signal => {
    // Format date
    let formattedDate;
    try {
      const dateObj = new Date(signal.date);
      formattedDate = format(dateObj, 'yyyy-MM-dd HH:mm:ss');
    } catch (error) {
      formattedDate = signal.date;
    }

    return {
      ...signal,
      formattedDate
    };
  });

  return (
      <Box>
        <Typography variant="h4" gutterBottom>
          Trading Signals
        </Typography>

        <Paper sx={{ p: 3, mb: 3 }}>
          <Grid container spacing={2} alignItems="center">
            <Grid item xs={12} sm={12} md={4}>
              <Typography variant="h6" gutterBottom>
                {ticker} Trading Signals
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
                  <MenuItem value={5}>5 days</MenuItem>
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
            <Grid item xs={12} md={2}>
              <Button
                  variant="contained"
                  fullWidth
                  onClick={fetchData}
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
            <Loading message={`Generating signals for ${ticker}...`} />
        ) : (
            <>
              <SignalSummary signals={signals} loading={false} />

              {historicalData.length > 0 && (
                  <Paper sx={{ p: 2, mb: 3 }}>
                    <Typography variant="h6" gutterBottom>
                      Price Chart with Signals
                    </Typography>
                    <Divider sx={{ mb: 2 }} />
                    <CandlestickChart data={historicalData} signals={signals} />
                  </Paper>
              )}

              <Paper>
                <TableContainer>
                  <Table>
                    <TableHead>
                      <TableRow>
                        <TableCell>Date & Time</TableCell>
                        <TableCell>Signal Type</TableCell>
                        <TableCell align="right">Entry Price</TableCell>
                        <TableCell align="right">Stop Loss</TableCell>
                        <TableCell align="right">Risk (%)</TableCell>
                      </TableRow>
                    </TableHead>
                    <TableBody>
                      {formattedSignals.length > 0 ? (
                          formattedSignals
                          .slice(page * rowsPerPage, page * rowsPerPage + rowsPerPage)
                          .map((row, index) => {
                            const riskPercent = Math.abs((row.stoploss - row.entry_price) / row.entry_price * 100).toFixed(2);
                            const isLong = row.signal_type === 'LONG';

                            return (
                                <TableRow key={index}>
                                  <TableCell>{row.formattedDate}</TableCell>
                                  <TableCell>
                                    <Chip
                                        icon={isLong ? <TrendingUpIcon /> : <TrendingDownIcon />}
                                        label={row.signal_type}
                                        color={isLong ? "success" : "error"}
                                        size="small"
                                    />
                                  </TableCell>
                                  <TableCell align="right">${row.entry_price.toFixed(2)}</TableCell>
                                  <TableCell align="right">${row.stoploss.toFixed(2)}</TableCell>
                                  <TableCell align="right">{riskPercent}%</TableCell>
                                </TableRow>
                            );
                          })
                      ) : (
                          <TableRow>
                            <TableCell colSpan={5} align="center">
                              No signals generated for the selected period
                            </TableCell>
                          </TableRow>
                      )}
                    </TableBody>
                  </Table>
                  {formattedSignals.length > 0 && (
                      <TablePagination
                          rowsPerPageOptions={[5, 10, 25]}
                          component="div"
                          count={formattedSignals.length}
                          rowsPerPage={rowsPerPage}
                          page={page}
                          onPageChange={handleChangePage}
                          onRowsPerPageChange={handleChangeRowsPerPage}
                      />
                  )}
                </TableContainer>
              </Paper>
            </>
        )}
      </Box>
  );
};

export default Signals;