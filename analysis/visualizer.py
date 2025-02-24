import pandas as pd
import matplotlib.pyplot as plt
import matplotlib.patches as patches
import numpy as np

class Visualizer:
    """Class for visualizing Red Candle Theory strategy results"""

    @staticmethod
    def plot_results(df, ticker):
        """
        Plot the Red Candle Theory strategy results with entry signals
        
        Parameters:
        df (pandas.DataFrame): DataFrame with signals
        ticker (str): Stock ticker symbol
        """
        # Create figure and subplots
        fig, (ax1, ax2, ax3) = plt.subplots(3, 1, figsize=(14, 10), gridspec_kw={'height_ratios': [3, 1, 1]})

        # Plot price chart
        Visualizer._plot_price_chart(ax1, df, ticker)

        # Plot RSI if available
        if 'rsi' in df.columns:
            Visualizer._plot_rsi(ax2, df)

        # Plot volume
        Visualizer._plot_volume(ax3, df)

        plt.tight_layout()
        # Use plt.draw() and plt.pause() instead of plt.show() to avoid blocking
        plt.draw()
        plt.pause(0.001)  # Small pause to render the plot

    @staticmethod
    def _plot_price_chart(ax, df, ticker):
        """Plot price chart with Red Candle Theory markers"""
        # Plot price
        ax.plot(df.index, df['close'], color='black', alpha=0.2, label='Close Price')

        # Plot candlesticks
        for i, (idx, row) in enumerate(df.iterrows()):
            # Candle body
            color = 'red' if row['close'] < row['open'] else 'green'
            bottom = min(row['open'], row['close'])
            height = abs(row['close'] - row['open'])
            rect = patches.Rectangle((i-0.4, bottom), 0.8, height, linewidth=1, color=color, alpha=0.8)
            ax.add_patch(rect)

            # Candle wicks
            ax.plot([i, i], [row['low'], min(row['open'], row['close'])], color='black', linewidth=1)
            ax.plot([i, i], [max(row['open'], row['close']), row['high']], color='black', linewidth=1)

        # Replace x-axis values with dates
        ax.set_xticks(range(len(df)))
        ax.set_xticklabels([d.strftime('%Y-%m-%d') for d in df.index], rotation=45)
        ax.set_xlim(-1, len(df))

        # Highlight first red candle (R)
        first_red_indices = df.index[df['first_red_candle']].tolist()
        for date in first_red_indices:
            idx = df.index.get_loc(date)
            ax.axvline(x=idx, color='purple', linestyle='--', alpha=0.7)
            ax.text(idx, df.iloc[idx]['high'] * 1.02, 'R', color='purple', fontweight='bold')

        # Highlight candle I
        candle_i_indices = df.index[df['candle_i']].tolist()
        for date in candle_i_indices:
            idx = df.index.get_loc(date)
            ax.axvline(x=idx, color='blue', linestyle='--', alpha=0.7)
            ax.text(idx, df.iloc[idx]['high'] * 1.02, 'I', color='blue', fontweight='bold')

        # Highlight long entries
        long_entry_indices = df.index[df['long_entry']].tolist()
        for date in long_entry_indices:
            idx = df.index.get_loc(date)
            ax.axvline(x=idx, color='green', linestyle='-', alpha=0.7)
            ax.text(idx, df.iloc[idx]['high'] * 1.02, 'LONG', color='green', fontweight='bold')

            # Draw stoploss level
            stoploss = df.loc[date, 'stoploss']
            ax.axhline(y=stoploss, xmin=(idx-0.5)/len(df), xmax=(idx+0.5)/len(df),
                       color='red', linestyle='-', linewidth=2, alpha=0.7)

        # Highlight short entries
        short_entry_indices = df.index[df['short_entry']].tolist()
        for date in short_entry_indices:
            idx = df.index.get_loc(date)
            ax.axvline(x=idx, color='red', linestyle='-', alpha=0.7)
            ax.text(idx, df.iloc[idx]['high'] * 1.02, 'SHORT', color='red', fontweight='bold')

            # Draw stoploss level
            stoploss = df.loc[date, 'stoploss']
            ax.axhline(y=stoploss, xmin=(idx-0.5)/len(df), xmax=(idx+0.5)/len(df),
                       color='green', linestyle='-', linewidth=2, alpha=0.7)

        ax.set_title(f'{ticker} Price Chart with Red Candle Theory Signals')
        ax.set_ylabel('Price')
        ax.grid(True, alpha=0.3)

    @staticmethod
    def _plot_rsi(ax, df):
        """Plot RSI indicator"""
        ax.plot(df.index, df['rsi'], color='blue', label='RSI')
        ax.axhline(y=30, color='green', linestyle='-', alpha=0.3)
        ax.axhline(y=70, color='red', linestyle='-', alpha=0.3)

        # Replace x-axis values with dates
        ax.set_xticks(range(len(df)))
        ax.set_xticklabels([d.strftime('%Y-%m-%d') for d in df.index], rotation=45)
        ax.set_xlim(-1, len(df))

        ax.set_title('RSI Indicator')
        ax.set_ylabel('RSI')
        ax.grid(True, alpha=0.3)

    @staticmethod
    def _plot_volume(ax, df):
        """Plot volume"""
        colors = ['red' if c < o else 'green' for c, o in zip(df['close'], df['open'])]

        # Use index positions for x-axis
        x_pos = np.arange(len(df))
        ax.bar(x_pos, df['volume'], color=colors, alpha=0.7, label='Volume')

        # Replace x-axis values with dates
        ax.set_xticks(range(len(df)))
        ax.set_xticklabels([d.strftime('%Y-%m-%d') for d in df.index], rotation=45)
        ax.set_xlim(-1, len(df))

        ax.set_title('Volume')
        ax.set_ylabel('Volume')
        ax.grid(True, alpha=0.3)

    @staticmethod
    def save_recommendations(recommendations, filename='recommendations.csv'):
        """
        Save recommendations to a CSV file
        
        Parameters:
        recommendations (list): List of recommendations
        filename (str): Output filename
        """
        if recommendations:
            df = pd.DataFrame(recommendations)
            df.to_csv(filename, index=False)
            print(f"Recommendations saved to {filename}")
        else:
            print("No recommendations to save.")