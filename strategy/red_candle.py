import numpy as np
import pandas as pd
from .base import Strategy

class RedCandleStrategy(Strategy):
    """
    Implementation of the Red Candle Theory strategy

    The strategy follows these rules:
    1. Ignore the first candle of each trading day (9:30 AM candle for US markets)
    2. Observe the first red candle after the opening candle
    3. Find the candle (I) that breaks either the low or high of the first red candle observed (R)
    4. For subsequent candles:
       - If a candle breaks the low of candle I, enter a short position with stoploss at the high of candle I
       - If a candle breaks the high of candle I, enter a long position with stoploss at the low of candle I
    """

    def __init__(self, use_additional_filters=False, rsi_period=14, rsi_threshold=30, volume_factor=1.5):
        """
        Initialize Red Candle Strategy with parameters

        Parameters:
        use_additional_filters (bool): Whether to apply additional filters (RSI, volume)
        rsi_period (int): Period for RSI calculation if using additional filters
        rsi_threshold (int): RSI threshold for confirming signals if using additional filters
        volume_factor (float): Factor for volume increase detection if using additional filters
        """
        self.use_additional_filters = use_additional_filters
        self.rsi_period = rsi_period
        self.rsi_threshold = rsi_threshold
        self.volume_factor = volume_factor

    def generate_signals(self, df):
        """
        Generate trading signals based on the Red Candle Theory

        Parameters:
        df (pandas.DataFrame): Historical price data with OHLCV columns

        Returns:
        pandas.DataFrame: Data with signals added
        """
        # Make a copy to avoid modifying the original DataFrame
        df = df.copy()

        # Ensure the index is datetime for time-based operations
        if not isinstance(df.index, pd.DatetimeIndex):
            if 'date' in df.columns:
                df = df.set_index('date')
            else:
                raise ValueError("DataFrame must have a datetime index or a 'date' column")

        # Mark first candle of each trading day
        df['is_first_candle'] = False

        # Group by trading day
        df['trading_day'] = df.index.date

        # For each trading day, mark the first candle
        for day in df['trading_day'].unique():
            day_mask = df['trading_day'] == day
            if day_mask.any():
                first_idx = df[day_mask].index[0]
                df.loc[first_idx, 'is_first_candle'] = True

        # Apply Red Candle Theory logic
        df = self._apply_red_candle_theory(df)

        # Apply additional filters if requested
        if self.use_additional_filters:
            df = self._apply_additional_filters(df)

        return df

    def _apply_red_candle_theory(self, df):
        """Apply the Red Candle Theory logic to identify entry signals"""
        # Initialize signal columns
        df['first_red_candle'] = False  # Marks the first red candle (R)
        df['candle_i'] = False          # Marks the candle that breaks the high/low of R
        df['long_entry'] = False        # Signals for long entries
        df['short_entry'] = False       # Signals for short entries
        df['stoploss'] = np.nan         # Stoploss levels
        df['signal_type'] = ''          # Type of signal (LONG or SHORT)

        # Process each trading day separately
        for day in df['trading_day'].unique():
            # Get data for current trading day
            day_df = df[df['trading_day'] == day].copy()

            # We need at least 4 candles to apply this strategy:
            # (opening to ignore, potential red candle, I, and entry)
            if len(day_df) < 4:
                continue

            # Skip the first candle of the day (9:30 AM for US markets)
            # Start looking for red candles from the second candle of the day
            day_data = day_df[~day_df['is_first_candle']].copy()

            if len(day_data) < 3:  # Need at least 3 candles after skipping first
                continue

            # Find the first red candle after the opening candle
            first_red_idx = None
            for i in range(len(day_data)):
                # Check if this candle is red (close < open)
                if day_data.iloc[i]['close'] < day_data.iloc[i]['open']:
                    first_red_idx = day_data.index[i]
                    df.loc[first_red_idx, 'first_red_candle'] = True
                    break

            # If no red candle found, skip this day
            if first_red_idx is None:
                continue

            # Get high and low of the first red candle
            r_high = df.loc[first_red_idx, 'high']
            r_low = df.loc[first_red_idx, 'low']

            # Find subsequent data points after the first red candle
            subsequent_data = day_data.loc[day_data.index > first_red_idx]

            # Find candle I that breaks high or low of first red candle
            # Breaking means the candle should CLOSE beyond the level, not just touch it
            candle_i_idx = None
            candle_i_broke_high = False

            for idx in subsequent_data.index:
                current_close = df.loc[idx, 'close']

                if current_close > r_high:
                    candle_i_idx = idx
                    candle_i_broke_high = True
                    df.loc[idx, 'candle_i'] = True
                    break
                elif current_close < r_low:
                    candle_i_idx = idx
                    candle_i_broke_high = False
                    df.loc[idx, 'candle_i'] = True
                    break

            # If candle I not found, skip this day
            if candle_i_idx is None:
                continue

            # Get high and low of candle I
            i_high = df.loc[candle_i_idx, 'high']
            i_low = df.loc[candle_i_idx, 'low']

            # Track if we've already seen a signal at these levels to avoid duplicates
            high_signal_generated = False
            low_signal_generated = False

            # Keep track of previous closes to detect when a candle actually breaks a level
            last_close = df.loc[candle_i_idx, 'close']

            # Look for entry signals in subsequent candles
            for idx in subsequent_data.loc[subsequent_data.index > candle_i_idx].index:
                current_close = df.loc[idx, 'close']

                # Reset signals if price moves back into the range between high and low of candle I
                if last_close > i_high and current_close <= i_high:
                    high_signal_generated = False  # Reset high break signal
                elif last_close < i_low and current_close >= i_low:
                    low_signal_generated = False   # Reset low break signal

                # Check for long entry (CLOSE above candle I high)
                if current_close > i_high:
                    # Only generate signal when price actually breaks above i_high
                    # from below (or at) i_high level
                    if (last_close <= i_high or not high_signal_generated):
                        df.loc[idx, 'long_entry'] = True
                        df.loc[idx, 'stoploss'] = i_low
                        df.loc[idx, 'signal_type'] = 'LONG'
                        high_signal_generated = True

                # Check for short entry (CLOSE below candle I low)
                elif current_close < i_low:
                    # Only generate signal when price actually breaks below i_low
                    # from above (or at) i_low level
                    if (last_close >= i_low or not low_signal_generated):
                        df.loc[idx, 'short_entry'] = True
                        df.loc[idx, 'stoploss'] = i_high
                        df.loc[idx, 'signal_type'] = 'SHORT'
                        low_signal_generated = True

                # Update last close for next iteration
                last_close = current_close

        # Create a combined entry signal column
        df['entry_signal'] = df['long_entry'] | df['short_entry']

        return df

    def _apply_additional_filters(self, df):
        """Apply additional filters to entry signals"""
        # Calculate RSI
        delta = df['close'].diff()
        gain = delta.where(delta > 0, 0)
        loss = -delta.where(delta < 0, 0)

        avg_gain = gain.rolling(window=self.rsi_period).mean()
        avg_loss = loss.rolling(window=self.rsi_period).mean()

        rs = avg_gain / avg_loss
        df['rsi'] = 100 - (100 / (1 + rs))

        # Calculate volume increase
        df['volume_ratio'] = df['volume'] / df['volume'].rolling(window=5).mean()

        # Filter long entries - only enter longs when RSI is below threshold (oversold)
        long_entries = df['long_entry'].copy()
        df.loc[long_entries, 'long_entry'] = (
                                                     df.loc[long_entries, 'rsi'] <= self.rsi_threshold
                                             ) & (
                                                     df.loc[long_entries, 'volume_ratio'] >= self.volume_factor
                                             )

        # Filter short entries - only enter shorts when RSI is above 100-threshold (overbought)
        short_entries = df['short_entry'].copy()
        df.loc[short_entries, 'short_entry'] = (
                                                       df.loc[short_entries, 'rsi'] >= (100 - self.rsi_threshold)
                                               ) & (
                                                       df.loc[short_entries, 'volume_ratio'] >= self.volume_factor
                                               )

        # Update the combined entry signal
        df['entry_signal'] = df['long_entry'] | df['short_entry']

        return df