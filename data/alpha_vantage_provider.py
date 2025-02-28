import requests
import pandas as pd
import numpy as np
from datetime import datetime, timedelta
import os

from .provider import DataProvider

class AlphaVantageDataProvider(DataProvider):
    """Data provider that fetches intraday and options data from Alpha Vantage API"""

    def __init__(self, api_key=None, interval='15min'):
        """
        Initialize the Alpha Vantage data provider

        Parameters:
        api_key (str): Alpha Vantage API key (if None, will try to get from environment variable)
        interval (str): Default interval for intraday data (1min, 5min, 15min, 30min, 60min)
        """
        self.api_key = api_key or os.getenv('ALPHA_VANTAGE_API_KEY')
        if not self.api_key:
            raise ValueError("Alpha Vantage API key is required. Provide it as a parameter or set ALPHA_VANTAGE_API_KEY environment variable.")

        self.base_url = "https://www.alphavantage.co/query"
        self.default_interval = interval

    def get_historical_data(self, ticker, days=30, interval=None):
        """
        Get historical intraday stock data for a given ticker using Alpha Vantage API

        Parameters:
        ticker (str): Stock ticker symbol
        days (int): Number of days of historical data to retrieve
        interval (str): Interval for intraday data (1min, 5min, 15min, 30min, 60min)
                       If None, uses default interval from initialization

        Returns:
        pandas.DataFrame: Historical data with OHLCV columns
        """
        # Use provided interval or fall back to default
        used_interval = interval or self.default_interval

        # Convert UI-friendly interval format to Alpha Vantage format if needed
        interval_mapping = {
            '1M': '1min',
            '5M': '5min',
            '15M': '15min',
            '30M': '30min',
            '1H': '60min',
            '2H': '120min',  # Alpha Vantage doesn't support this directly
            '4H': '240min',  # Alpha Vantage doesn't support this directly
            '1D': 'daily'
        }

        # Check if the interval is in our UI-friendly format and convert if needed
        if used_interval in interval_mapping:
            used_interval = interval_mapping[used_interval]

        # Handle special cases for intervals Alpha Vantage doesn't directly support
        if used_interval in ['120min', '240min']:
            # For these intervals, we'll need to get smaller intervals and aggregate
            print(f"Note: Alpha Vantage doesn't directly support {used_interval}. Using 60min data and will aggregate.")
            used_interval = '60min'
            need_aggregation = True
        else:
            need_aggregation = False

        # Define API parameters
        function = 'TIME_SERIES_INTRADAY' if used_interval != 'daily' else 'TIME_SERIES_DAILY'

        params = {
            'function': function,
            'symbol': ticker,
            'outputsize': 'full',  # Get full data
            'apikey': self.api_key,
            'datatype': 'json',
            'adjusted': 'true',
            'extended_hours': 'false',
        }

        # Add interval only for intraday data
        if function == 'TIME_SERIES_INTRADAY':
            params['interval'] = used_interval
            # For recent month data - adjust as needed
            params['month'] = datetime.now().strftime('%Y-%m')

        try:
            # Make request to Alpha Vantage API
            print(f"Requesting data from Alpha Vantage for {ticker} with interval {used_interval}")
            response = requests.get(self.base_url, params=params)
            response.raise_for_status()
            data = response.json()

            # Check if there's an API error
            if 'Error Message' in data:
                raise ValueError(f"Alpha Vantage API error: {data['Error Message']}")

            # Extract time series data
            time_series_key = f"Time Series ({used_interval})" if function == 'TIME_SERIES_INTRADAY' else "Time Series (Daily)"
            if time_series_key not in data:
                raise ValueError(f"No time series data found. Available keys: {data.keys()}")

            time_series = data[time_series_key]

            # Convert to DataFrame
            df = pd.DataFrame(time_series).T

            # Rename columns
            df.columns = [col.split('. ')[1] for col in df.columns]
            column_mapping = {
                'open': 'open',
                'high': 'high',
                'low': 'low',
                'close': 'close',
                'volume': 'volume'
            }
            df = df.rename(columns=column_mapping)

            # Convert types
            for col in ['open', 'high', 'low', 'close']:
                df[col] = pd.to_numeric(df[col])
            df['volume'] = pd.to_numeric(df['volume'], downcast='integer')

            # Convert index to datetime and sort
            df.index = pd.to_datetime(df.index)
            df = df.sort_index()

            # Filter based on requested days
            start_date = datetime.now() - timedelta(days=days)
            df = df[df.index >= start_date]

            # Group by day to get end-of-day data if needed
            # For Red Candle Theory, we need to identify the opening candle of each day
            df['date'] = df.index.date

            # Create 'is_first_candle' column to identify the opening candle of each day
            df['is_first_candle'] = False
            for date in df['date'].unique():
                day_mask = df['date'] == date
                if day_mask.any():
                    first_idx = df.index[day_mask][0]
                    df.loc[first_idx, 'is_first_candle'] = True

            # If we need to aggregate to 2H or 4H intervals
            if need_aggregation and interval in ['120min', '240min']:
                # Calculate how many 60min bars to aggregate
                bars_per_group = 2 if interval == '120min' else 4

                # Implement resampling
                # This is a simplified approach - a more robust implementation would handle
                # incomplete groups at day boundaries, etc.
                print(f"Aggregating 60min data to {interval}")

                # Reset index to make datetime accessible as a column
                df = df.reset_index()

                # Group by date and then by groups of bars_per_group
                df['group'] = df.groupby('date').cumcount() // bars_per_group

                # Aggregate within each date and group
                aggregated = df.groupby(['date', 'group']).agg({
                    'index': 'first',  # Use first timestamp
                    'open': 'first',   # Use first open
                    'high': 'max',     # Use highest high
                    'low': 'min',      # Use lowest low
                    'close': 'last',   # Use last close
                    'volume': 'sum',   # Sum the volume
                    'is_first_candle': 'first'  # Keep first candle flag
                }).reset_index()

                # Restore datetime index
                aggregated = aggregated.set_index('index')

                # Clean up
                del aggregated['group']

                df = aggregated

            return df

        except requests.exceptions.RequestException as e:
            print(f"Error fetching historical data: {e}")
            return None
        except Exception as e:
            print(f"Error processing Alpha Vantage data: {e}")
            return None

    # Rest of the class remains unchanged
    def get_options_data(self, ticker, days_to_expiration=30, strike_count=5):
        """
        Get options data for a given ticker using Alpha Vantage API

        Parameters:
        ticker (str): Stock ticker symbol
        days_to_expiration (int): Target number of days to expiration (approximate)
        strike_count (int): Not used directly with Alpha Vantage

        Returns:
        pandas.DataFrame: Options data
        """
        # Define API parameters for options chain
        params = {
            'function': 'OPTION_CHAIN',
            'symbol': ticker,
            'apikey': self.api_key
        }

        try:
            # Make request to Alpha Vantage API
            response = requests.get(self.base_url, params=params)
            response.raise_for_status()
            data = response.json()

            # Check if there's an API error
            if 'Error Message' in data:
                raise ValueError(f"Alpha Vantage API error: {data['Error Message']}")

            # Check if we have options data
            if 'options' not in data:
                raise ValueError(f"No options data found for {ticker}")

            # Extract options data
            options_data = []

            # Process each expiration date
            for exp_date in data['options']:
                expiration = exp_date.get('expiration_date')

                # Calculate days to expiration to filter
                exp_datetime = datetime.strptime(expiration, '%Y-%m-%d')
                current_datetime = datetime.now()
                dte = (exp_datetime - current_datetime).days

                # Filter by days to expiration (approximate match)
                if abs(dte - days_to_expiration) > 14:  # Allow up to 2 weeks difference
                    continue

                # Process calls
                for call in exp_date.get('calls', []):
                    call_data = {
                        'tradeDate': current_datetime.strftime('%Y-%m-%d'),
                        'expirDate': expiration,
                        'strike': float(call.get('strike_price')),
                        'stockPrice': float(data.get('underlying_price', 0)),
                        'option_type': 'CALL',
                        'callValue': float(call.get('last_price', 0)),
                        'putValue': 0,
                        'iv': float(call.get('implied_volatility', 0)) * 100,  # Convert to percentage
                        'delta': self._estimate_delta(call),  # Estimate delta from data
                        'gamma': 0  # Alpha Vantage doesn't provide gamma
                    }
                    options_data.append(call_data)

                # Process puts
                for put in exp_date.get('puts', []):
                    put_data = {
                        'tradeDate': current_datetime.strftime('%Y-%m-%d'),
                        'expirDate': expiration,
                        'strike': float(put.get('strike_price')),
                        'stockPrice': float(data.get('underlying_price', 0)),
                        'option_type': 'PUT',
                        'callValue': 0,
                        'putValue': float(put.get('last_price', 0)),
                        'iv': float(put.get('implied_volatility', 0)) * 100,  # Convert to percentage
                        'delta': -self._estimate_delta(put),  # Put deltas are negative
                        'gamma': 0  # Alpha Vantage doesn't provide gamma
                    }
                    options_data.append(put_data)

            # Convert to DataFrame
            df = pd.DataFrame(options_data)

            # If empty, notify user
            if df.empty:
                print(f"No options data available for {ticker} with expiration near {days_to_expiration} days")
                return None

            return df

        except requests.exceptions.RequestException as e:
            print(f"Error fetching options data: {e}")
            return None
        except Exception as e:
            print(f"Error processing Alpha Vantage options data: {e}")
            return None

    def _estimate_delta(self, option_data):
        """
        Estimate delta from option data when not provided directly by API

        Parameters:
        option_data (dict): Option data dictionary

        Returns:
        float: Estimated delta (0-1)
        """
        # If in-the-money, assume higher delta
        strike = float(option_data.get('strike_price', 0))
        stock_price = float(option_data.get('underlying_price', 0))
        days_to_expiration = (datetime.strptime(option_data.get('expiration_date', ''), '%Y-%m-%d') - datetime.now()).days

        # For calls
        if option_data.get('option_type') == 'call':
            if stock_price > strike:  # ITM call
                # Delta increases with deeper ITM and shorter expiration
                moneyness = (stock_price - strike) / strike
                delta = 0.5 + min(0.5, 0.5 * moneyness + 0.1 * (30 / max(days_to_expiration, 1)))
            else:  # OTM call
                # Delta decreases with deeper OTM and shorter expiration
                moneyness = (strike - stock_price) / stock_price
                delta = max(0.05, 0.5 - min(0.45, 0.5 * moneyness + 0.05 * (30 / max(days_to_expiration, 1))))
        # For puts
        else:
            if stock_price < strike:  # ITM put
                # Put delta is negative, increases with deeper ITM
                moneyness = (strike - stock_price) / strike
                delta = 0.5 + min(0.5, 0.5 * moneyness + 0.1 * (30 / max(days_to_expiration, 1)))
            else:  # OTM put
                # Put delta decreases with deeper OTM
                moneyness = (stock_price - strike) / strike
                delta = max(0.05, 0.5 - min(0.45, 0.5 * moneyness + 0.05 * (30 / max(days_to_expiration, 1))))

        # May need further refinement based on IV, but this is a reasonable first approximation
        return min(0.99, delta)

    def get_daily_data(self, ticker, days=30):
        """
        Get daily stock data for a given ticker using Alpha Vantage API

        Parameters:
        ticker (str): Stock ticker symbol
        days (int): Number of days of historical data to retrieve

        Returns:
        pandas.DataFrame: Daily historical data with OHLCV columns
        """
        # Define API parameters
        params = {
            'function': 'TIME_SERIES_DAILY',
            'symbol': ticker,
            'outputsize': 'full',  # Get full data
            'apikey': self.api_key,
            'datatype': 'json'
        }

        try:
            # Make request to Alpha Vantage API
            response = requests.get(self.base_url, params=params)
            response.raise_for_status()
            data = response.json()

            # Check if there's an API error
            if 'Error Message' in data:
                raise ValueError(f"Alpha Vantage API error: {data['Error Message']}")

            # Extract time series data
            if 'Time Series (Daily)' not in data:
                raise ValueError(f"No daily time series data found.")

            time_series = data['Time Series (Daily)']

            # Convert to DataFrame
            df = pd.DataFrame(time_series).T

            # Rename columns
            df.columns = [col.split('. ')[1] for col in df.columns]
            column_mapping = {
                'open': 'open',
                'high': 'high',
                'low': 'low',
                'close': 'close',
                'volume': 'volume'
            }
            df = df.rename(columns=column_mapping)

            # Convert types
            for col in ['open', 'high', 'low', 'close']:
                df[col] = pd.to_numeric(df[col])
            df['volume'] = pd.to_numeric(df['volume'], downcast='integer')

            # Convert index to datetime and sort
            df.index = pd.to_datetime(df.index)
            df = df.sort_index()

            # Filter based on requested days
            start_date = datetime.now() - timedelta(days=days)
            df = df[df.index >= start_date]

            return df

        except requests.exceptions.RequestException as e:
            print(f"Error fetching daily data: {e}")
            return None
        except Exception as e:
            print(f"Error processing Alpha Vantage data: {e}")
            return None