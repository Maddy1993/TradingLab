import React, { useEffect, useRef, useState } from 'react';
import { createChart, CrosshairMode } from 'lightweight-charts';
import {
  Box,
  useTheme,
  Paper,
  Typography,
  ToggleButtonGroup,
  ToggleButton
} from '@mui/material';

const TimeRangeSelector = ({ selectedRange, onRangeChange }) => {
  const ranges = [
    { value: '1M', label: '1M' },
    { value: '5M', label: '5M' },
    { value: '15M', label: '15M' },
    { value: '30M', label: '30M' },
    { value: '1H', label: '1HR' },
    { value: '2H', label: '2HR' },
    { value: '4H', label: '4HR' },
    { value: '1D', label: '1D' },
  ];

  return (
      <Box sx={{ mb: 2, display: 'flex', justifyContent: 'center' }}>
        <ToggleButtonGroup
            value={selectedRange}
            exclusive
            onChange={(event, newValue) => {
              if (newValue !== null) {
                onRangeChange(newValue);
              }
            }}
            size="small"
            aria-label="time range"
        >
          {ranges.map((range) => (
              <ToggleButton
                  key={range.value}
                  value={range.value}
                  sx={{
                    px: 1.5,
                    py: 0.5,
                    fontSize: '0.75rem',
                  }}
              >
                {range.label}
              </ToggleButton>
          ))}
        </ToggleButtonGroup>
      </Box>
  );
};

