class OptionsRecommender:
    """Class for generating options recommendations based on entry signals"""

    def __init__(self, min_delta=0.30, max_delta=0.60, target_delta=0.45):
        """
        Initialize Options Recommender

        Parameters:
        min_delta (float): Minimum delta for option selection
        max_delta (float): Maximum delta for option selection
        target_delta (float): Target delta for optimal option selection
        """
        self.min_delta = min_delta
        self.max_delta = max_delta
        self.target_delta = target_delta

    def generate_recommendations(self, signal_df, options_df):
        """
        Generate options recommendations based on entry signals

        Parameters:
        signal_df (pandas.DataFrame): DataFrame with entry signals
        options_df (pandas.DataFrame): DataFrame with options data

        Returns:
        list: List of option recommendations for entry points
        """
        # Get dates with entry signals
        entry_dates = signal_df[signal_df['entry_signal']].index

        recommendations = []

        for date in entry_dates:
            date_str = date.strftime('%Y-%m-%d')

            # Get current stock price and signal type
            current_price = signal_df.loc[date, 'close']
            signal_type = signal_df.loc[date, 'signal_type']
            stoploss = signal_df.loc[date, 'stoploss']

            # Calculate risk (distance to stoploss)
            risk = abs(current_price - stoploss)

            # Filter options for the current date
            if 'tradeDate' in options_df.columns:
                day_options = options_df[options_df['tradeDate'] == date_str]

                # If we have options data for this date
                if not day_options.empty:
                    if signal_type == 'LONG':
                        # For long signals, we want call options
                        # Find call options for strikes above current price
                        filtered_options = day_options[
                            (day_options['strike'] > current_price) &
                            (day_options['delta'] > self.min_delta) &
                            (day_options['delta'] < self.max_delta)
                            ]
                        option_type = 'CALL'
                        price_column = 'callValue'

                    elif signal_type == 'SHORT':
                        # For short signals, we want put options
                        # Find put options for strikes below current price
                        filtered_options = day_options[
                            (day_options['strike'] < current_price) &
                            (-day_options['delta'] > self.min_delta) &  # Put delta is negative
                            (-day_options['delta'] < self.max_delta)
                            ]
                        option_type = 'PUT'
                        price_column = 'putValue'
                    else:
                        # Skip if no valid signal type
                        continue

                    if not filtered_options.empty:
                        # Adjust target delta based on signal type
                        target = self.target_delta if signal_type == 'LONG' else -self.target_delta

                        # Get the option with delta closest to target delta
                        filtered_options['delta_target'] = abs(filtered_options['delta'] - target)
                        best_option = filtered_options.loc[filtered_options['delta_target'].idxmin()]

                        # Calculate risk/reward
                        option_price = best_option[price_column] if price_column in best_option else None

                        recommendation = {
                            'date': date_str,
                            'signal_type': signal_type,
                            'stock_price': current_price,
                            'stoploss': stoploss,
                            'risk': risk,
                            'option_type': option_type,
                            'strike': best_option['strike'],
                            'expiration': best_option['expirDate'],
                            'delta': best_option['delta'],
                            'iv': best_option['iv'] if 'iv' in best_option else None,
                            'price': option_price
                        }

                        recommendations.append(recommendation)

        return recommendations