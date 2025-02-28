import React from 'react';
import {
  Card,
  CardContent,
  Typography,
  Chip,
  Box,
  Divider,
  Grid
} from '@mui/material';
import CallMadeIcon from '@mui/icons-material/CallMade';
import CallReceivedIcon from '@mui/icons-material/CallReceived';
import { format, parse } from 'date-fns';

const RecommendationCard = ({ recommendation }) => {
  const {
    date,
    signal_type,
    stock_price,
    stoploss,
    option_type,
    strike,
    expiration,
    delta,
    iv,
    price
  } = recommendation;

  // Format dates
  let formattedTradeDate;
  let formattedExpiration;

  try {
    const tradeDateObj = new Date(date);
    formattedTradeDate = format(tradeDateObj, 'MMM dd, yyyy');

    const expirationDateObj = new Date(expiration);
    formattedExpiration = format(expirationDateObj, 'MMM dd, yyyy');
  } catch (error) {
    formattedTradeDate = date;
    formattedExpiration = expiration;
  }

  // Calculate risk and potential reward
  const riskAmount = Math.abs(stock_price - stoploss).toFixed(2);
  const isBuy = signal_type === 'LONG';
  const cardClassName = `recommendation-card ${isBuy ? 'buy' : 'sell'}`;

  return (
      <Card className={cardClassName} sx={{ mb: 2 }}>
        <CardContent>
          <Grid container spacing={2}>
            <Grid item xs={12} sm={4}>
              <Box>
                <Typography variant="subtitle2" color="text.secondary">
                  Signal Date
                </Typography>
                <Typography variant="body1">
                  {formattedTradeDate}
                </Typography>
              </Box>

              <Box sx={{ mt: 2 }}>
                <Typography variant="subtitle2" color="text.secondary">
                  Signal Type
                </Typography>
                <Chip
                    icon={isBuy ? <CallMadeIcon /> : <CallReceivedIcon />}
                    label={signal_type}
                    color={isBuy ? "success" : "error"}
                    size="small"
                    sx={{ mt: 0.5 }}
                />
              </Box>

              <Box sx={{ mt: 2 }}>
                <Typography variant="subtitle2" color="text.secondary">
                  Stock Price
                </Typography>
                <Typography variant="body1">
                  ${stock_price.toFixed(2)}
                </Typography>
              </Box>

              <Box sx={{ mt: 2 }}>
                <Typography variant="subtitle2" color="text.secondary">
                  Stop Loss
                </Typography>
                <Typography variant="body1" color="error.main">
                  ${stoploss.toFixed(2)}
                </Typography>
              </Box>

              <Box sx={{ mt: 2 }}>
                <Typography variant="subtitle2" color="text.secondary">
                  Risk Amount
                </Typography>
                <Typography variant="body1">
                  ${riskAmount}
                </Typography>
              </Box>
            </Grid>

            <Grid item xs={12} sm={8}>
              <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', mb: 2 }}>
                <Typography variant="h6">
                  {option_type} Option
                </Typography>
                <Chip
                    label={`Strike: $${strike.toFixed(2)}`}
                    color="primary"
                    variant="outlined"
                />
              </Box>

              <Divider sx={{ mb: 2 }} />

              <Box sx={{ display: 'flex', flexWrap: 'wrap', gap: 3 }}>
                <Box>
                  <Typography variant="subtitle2" color="text.secondary">
                    Expiration
                  </Typography>
                  <Typography variant="body1">
                    {formattedExpiration}
                  </Typography>
                </Box>

                <Box>
                  <Typography variant="subtitle2" color="text.secondary">
                    Delta
                  </Typography>
                  <Typography variant="body1">
                    {delta.toFixed(2)}
                  </Typography>
                </Box>

                {iv !== undefined && (
                    <Box>
                      <Typography variant="subtitle2" color="text.secondary">
                        Implied Volatility
                      </Typography>
                      <Typography variant="body1">
                        {iv.toFixed(2)}%
                      </Typography>
                    </Box>
                )}

                {price !== undefined && (
                    <Box>
                      <Typography variant="subtitle2" color="text.secondary">
                        Option Price
                      </Typography>
                      <Typography variant="body1">
                        ${price.toFixed(2)}
                      </Typography>
                    </Box>
                )}
              </Box>

              <Box sx={{ mt: 3 }}>
                <Typography variant="subtitle2" color="text.secondary">
                  Recommendation
                </Typography>
                <Typography variant="body1">
                  {isBuy
                      ? `Buy ${option_type} option at strike $${strike.toFixed(2)} expiring on ${formattedExpiration}`
                      : `Buy ${option_type} option at strike $${strike.toFixed(2)} expiring on ${formattedExpiration}`
                  }
                </Typography>
                <Typography variant="body2" color="text.secondary" sx={{ mt: 1 }}>
                  {isBuy
                      ? `Set stop loss at $${stoploss.toFixed(2)}. This is a bullish trade based on a LONG signal.`
                      : `Set stop loss at $${stoploss.toFixed(2)}. This is a bearish trade based on a SHORT signal.`
                  }
                </Typography>
              </Box>
            </Grid>
          </Grid>
        </CardContent>
      </Card>
  );
};

export default RecommendationCard;