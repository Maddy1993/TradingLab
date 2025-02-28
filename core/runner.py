from analysis.recommender import OptionsRecommender
from analysis.visualizer import Visualizer


class StrategyRunner:
    """Class for running a strategy with a data provider"""

    def __init__(self, data_provider, strategy, recommender=None, visualizer=None):
        """
        Initialize Strategy Runner

        Parameters:
        data_provider (DataProvider): Data provider instance
        strategy (Strategy): Strategy instance
        recommender (OptionsRecommender, optional): Options recommender
        visualizer (Visualizer, optional): Visualizer for results
        """
        self.data_provider = data_provider
        self.strategy = strategy
        self.recommender = recommender or OptionsRecommender()
        self.visualizer = visualizer or Visualizer()

    def run(self, ticker, days=30, visualize=True, save_recommendations=True, interval=None):
        """
        Run the strategy for a ticker

        Parameters:
        ticker (str): Stock ticker symbol
        days (int): Historical data days to analyze
        visualize (bool): Whether to visualize results
        save_recommendations (bool): Whether to save recommendations to file
        interval (str): Time interval for historical data (e.g., '15min')

        Returns:
        tuple: (DataFrame with signals, list of option recommendations)
        """
        # Step 1: Get historical data with specified interval
        df = self.data_provider.get_historical_data(ticker, days, interval)
        if df is None:
            return None, []

        # Step 2: Apply strategy to generate signals
        df = self.strategy.generate_signals(df)

        # Step 3: Get options data
        # options_df = self.data_provider.get_options_data(ticker)
        options_df = None

        # Step 4: Generate recommendations
        recommendations = []
        if options_df is not None and self.recommender:
            recommendations = self.recommender.generate_recommendations(df, options_df)

        # Step 5: Visualize results
        if visualize and self.visualizer:
            self.visualizer.plot_results(df, ticker)

        # Step 6: Save recommendations
        if save_recommendations and recommendations and self.visualizer:
            self.visualizer.save_recommendations(recommendations, f"{ticker}_recommendations.csv")

        return df, recommendations