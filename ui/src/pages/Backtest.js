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
  TextField,
  Button,
  Divider,
  Alert,
  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow
} from '@mui/material';
import axios from 'axios';
import {
  BarChart,
  Bar,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  Legend,
  ResponsiveContainer,
  PieChart,
  Pie,
  Cell
} from 'recharts';
import BacktestCard from '../components/BacktestCard';
import Loading from '../components/Loading';

const Backtest = () => {
  const [loading, setLoading] = useState(false);
  const [ticker, setTicker] = useState(localStorage.getItem('selectedTicker') || 'SPY');
  const [days, setDays] = useState(30);
  const [strategy, setStrategy] = useState('RedCandle');
  const [profitTargets, setProfitTargets] = useState('5,10,15');
  const [riskRewardRatios, setRiskRewardRatios] = useState('1,2,3');
  const [profitTargetsDollar, setProfitTargetsDollar] = useState('100,250,500');
  const [backtestResults, setBacktestResults] = useState({});
  const [error, setError] = useState(null);
  const [selectedResult, setSelectedResult] = useState(null);
  const [detailsOpen, setDetailsOpen] = useState(false);

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
    runBacktest();
  }, [ticker]);

  const runBacktest = async () => {
    setLoading(true);
    setError(null);

    try {
      // Prepare query parameters
      const params = new URLSearchParams({
        ticker,
        days,
        strategy,
        profit_targets: profitTargets,
        risk_reward_ratios: riskRewardRatios,
        profit_targets_dollar: profitTargetsDollar
      });

      const response = await axios.get(`/api/backtest?${params.toString()}`);
      setBacktestResults(response.data);
    } catch (error) {
      console.error('Error running backtest:', error);
      setError('Failed to run backtest. Please try again later.');
    } finally {
      setLoading(false);
    }
  };

  const handleShowDetails = (key, result) => {
    setSelectedResult({
      title: key,
      ...result
    });
    setDetailsOpen(true);
  };

  const handleCloseDetails = () => {
    setDetailsOpen(false);
  };

  // Prepare chart data
  const winRateData = Object.entries(backtestResults).map(([key, result]) => ({
    name: key,
    winRate: result.win_rate,
    profitFactor: Math.min(result.profit_factor, 5) // Cap at 5 for better visualization
  }));

  const tradesData = Object.entries(backtestResults).map(([key, result]) => ({
    name: key,
    winning: result.winning_trades,
    losing: result.losing_trades
  }));

  // Colors for charts
  const COLORS = ['#4caf50', '#f44336', '#2196f3', '#ff9800', '#9c27b0', '#00bcd4'];

  // Pie chart data for selected result
  const selectedResultPieData = selectedResult ? [
    { name: 'Winning', value: selectedResult.winning_trades },
    { name: 'Losing', value: selectedResult.losing_trades }
  ] : [];

  return (
      <Box>
        <Typography variant="h4" gutterBottom>
          Backtest Results
        </Typography>

        <Paper sx={{ p: 3, mb: 3 }}>
          <Grid container spacing={2} alignItems="center">
            <Grid item xs={12} sm={6} md={3}>
              <Typography variant="h6" gutterBottom>
                {ticker} Backtest
              </Typography>
            </Grid>
            <Grid item xs={12} sm={6} md={3}>
              <FormControl fullWidth>
                <InputLabel>Days</InputLabel>
                <Select
                    value={days}
                    label="Days"
                    onChange={(e) => setDays(e.target.value)}
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
                    onChange={(e) => setStrategy(e.target.value)}
                >
                  <MenuItem value="RedCandle">Red Candle Theory</MenuItem>
                </Select>
              </FormControl>
            </Grid>
            <Grid item xs={12} sm={6} md={3}>
              <Button
                  variant="contained"
                  fullWidth
                  onClick={runBacktest}
              >
                Run Backtest
              </Button>
            </Grid>
          </Grid>

          <Divider sx={{ my: 2 }} />

          <Grid container spacing={2}>
            <Grid item xs={12} sm={4}>
              <TextField
                  label="Profit Targets (%)"
                  fullWidth
                  value={profitTargets}
                  onChange={(e) => setProfitTargets(e.target.value)}
                  helperText="Comma-separated values, e.g. 5,10,15"
              />
            </Grid>
            <Grid item xs={12} sm={4}>
              <TextField
                  label="Risk-Reward Ratios"
                  fullWidth
                  value={riskRewardRatios}
                  onChange={(e) => setRiskRewardRatios(e.target.value)}
                  helperText="Comma-separated values, e.g. 1,2,3"
              />
            </Grid>
            <Grid item xs={12} sm={4}>
              <TextField
                  label="Profit Targets ($)"
                  fullWidth
                  value={profitTargetsDollar}
                  onChange={(e) => setProfitTargetsDollar(e.target.value)}
                  helperText="Comma-separated values, e.g. 100,250,500"
              />
            </Grid>
          </Grid>
        </Paper>

        {error && (
            <Alert severity="error" sx={{ mb: 3 }}>
              {error}
            </Alert>
        )}

        {loading ? (
            <Loading message={`Running backtest for ${ticker}...`} />
        ) : (
            <>
              {Object.keys(backtestResults).length > 0 ? (
                  <>
                    <Typography variant="h5" gutterBottom>
                      Results Summary
                    </Typography>

                    <Grid container spacing={3} sx={{ mb: 3 }}>
                      {/* Win Rate Chart */}
                      <Grid item xs={12} md={6}>
                        <Paper sx={{ p: 2, height: 350 }}>
                          <Typography variant="h6" gutterBottom>
                            Win Rate & Profit Factor
                          </Typography>
                          <ResponsiveContainer width="100%" height="90%">
                            <BarChart
                                data={winRateData}
                                margin={{ top: 10, right: 30, left: 0, bottom: 30 }}
                            >
                              <CartesianGrid strokeDasharray="3 3" />
                              <XAxis dataKey="name" angle={-45} textAnchor="end" height={60} />
                              <YAxis yAxisId="left" orientation="left" label={{ value: 'Win Rate (%)', angle: -90, position: 'insideLeft' }} />
                              <YAxis yAxisId="right" orientation="right" label={{ value: 'Profit Factor', angle: 90, position: 'insideRight' }} />
                              <Tooltip />
                              <Legend />
                              <Bar yAxisId="left" dataKey="winRate" name="Win Rate (%)" fill="#8884d8" />
                              <Bar yAxisId="right" dataKey="profitFactor" name="Profit Factor" fill="#82ca9d" />
                            </BarChart>
                          </ResponsiveContainer>
                        </Paper>
                      </Grid>

                      {/* Trades Chart */}
                      <Grid item xs={12} md={6}>
                        <Paper sx={{ p: 2, height: 350 }}>
                          <Typography variant="h6" gutterBottom>
                            Winning vs Losing Trades
                          </Typography>
                          <ResponsiveContainer width="100%" height="90%">
                            <BarChart
                                data={tradesData}
                                margin={{ top: 10, right: 30, left: 0, bottom: 30 }}
                            >
                              <CartesianGrid strokeDasharray="3 3" />
                              <XAxis dataKey="name" angle={-45} textAnchor="end" height={60} />
                              <YAxis />
                              <Tooltip />
                              <Legend />
                              <Bar dataKey="winning" name="Winning Trades" fill="#4caf50" />
                              <Bar dataKey="losing" name="Losing Trades" fill="#f44336" />
                            </BarChart>
                          </ResponsiveContainer>
                        </Paper>
                      </Grid>
                    </Grid>

                    <Typography variant="h5" gutterBottom>
                      Detailed Results
                    </Typography>

                    <Grid container spacing={2}>
                      {Object.entries(backtestResults).map(([key, result]) => (
                          <Grid item xs={12} sm={6} md={4} key={key}>
                            <BacktestCard
                                title={key}
                                result={result}
                                onClick={() => handleShowDetails(key, result)}
                            />
                          </Grid>
                      ))}
                    </Grid>
                  </>
              ) : (
                  <Alert severity="info">
                    Run a backtest to see results.
                  </Alert>
              )}
            </>
        )}

        {/* Details Dialog */}
        <Dialog
            open={detailsOpen}
            onClose={handleCloseDetails}
            maxWidth="md"
            fullWidth
        >
          {selectedResult && (
              <>
                <DialogTitle>
                  {selectedResult.title} - Detailed Results
                </DialogTitle>
                <DialogContent>
                  <Grid container spacing={3}>
                    <Grid item xs={12} md={6}>
                      <TableContainer>
                        <Table>
                          <TableBody>
                            <TableRow>
                              <TableCell><strong>Win Rate</strong></TableCell>
                              <TableCell>{selectedResult.win_rate.toFixed(2)}%</TableCell>
                            </TableRow>
                            <TableRow>
                              <TableCell><strong>Profit Factor</strong></TableCell>
                              <TableCell>{selectedResult.profit_factor === Infinity ? '∞' : selectedResult.profit_factor.toFixed(2)}</TableCell>
                            </TableRow>
                            <TableRow>
                              <TableCell><strong>Total Return</strong></TableCell>
                              <TableCell>{selectedResult.total_return.toFixed(2)}</TableCell>
                            </TableRow>
                            <TableRow>
                              <TableCell><strong>Total Return (%)</strong></TableCell>
                              <TableCell>{selectedResult.total_return_pct.toFixed(2)}%</TableCell>
                            </TableRow>
                            <TableRow>
                              <TableCell><strong>Max Drawdown</strong></TableCell>
                              <TableCell>${selectedResult.max_drawdown.toFixed(2)} ({selectedResult.max_drawdown_pct.toFixed(2)}%)</TableCell>
                            </TableRow>
                            <TableRow>
                              <TableCell><strong>Total Trades</strong></TableCell>
                              <TableCell>{selectedResult.total_trades}</TableCell>
                            </TableRow>
                            <TableRow>
                              <TableCell><strong>Winning Trades</strong></TableCell>
                              <TableCell>{selectedResult.winning_trades}</TableCell>
                            </TableRow>
                            <TableRow>
                              <TableCell><strong>Losing Trades</strong></TableCell>
                              <TableCell>{selectedResult.losing_trades}</TableCell>
                            </TableRow>
                          </TableBody>
                        </Table>
                      </TableContainer>
                    </Grid>
                    <Grid item xs={12} md={6}>
                      <Typography variant="h6" align="center" gutterBottom>
                        Trade Distribution
                      </Typography>
                      <ResponsiveContainer width="100%" height={250}>
                        <PieChart>
                          <Pie
                              data={selectedResultPieData}
                              dataKey="value"
                              nameKey="name"
                              cx="50%"
                              cy="50%"
                              outerRadius={80}
                              label={(entry) => `${entry.name}: ${entry.value}`}
                          >
                            {selectedResultPieData.map((entry, index) => (
                                <Cell key={`cell-${index}`} fill={index === 0 ? '#4caf50' : '#f44336'} />
                            ))}
                          </Pie>
                          <Tooltip formatter={(value) => [value, 'Trades']} />
                        </PieChart>
                      </ResponsiveContainer>
                    </Grid>
                  </Grid>
                </DialogContent>
                <DialogActions>
                  <Button onClick={handleCloseDetails}>Close</Button>
                </DialogActions>
              </>
          )}
        </Dialog>
      </Box>
  );
};

export default Backtest;