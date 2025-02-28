import React, { useEffect, useRef } from 'react';
import { createChart } from 'lightweight-charts';
import { Box, useTheme } from '@mui/material';

const TradingViewChart = ({ data, signals, height = 500 }) => {
  const chartContainerRef = useRef(null);
  const theme = useTheme();
  const isDarkMode = theme.palette.mode === 'dark';

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
        mode: 0, // CrosshairMode.Normal
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
  }, [data, signals, height, isDarkMode]);

  return (
      <Box
          ref={chartContainerRef}
          sx={{
            width: '100%',
            height: `${height}px`,
          }}
      />
  );
};

export default TradingViewChart;