const TradingViewChart = ({
  data,
  signals,
  height = 500,
  onRangeChange,
  initialRange = '15M',
  focusedSignal = null
}) => {
  const chartContainerRef = useRef(null);
  const chartRef = useRef(null);
  const candlestickSeriesRef = useRef(null);
  const theme = useTheme();
  const isDarkMode = theme.palette.mode === 'dark';
  const [tooltip, setTooltip] = useState(null);
  const [timeRange, setTimeRange] = useState(initialRange);

  // Update timeRange if initialRange changes (from parent component)
  useEffect(() => {
    setTimeRange(initialRange);
  }, [initialRange]);

  const handleRangeChange = (newRange) => {
    setTimeRange(newRange);
    if (onRangeChange) {
      onRangeChange(newRange);
    }
  };

  useEffect(() => {
    if (!data || data.length === 0 || !chartContainerRef.current) {
      return;
    }

    // Clear previous chart if any
    chartContainerRef.current.innerHTML = '';

    // Set chart options based on theme
    const chartOptions = {
      width: chartContainerRef.current.clientWidth,
      height: height,
      layout: {
        background: {
          color: isDarkMode ? '#1e1e1e' : '#ffffff',
        },
        textColor: isDarkMode ? '#d1d4dc' : '#333',
      },
      grid: {
        vertLines: {
          color: isDarkMode ? '#2e2e2e' : '#f0f0f0',
        },
        horzLines: {
          color: isDarkMode ? '#2e2e2e' : '#f0f0f0',
        },
      },
      crosshair: {
        mode: CrosshairMode.Normal,
      },
      rightPriceScale: {
        borderColor: isDarkMode ? '#2e2e2e' : '#d6d6d6',
      },
      timeScale: {
        borderColor: isDarkMode ? '#2e2e2e' : '#d6d6d6',
        timeVisible: true,
      },
    };

    // Create chart
    const chart = createChart(chartContainerRef.current, chartOptions);
    chartRef.current = chart;

    // Format data for candlestick series
    const candleData = data.map(d => {
      // Parse date string to timestamp
      let timestamp;
      try {
        timestamp = new Date(d.date).getTime() / 1000;
      } catch (e) {
        // If date parsing fails, use sequential time
        timestamp = data.indexOf(d);
      }

      return {
        time: timestamp,
        open: d.open,
        high: d.high,
        low: d.low,
        close: d.close,
        // Store original data for tooltip
        originalDate: d.date
      };
    });

    // Create and add candlestick series
    const candlestickSeries = chart.addCandlestickSeries({
      upColor: '#4caf50',   // Green for bullish candles
      downColor: '#f44336', // Red for bearish candles
      borderVisible: false,
      wickUpColor: '#4caf50',
      wickDownColor: '#f44336',
    });

    candlestickSeriesRef.current = candlestickSeries;
    candlestickSeries.setData(candleData);

    // Add signal markers if available
    if (signals && signals.length > 0) {
      // Create a marker series
      const markersSeries = chart.addLineSeries({
        lineWidth: 0, // We only want markers, not a line
        lastValueVisible: false,
        priceLineVisible: false,
      });

      const markers = signals.map(signal => {
        // Find matching data point
        const matchingPoint = data.find(d => d.date === signal.date);
        if (!matchingPoint) return null;

        // Parse date
        let timestamp;
        try {
          timestamp = new Date(signal.date).getTime() / 1000;
        } catch (e) {
          // If date parsing fails, find index in data
          const index = data.findIndex(d => d.date === signal.date);
          timestamp = index >= 0 ? index : null;
        }

        if (!timestamp) return null;

        const isLong = signal.signal_type === 'LONG';
        return {
          time: timestamp,
          position: isLong ? 'belowBar' : 'aboveBar',
          color: isLong ? '#4caf50' : '#f44336',
          shape: isLong ? 'arrowUp' : 'arrowDown',
          text: signal.signal_type,
          size: 2,
        };
      }).filter(Boolean);

      if (markers.length > 0) {
        markersSeries.setMarkers(markers);
      }
    }

    // Setup tooltip for OHLC values
    chart.subscribeCrosshairMove(param => {
      if (
          param.point === undefined ||
          !param.time ||
          param.point.x < 0 ||
          param.point.x > chartContainerRef.current.clientWidth ||
          param.point.y < 0 ||
          param.point.y > height
      ) {
        // Hide tooltip when mouse leaves chart area
        setTooltip(null);
      } else {
        // Find the data point for the crosshair position
        const dataPoint = candleData.find(d => d.time === param.time);
        if (dataPoint) {
          // Show tooltip with OHLC values
          setTooltip({
            date: dataPoint.originalDate,
            open: dataPoint.open.toFixed(2),
            high: dataPoint.high.toFixed(2),
            low: dataPoint.low.toFixed(2),
            close: dataPoint.close.toFixed(2),
            x: param.point.x,
            y: param.point.y
          });
        }
      }
    });

    // Handle window resizing
    const handleResize = () => {
      if (chartContainerRef.current) {
        chart.applyOptions({ width: chartContainerRef.current.clientWidth });
      }
    };

    window.addEventListener('resize', handleResize);

    // Fit content
    chart.timeScale().fitContent();

    return () => {
      chart.remove();
      window.removeEventListener('resize', handleResize);
    };
  }, [data, signals, height, isDarkMode, timeRange]);

  // Effect to handle focused signal
  useEffect(() => {
    if (!focusedSignal || !chartRef.current || !data || data.length === 0) return;

    // Find the data point for this signal
    const signalPoint = data.find(d => d.date === focusedSignal.date);
    if (!signalPoint) return;

    // Convert to timestamp
    let timestamp;
    try {
      timestamp = new Date(focusedSignal.date).getTime() / 1000;
    } catch (e) {
      const index = data.findIndex(d => d.date === focusedSignal.date);
      timestamp = index >= 0 ? index : null;
    }

    if (!timestamp) return;

    // Scroll to the bar
    const timeScale = chartRef.current.timeScale();
    timeScale.scrollToPosition(timeScale.coordinateToLogical(chartContainerRef.current.clientWidth / 2) - timeScale.timeToCoordinate(timestamp), true);

  }, [focusedSignal, data]);

  return (
      <Box>
        <TimeRangeSelector
            selectedRange={timeRange}
            onRangeChange={handleRangeChange}
        />

        <Box sx={{ position: 'relative', width: '100%', height: `${height}px` }}>
          <Box
              ref={chartContainerRef}
              sx={{
                width: '100%',
                height: '100%',
              }}
          />

          {tooltip && (
              <Paper
                  sx={{
                    position: 'absolute',
                    left: `${Math.min(tooltip.x, chartContainerRef.current.clientWidth - 150)}px`,
                    top: `${Math.min(tooltip.y, height - 150)}px`,
                    padding: 1,
                    backgroundColor: isDarkMode ? 'rgba(30,30,30,0.9)' : 'rgba(255,255,255,0.9)',
                    borderRadius: 1,
                    boxShadow: 3,
                    zIndex: 1000,
                    pointerEvents: 'none', // Don't interfere with mouse events
                  }}
              >
                <Typography variant="subtitle2">{tooltip.date}</Typography>
                <Box sx={{ display: 'grid', gridTemplateColumns: 'auto auto', gap: 1 }}>
                  <Typography variant="body2" color="text.secondary">Open:</Typography>
                  <Typography variant="body2">${tooltip.open}</Typography>

                  <Typography variant="body2" color="text.secondary">High:</Typography>
                  <Typography variant="body2">${tooltip.high}</Typography>

                  <Typography variant="body2" color="text.secondary">Low:</Typography>
                  <Typography variant="body2">${tooltip.low}</Typography>

                  <Typography variant="body2" color="text.secondary">Close:</Typography>
                  <Typography variant="body2">${tooltip.close}</Typography>
                </Box>
              </Paper>
          )}
        </Box>
      </Box>
  );
};

export default TradingViewChart;