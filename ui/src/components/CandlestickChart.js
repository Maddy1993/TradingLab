import React from 'react';
import {
  ComposedChart,
  Line,
  Bar,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  Legend,
  ResponsiveContainer,
  ReferenceLine
} from 'recharts';
import { format } from 'date-fns';

const CandlestickChart = ({ data, signals }) => {
  if (!data || data.length === 0) {
    return <div>No data available for chart</div>;
  }

  // Process data to add color based on candle direction
  const processedData = data.map((candle, index) => {
    // Format the date for display
    let formattedDate;
    try {
      const dateObj = new Date(candle.date);
      formattedDate = format(dateObj, 'MM/dd HH:mm');
    } catch (error) {
      formattedDate = candle.date;
    }

    // Determine candle color based on open/close relationship
    const isGreen = candle.close >= candle.open;

    return {
      ...candle,
      index,
      formattedDate,
      color: isGreen ? '#4caf50' : '#f44336',
      // For bar chart - need to provide appropriate high and low
      highLowDiff: candle.high - candle.low,
      openCloseDiff: Math.abs(candle.close - candle.open),
      // Handle the drawing of candle body
      bodyStart: Math.min(candle.open, candle.close),
      bodyEnd: Math.max(candle.open, candle.close),
    };
  });

  // Find signal markers if present
  const signalMarkers = signals ? signals.map(signal => {
    // Find the corresponding data point
    const matchingCandle = processedData.find(candle =>
        candle.date === signal.date || candle.formattedDate === signal.date
    );

    if (matchingCandle) {
      return {
        index: matchingCandle.index,
        price: signal.entry_price,
        type: signal.signal_type,
        stoploss: signal.stoploss
      };
    }
    return null;
  }).filter(Boolean) : [];

  // Custom tooltip to display candle data
  const CustomTooltip = ({ active, payload }) => {
    if (active && payload && payload.length > 0) {
      const data = payload[0].payload;

      return (
          <div className="candlestick-tooltip">
            <p>{`Date: ${data.formattedDate}`}</p>
            <p>{`Open: ${data.open.toFixed(2)}`}</p>
            <p>{`High: ${data.high.toFixed(2)}`}</p>
            <p>{`Low: ${data.low.toFixed(2)}`}</p>
            <p>{`Close: ${data.close.toFixed(2)}`}</p>
            <p>{`Volume: ${data.volume.toLocaleString()}`}</p>
          </div>
      );
    }

    return null;
  };

  return (
      <ResponsiveContainer width="100%" height={500}>
        <ComposedChart
            data={processedData}
            margin={{ top: 20, right: 30, left: 20, bottom: 50 }}
        >
          <CartesianGrid strokeDasharray="3 3" />
          <XAxis
              dataKey="formattedDate"
              angle={-45}
              textAnchor="end"
              height={80}
              interval="preserveStartEnd"
              tickCount={10}
          />
          <YAxis
              domain={['auto', 'auto']}
              tickCount={8}
              allowDecimals={true}
              tickFormatter={(value) => value.toFixed(2)}
          />
          <Tooltip content={<CustomTooltip />} />
          <Legend />

          {/* High-low wicks */}
          <Bar
              dataKey="highLowDiff"
              fill="transparent"
              stroke="#000"
              barSize={3}
              yAxisId={0}
              stackId="stack"
              baseValue={(d) => d.low}
          />

          {/* Candle bodies */}
          <Bar
              dataKey="openCloseDiff"
              yAxisId={0}
              barSize={10}
              fill="transparent"
              stroke="none"
              stackId="separate"
              baseValue={(d) => d.bodyStart}
              shape={(props) => {
                const { x, y, width, height, fill, payload } = props;
                return (
                    <rect
                        x={x - width / 2}
                        y={y}
                        width={width}
                        height={height}
                        fill={payload.color}
                        stroke="#000"
                        strokeWidth={1}
                    />
                );
              }}
          />

          {/* Signal markers */}
          {signalMarkers.map((signal, idx) => (
              <ReferenceLine
                  key={`signal-${idx}`}
                  x={signal.index}
                  stroke={signal.type === 'LONG' ? '#4caf50' : '#f44336'}
                  strokeWidth={2}
                  isFront={true}
                  label={{
                    value: signal.type,
                    position: 'top',
                    fill: signal.type === 'LONG' ? '#4caf50' : '#f44336',
                  }}
              />
          ))}

          {/* Stoploss lines */}
          {signalMarkers.map((signal, idx) => (
              signal.stoploss && (
                  <ReferenceLine
                      key={`stoploss-${idx}`}
                      y={signal.stoploss}
                      stroke={signal.type === 'LONG' ? '#f44336' : '#4caf50'}
                      strokeDasharray="3 3"
                      label={{
                        value: 'Stop',
                        position: 'right',
                        fill: signal.type === 'LONG' ? '#f44336' : '#4caf50',
                      }}
                  />
              )
          ))}
        </ComposedChart>
      </ResponsiveContainer>
  );
};

export default CandlestickChart;