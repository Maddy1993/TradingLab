import requests
import pandas as pd
from datetime import datetime, timedelta
import os

from .provider import DataProvider

class ORATSDataProvider(DataProvider):
    """Data provider that fetches data from ORATS API"""

    def __init__(self, api_key=None):
        """
        Initialize the ORATS data provider

        Parameters:
        api_key (str): ORATS API key (if None, will try to get from environment variable)
        """
        self.api_key = api_key or os.getenv('ORATS_API_KEY')
        if not self.api_key:
            raise ValueError("ORATS API key is required. Provide it as a parameter or set ORATS_API_KEY environment variable.")

        self.base_url = "https://api.orats.io/v2"
        self.headers = {"Authorization": f"Bearer {self.api_key}"}

    def get_historical_data(self, ticker, days=30):
        """
        Get historical stock data for a given ticker using ORATS API

        Parameters:
        ticker (str): Stock ticker symbol
        days (int): Number of days of historical data to retrieve

        Returns:
        pandas.DataFrame: Historical data with OHLCV columns
        """
        end_date = datetime.now()
        start_date = end_date - timedelta(days=days)

        # Format dates for API request
        start_str = start_date.strftime('%Y-%m-%d')
        end_str = end_date.strftime('%Y-%m-%d')

        # Endpoint for historical stock data
        endpoint = f"{self.base_url}/history/stocks"

        params = {
            'ticker': ticker,
            'start': start_str,
            'end': end_str
        }

        try:
            response = requests.get(endpoint, headers=self.headers, params=params)
            response.raise_for_status()
            data = response.json()

            if 'data' not in data or len(data['data']) == 0:
                raise ValueError(f"No historical data found for {ticker}")

            # Convert to DataFrame
            df = pd.DataFrame(data['data'])

            # Ensure we have OHLCV columns
            required_columns = ['tradeDate', 'open', 'high', 'low', 'close', 'volume']
            if not all(col in df.columns for col in required_columns):
                raise ValueError(f"Missing required columns in data. Available columns: {df.columns.tolist()}")

            # Rename and convert columns
            df['date'] = pd.to_datetime(df['tradeDate'])
            df = df.set_index('date')
            df = df.sort_index()

            # Keep only necessary columns
            df = df[['open', 'high', 'low', 'close', 'volume']]

            return df

        except requests.exceptions.RequestException as e:
            print(f"Error fetching historical data: {e}")
            return None

    def get_options_data(self, ticker, days_to_expiration=30, strike_count=5):
        """
        Get options data for a given ticker using ORATS API

        Parameters:
        ticker (str): Stock ticker symbol
        days_to_expiration (int): Target number of days to expiration
        strike_count (int): Number of strikes above and below the current price

        Returns:
        pandas.DataFrame: Options data
        """
        # Endpoint for options data
        endpoint = f"{self.base_url}/strikes/summary"

        params = {
            'ticker': ticker,
            'minDaysToExpiration': max(1, days_to_expiration - 5),
            'maxDaysToExpiration': days_to_expiration + 5,
            'strikePct': strike_count / 10  # ORATS uses percentage for strike range
        }

        try:
            response = requests.get(endpoint, headers=self.headers, params=params)
            response.raise_for_status()
            data = response.json()

            if 'data' not in data or len(data['data']) == 0:
                raise ValueError(f"No options data found for {ticker}")

            # Convert to DataFrame
            df = pd.DataFrame(data['data'])

            # Keep only the most relevant columns for our strategy
            relevant_columns = [
                'tradeDate', 'expirDate', 'strike', 'delta', 'gamma',
                'ticker', 'stockPrice', 'iv', 'callValue', 'putValue'
            ]

            df = df[[col for col in relevant_columns if col in df.columns]]
            return df

        except requests.exceptions.RequestException as e:
            print(f"Error fetching options data: {e}")
            return None