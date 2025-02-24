from abc import ABC, abstractmethod


class Strategy(ABC):
    """Abstract base class for trading strategies"""

    @abstractmethod
    def generate_signals(self, data):
        """
        Generate trading signals from data

        Parameters:
        data (pandas.DataFrame): Historical price data

        Returns:
        pandas.DataFrame: Data with signals added
        """
        pass