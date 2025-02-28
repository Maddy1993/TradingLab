import React from 'react';
import { Card, CardContent, Typography, Box, LinearProgress, Chip } from '@mui/material';
import TrendingUpIcon from '@mui/icons-material/TrendingUp';
import TrendingDownIcon from '@mui/icons-material/TrendingDown';

const BacktestCard = ({ title, result, onClick }) => {
  // Extract key metrics
  const {
    win_rate = 0,
    profit_factor = 0,
    total_return = 0,
    total_trades = 0,
    winning_trades = 0,
    losing_trades = 0,
    max_drawdown_pct = 0
  } = result || {};

  // Determine card color based on performance
  const isPositive = total_return > 0;
  const cardColor = isPositive ? '#1b5e20' : '#b71c1c';
  const backgroundColor = isPositive ? 'rgba(27, 94, 32, 0.1)' : 'rgba(183, 28, 28, 0.1)';

  return (
      <Card
          className="backtest-card"
          onClick={onClick}
          sx={{
            backgroundColor,
            borderLeft: `4px solid ${cardColor}`,
            transition: 'transform 0.2s',
            '&:hover': {
              transform: 'translateY(-5px)',
              boxShadow: 3
            }
          }}
      >
        <CardContent>
          <Typography variant="h6" component="div" gutterBottom>
            {title}
          </Typography>

          <Box sx={{ mb: 2 }}>
            <Typography variant="body2" color="text.secondary" gutterBottom>
              Win Rate
            </Typography>
            <LinearProgress
                variant="determinate"
                value={win_rate}
                color={win_rate > 50 ? "success" : "error"}
                sx={{ height: 8, borderRadius: 4 }}
            />
            <Typography variant="body2" align="right">
              {win_rate.toFixed(1)}%
            </Typography>
          </Box>

          <Box sx={{ display: 'flex', justifyContent: 'space-between', mb: 1 }}>
            <Box>
              <Typography variant="body2" color="text.secondary">
                Total Return:
              </Typography>
              <Typography
                  variant="h6"
                  color={isPositive ? "success.main" : "error.main"}
                  sx={{ display: 'flex', alignItems: 'center' }}
              >
                {isPositive ? <TrendingUpIcon fontSize="small" /> : <TrendingDownIcon fontSize="small" />}
                {total_return.toFixed(2)}
              </Typography>
            </Box>

            <Box>
              <Typography variant="body2" color="text.secondary">
                Profit Factor:
              </Typography>
              <Typography variant="h6">
                {profit_factor === Infinity ? '∞' : profit_factor.toFixed(2)}
              </Typography>
            </Box>
          </Box>

          <Box sx={{ display: 'flex', justifyContent: 'space-between', mb: 1 }}>
            <Chip
                label={`${total_trades} Trades`}
                size="small"
                color="primary"
                variant="outlined"
            />

            <Box sx={{ display: 'flex', gap: 1 }}>
              <Chip
                  label={`${winning_trades} Wins`}
                  size="small"
                  color="success"
                  variant="outlined"
              />
              <Chip
                  label={`${losing_trades} Losses`}
                  size="small"
                  color="error"
                  variant="outlined"
              />
            </Box>
          </Box>

          <Typography variant="body2" color="text.secondary">
            Max Drawdown: {max_drawdown_pct.toFixed(2)}%
          </Typography>
        </CardContent>
      </Card>
  );
};

export default BacktestCard;