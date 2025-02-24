from dotenv import load_dotenv

from analysis import OptionsRecommender, Visualizer
from core import StrategyRunner
from data import AlphaVantageDataProvider
from data import CachingDataProvider
from strategy import RedCandleStrategy

# Load environment variables (for API keys)
load_dotenv()

if __name__ == "__main__":
    # Create data provider
    base_provider = AlphaVantageDataProvider(interval="5min", api_key="2Q3G41GMYBD5ETKD")

    # Wrap with caching provider
    data_provider = CachingDataProvider(
            data_provider=base_provider,
            cache_dir="/Users/mopothu/Desktop/MyApps/trading_system/data_cache",
            cache_expiry_days=1  # Cache expires after 1 day
    )

    # Create strategy with Red Candle Theory
    # Setting use_additional_filters=True will apply RSI and volume filters
    strategy = RedCandleStrategy()

    # Create options recommender
    recommender = OptionsRecommender(
            min_delta=0.30,
            max_delta=0.60,
            target_delta=0.45
    )

    # Create visualizer
    visualizer = Visualizer()

    # Create strategy runner
    runner = StrategyRunner(
            data_provider=data_provider,
            strategy=strategy,
            recommender=recommender,
            visualizer=visualizer
    )

    # Define tickers to analyze
    # tickers = ['SPY', 'AAPL', 'MSFT', 'NVDA']

    # Or test with a single ticker
    tickers = ['SPY']

    for ticker in tickers:
        print(f"\nAnalyzing {ticker}...")

        # Run strategy (60 days of data)
        df, recommendations = runner.run(ticker, days=60, visualize=True, save_recommendations=False)

        if df is not None:
            # Print entry signals
            entry_signals = df[df['entry_signal']]
            if not entry_signals.empty:
                print(f"Found {len(entry_signals)} entry signals for {ticker}:")
                for date, row in entry_signals.iterrows():
                    print(f"  {date.strftime('%Y-%m-%d %H:%M:%S')}: Signal={row['signal_type']}, Close=${row['close']:.2f}, Stoploss=${row['stoploss']:.2f}")
            else:
                print(f"No entry signals found for {ticker}")

            # Print recommendations
            if recommendations:
                print(f"\nOptions Recommendations for {ticker}:")
                for rec in recommendations:
                    print(
                        f"  {rec['date']} - {rec['option_type']} @ ${rec['strike']} exp {rec['expiration']}")
                    print(
                        f"    Signal: {rec['signal_type']}, Stock: ${rec['stock_price']:.2f}, Stoploss: ${rec['stoploss']:.2f}")
                    if rec['price']:
                        print(f"    Option price: ${rec['price']:.2f}, Delta: {rec['delta']:.2f}")
            else:
                print(f"No options recommendations for {ticker}")
