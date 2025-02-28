from abc import ABC, abstractmethod

class DataProvider(ABC):
    """Abstract base class for data providers"""

    @abstractmethod
    def get_historical_data(self, ticker, days=30, interval=None):
        """
        Get historical price data for a ticker

        Parameters:
        ticker (str): Stock ticker symbol
        days (int): Number of days of historical data to retrieve
        interval (str): Time interval for the data (e.g., '1min', '5min', '15min')

        Returns:
        pandas.DataFrame: Historical data with OHLCV columns
        """
        pass

    @abstractmethod
    def get_options_data(self, ticker, days_to_expiration=30, strike_count=5):
        """
        Get options data for a ticker

        Parameters:
        ticker (str): Stock ticker symbol
        days_to_expiration (int): Target number of days to expiration
        strike_count (int): Number of strikes above and below current price

        Returns:
        pandas.DataFrame: Options data
        """
        pass