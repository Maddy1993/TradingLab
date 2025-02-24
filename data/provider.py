from abc import ABC, abstractmethod

class DataProvider(ABC):
    """Abstract base class for data providers"""

    @abstractmethod
    def get_historical_data(self, ticker, days=30):
        """Get historical price data for a ticker"""
        pass

    @abstractmethod
    def get_options_data(self, ticker, days_to_expiration=30, strike_count=5):
        """Get options data for a ticker"""
        pass