import React, { useState, useEffect } from 'react';
import {
  Typography,
  Paper,
  Box,
  Chip,
  List,
  ListItem,
  ListItemText,
  Divider,
  Alert,
  IconButton
} from '@mui/material';
import TrendingUpIcon from '@mui/icons-material/TrendingUp';
import TrendingDownIcon from '@mui/icons-material/TrendingDown';
import NotificationsIcon from '@mui/icons-material/Notifications';
import ClearAllIcon from '@mui/icons-material/ClearAll';
import { useSignals } from '../services/websocketService';

const RealTimeSignals = ({ ticker }) => {
  const signals = useSignals(ticker);
  const [notifications, setNotifications] = useState([]);
  const [showNotification, setShowNotification] = useState(false);

  // When we receive a new signal, create a notification
  useEffect(() => {
    if (signals.length > 0) {
      const latestSignal = signals[signals.length - 1];

      // Only create notification for new signals
      if (!notifications.find(n =>
          n.timestamp === latestSignal.timestamp &&
          n.signal_type === latestSignal.signal_type
      )) {
        const newNotification = {
          id: Date.now(),
          timestamp: latestSignal.timestamp,
          ticker: latestSignal.ticker,
          signal_type: latestSignal.signal_type,
          entry_price: latestSignal.entry_price,
          seen: false
        };

        setNotifications(prev => [...prev, newNotification]);

        // Show notification
        setShowNotification(true);

        // Auto-hide after 5 seconds
        setTimeout(() => {
          setShowNotification(false);
        }, 5000);
      }
    }
  }, [signals, notifications]);

  const clearNotifications = () => {
    setNotifications([]);
    setShowNotification(false);
  };

  // Format date for display
  const formatDate = (dateStr) => {
    try {
      const date = new Date(dateStr);
      return date.toLocaleString();
    } catch (e) {
      return dateStr;
    }
  };

  return (
      <>
        {/* Notification alert */}
        {showNotification && notifications.length > 0 && (
            <Alert
                severity="info"
                sx={{ mb: 2, position: 'relative' }}
                icon={<NotificationsIcon />}
                action={
                  <IconButton
                      aria-label="close"
                      color="inherit"
                      size="small"
                      onClick={() => setShowNotification(false)}
                  >
                    <ClearAllIcon fontSize="inherit" />
                  </IconButton>
                }
            >
              {notifications.length === 1 ? (
                  <span>
              New {notifications[0].signal_type} signal for {notifications[0].ticker} at ${notifications[0].entry_price.toFixed(2)}
            </span>
              ) : (
                  <span>{notifications.length} new trading signals</span>
              )}
            </Alert>
        )}

        <Paper sx={{ p: 2, mb: 3 }}>
          <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', mb: 2 }}>
            <Typography variant="h6">
              Real-Time Trading Signals
            </Typography>

            {notifications.length > 0 && (
                <IconButton size="small" onClick={clearNotifications}>
                  <ClearAllIcon />
                </IconButton>
            )}
          </Box>

          {signals.length === 0 ? (
              <Typography variant="body2" color="text.secondary">
                Waiting for trading signals...
              </Typography>
          ) : (
              <List>
                {signals.slice(-5).reverse().map((signal, index) => (
                    <React.Fragment key={`${signal.timestamp}-${index}`}>
                      <ListItem alignItems="flex-start">
                        <ListItemText
                            primary={
                              <Box sx={{ display: 'flex', alignItems: 'center' }}>
                                <Chip
                                    icon={signal.signal_type === 'LONG' ? <TrendingUpIcon /> : <TrendingDownIcon />}
                                    label={signal.signal_type}
                                    color={signal.signal_type === 'LONG' ? "success" : "error"}
                                    size="small"
                                    sx={{ mr: 1 }}
                                />
                                <Typography component="span">
                                  ${signal.entry_price.toFixed(2)}
                                </Typography>
                              </Box>
                            }
                            secondary={
                              <>
                                <Typography component="span" variant="body2" color="text.secondary">
                                  {formatDate(signal.timestamp)}
                                </Typography>
                                <br />
                                <Typography component="span" variant="body2">
                                  Stoploss: ${signal.stoploss?.toFixed(2) || 'N/A'}
                                </Typography>
                              </>
                            }
                        />
                      </ListItem>
                      {index < signals.length - 1 && <Divider component="li" />}
                    </React.Fragment>
                ))}
              </List>
          )}
        </Paper>
      </>
  );
};

export default RealTimeSignals;