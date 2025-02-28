import os
import pandas as pd
import pickle
from datetime import datetime, timedelta
from pathlib import Path

class CachingDataProvider:
    """
    Wrapper for a data provider that caches results to avoid repeated API calls
    """

    def __init__(self, data_provider, cache_dir="./data_cache", cache_expiry_days=1):
        """
        Initialize the caching data provider

        Parameters:
        data_provider: The actual data provider to wrap
        cache_dir (str): Directory to store cached data
        cache_expiry_days (int): Number of days before cached data expires
        """
        self.data_provider = data_provider
        self.cache_dir = Path(cache_dir)
        self.cache_expiry_days = cache_expiry_days

        # Create cache directory if it doesn't exist
        os.makedirs(self.cache_dir, exist_ok=True)

        # Create subdirectories for different data types
        self.historical_cache_dir = self.cache_dir / "historical"
        self.options_cache_dir = self.cache_dir / "options"

        os.makedirs(self.historical_cache_dir, exist_ok=True)
        os.makedirs(self.options_cache_dir, exist_ok=True)

    def get_historical_data(self, ticker, days=30, interval='15min'):
        """
        Get historical data with caching

        Parameters:
        ticker (str): Stock ticker symbol
        days (int): Number of days of historical data to retrieve
        interval (str): Time interval for candles (e.g., '1min', '5min', '15min')

        Returns:
        pandas.DataFrame: Historical price data
        """
        # Create a cache key based on ticker, days, and interval
        cache_key = f"{ticker}_{days}_{interval}"
        cache_file = self.historical_cache_dir / f"{cache_key}.pkl"

        # Check if cached data exists and is still valid
        if self._is_cache_valid(cache_file):
            print(f"Using cached historical data for {ticker} with interval {interval}")
            return self._load_from_cache(cache_file)

        # If no valid cache, get data from the provider
        print(f"Fetching fresh historical data for {ticker} with interval {interval}")
        data = self.data_provider.get_historical_data(ticker, days, interval)

        # Cache the result if we got valid data
        if data is not None:
            self._save_to_cache(data, cache_file)

        return data

    def get_options_data(self, ticker, days_to_expiration=30, strike_count=5):
        """
        Get options data with caching
        """
        # Create a cache key based on parameters
        cache_key = f"{ticker}_{days_to_expiration}_{strike_count}"
        cache_file = self.options_cache_dir / f"{cache_key}.pkl"

        # Check if cached data exists and is still valid
        if self._is_cache_valid(cache_file):
            print(f"Using cached options data for {ticker}")
            return self._load_from_cache(cache_file)

        # If no valid cache, get data from the provider
        print(f"Fetching fresh options data for {ticker}")
        data = self.data_provider.get_options_data(ticker, days_to_expiration, strike_count)

        # Cache the result if we got valid data
        if data is not None:
            self._save_to_cache(data, cache_file)

        return data

    def _is_cache_valid(self, cache_file):
        """Check if a cache file exists and is still valid"""
        if not cache_file.exists():
            return False

        # Check if file is too old
        file_mtime = datetime.fromtimestamp(cache_file.stat().st_mtime)
        cache_age = datetime.now() - file_mtime

        return cache_age.days < self.cache_expiry_days

    def _save_to_cache(self, data, cache_file):
        """Save data to cache file"""
        try:
            with open(cache_file, 'wb') as f:
                pickle.dump(data, f)
        except Exception as e:
            print(f"Error saving to cache: {e}")

    def _load_from_cache(self, cache_file):
        """Load data from cache file"""
        try:
            with open(cache_file, 'rb') as f:
                return pickle.load(f)
        except Exception as e:
            print(f"Error loading from cache: {e}")
            return None

    # Add any other methods from your data provider that you need to cache
    def get_daily_data(self, ticker, days=30):
        """Get daily data with caching"""
        # Similar implementation as get_historical_data
        cache_key = f"{ticker}_{days}_daily"
        cache_file = self.historical_cache_dir / f"{cache_key}.pkl"

        if self._is_cache_valid(cache_file):
            print(f"Using cached daily data for {ticker}")
            return self._load_from_cache(cache_file)

        print(f"Fetching fresh daily data for {ticker}")
        data = self.data_provider.get_daily_data(ticker, days)

        if data is not None:
            self._save_to_cache(data, cache_file)

        return data