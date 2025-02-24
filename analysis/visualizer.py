import pandas as pd
import numpy as np
import matplotlib.pyplot as plt
import matplotlib.patches as patches
from matplotlib.widgets import Slider, Button
import matplotlib.dates as mdates

class Visualizer:
    """Class for visualizing Red Candle Theory strategy results with improved symbol positioning and dragging"""

    @staticmethod
    def plot_results(df, ticker, block=True, save_fig=False, output_path=None):
        """
        Plot the Red Candle Theory strategy results with entry signals - price chart only

        Parameters:
        df (pandas.DataFrame): DataFrame with signals
        ticker (str): Stock ticker symbol
        block (bool): Whether to block execution until the plot window is closed
        save_fig (bool): Whether to save the figure to a file
        output_path (str): Path to save the figure (if save_fig is True)
        """
        # Ensure we have a datetime index for proper date handling
        if not isinstance(df.index, pd.DatetimeIndex):
            if 'date' in df.columns:
                df = df.set_index('date')
            else:
                # Try to convert the index to datetime
                try:
                    df.index = pd.to_datetime(df.index)
                except:
                    pass

        # Create figure with only price chart
        fig, ax = plt.subplots(figsize=(14, 8))

        # Store the full dataframe and settings for panning/zooming
        fig.full_df = df.copy()
        fig.ticker = ticker
        fig.visible_df = df.copy()  # Initialize with full data

        # Explicitly initialize dragging state variables
        fig.is_dragging = False
        fig.drag_start_x = None

        # Set matplotlib to interactive mode to improve responsiveness
        plt.ion()

        # Plot price chart
        Visualizer._plot_price_chart(ax, df, ticker)

        # Add zoom and pan controls
        Visualizer._add_interactive_controls(fig, ax)

        # Set window title
        fig.canvas.manager.set_window_title(f"{ticker} - Red Candle Theory Analysis")

        # Try to maximize window
        try:
            fig.canvas.manager.window.showMaximized()
        except:
            try:
                fig.canvas.manager.frame.Maximize(True)
            except:
                try:
                    fig.canvas.manager.window.state('zoomed')
                except:
                    pass

        # Add toolbar info
        plt.figtext(0.01, 0.01, "Drag chart to pan, use +/- buttons to zoom, or use the navigation toolbar",
                    fontsize=9, color='gray')

        plt.tight_layout()

        # Save figure if requested
        if save_fig and output_path:
            plt.savefig(output_path, dpi=300, bbox_inches='tight')
            print(f"Figure saved to {output_path}")

        # Show the plot and block execution until window is closed if block=True
        if block:
            plt.ioff()  # Turn off interactive mode for blocking
            plt.show()
        else:
            plt.show(block=False)
            return fig

    @staticmethod
    def _plot_price_chart(ax, df, ticker):
        """Plot price chart with Red Candle Theory markers"""
        ax.clear()  # Clear previous plot if any

        # Get price range for better symbol positioning
        y_min = df['low'].min()
        y_max = df['high'].max()
        y_range = y_max - y_min

        # Add a small padding to y-axis limits for marker visibility
        padding = y_range * 0.02
        y_axis_min = y_min - padding
        y_axis_max = y_max + padding

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

        # Create date labels with appropriate format based on data density
        if len(df) > 50:  # If many data points, use a more compact format
            date_labels = [d.strftime('%m/%d %H:%M') if hasattr(d, 'strftime') else str(d) for d in df.index]
        else:
            date_labels = [d.strftime('%Y-%m-%d %H:%M') if hasattr(d, 'strftime') else str(d) for d in df.index]

        # If too many labels, reduce them
        if len(df) > 100:
            show_every = max(1, len(df) // 30)  # Show approximately 30 labels
            date_labels = [label if i % show_every == 0 else '' for i, label in enumerate(date_labels)]

        ax.set_xticklabels(date_labels, rotation=45, ha='right')
        ax.set_xlim(-1, len(df))

        # Set y-axis limits with padding
        ax.set_ylim(y_axis_min, y_axis_max)

        # Plot markers for strategy elements with improved positioning
        Visualizer._plot_strategy_markers(ax, df, y_min, y_max, y_range)

        ax.set_title(f'{ticker} Price Chart with Red Candle Theory Signals')
        ax.set_ylabel('Price')
        ax.grid(True, alpha=0.3)

        # Add legend with custom markers for signals
        from matplotlib.lines import Line2D
        legend_elements = [
            Line2D([0], [0], marker='o', color='w', markerfacecolor='purple', markersize=8, label='Red Candle (R)'),
            Line2D([0], [0], marker='o', color='w', markerfacecolor='blue', markersize=8, label='Candle I'),
            Line2D([0], [0], marker='^', color='w', markerfacecolor='green', markersize=8, label='Long Signal'),
            Line2D([0], [0], marker='v', color='w', markerfacecolor='red', markersize=8, label='Short Signal')
        ]
        ax.legend(handles=legend_elements, loc='upper left')

    @staticmethod
    def _plot_strategy_markers(ax, df, y_min, y_max, y_range):
        """Plot markers for the Red Candle Theory strategy elements close to candlesticks"""
        # Calculate small offsets for positioning markers near candlesticks
        candle_offset = y_range * 0.01  # Small offset from candle high/low

        # Mark first candles of days
        if 'is_first_candle' in df.columns:
            first_candle_indices = df.index[df['is_first_candle']].tolist()
            for date in first_candle_indices:
                idx = df.index.get_loc(date)
                ax.axvline(x=idx, color='gray', linestyle='-', alpha=0.1)

        # Highlight first red candle (R)
        if 'first_red_candle' in df.columns:
            first_red_indices = df.index[df['first_red_candle']].tolist()
            for date in first_red_indices:
                idx = df.index.get_loc(date)
                # Position marker just above the high of this specific candle
                candle_high = df.loc[date, 'high']
                ax.plot(idx, candle_high + candle_offset, 'o', color='purple', markersize=8, alpha=0.7)

        # Highlight candle I
        if 'candle_i' in df.columns:
            candle_i_indices = df.index[df['candle_i']].tolist()
            for date in candle_i_indices:
                idx = df.index.get_loc(date)
                # Position marker just above the high of this specific candle
                candle_high = df.loc[date, 'high']
                ax.plot(idx, candle_high + candle_offset, 'o', color='blue', markersize=8, alpha=0.7)

                # Draw horizontal lines at candle I high and low for reference
                high_level = df.loc[date, 'high']
                low_level = df.loc[date, 'low']
                ax.axhline(y=high_level, color='blue', linestyle=':', linewidth=1, alpha=0.3)
                ax.axhline(y=low_level, color='blue', linestyle=':', linewidth=1, alpha=0.3)

        # Highlight long entries
        if 'long_entry' in df.columns:
            long_entry_indices = df.index[df['long_entry']].tolist()
            for date in long_entry_indices:
                idx = df.index.get_loc(date)
                # Position marker just above the high of this specific candle
                candle_high = df.loc[date, 'high']
                ax.plot(idx, candle_high + candle_offset, '^', color='green', markersize=10, alpha=0.9)

                # Draw stoploss level if available
                if 'stoploss' in df.columns and not pd.isna(df.loc[date, 'stoploss']):
                    stoploss = df.loc[date, 'stoploss']
                    ax.axhline(y=stoploss, xmin=(idx-0.3)/len(df), xmax=(idx+0.3)/len(df),
                               color='red', linestyle='-', linewidth=1, alpha=0.4)

        # Highlight short entries
        if 'short_entry' in df.columns:
            short_entry_indices = df.index[df['short_entry']].tolist()
            for date in short_entry_indices:
                idx = df.index.get_loc(date)
                # Position marker just below the low of this specific candle
                candle_low = df.loc[date, 'low']
                ax.plot(idx, candle_low - candle_offset, 'v', color='red', markersize=10, alpha=0.9)

                # Draw stoploss level if available
                if 'stoploss' in df.columns and not pd.isna(df.loc[date, 'stoploss']):
                    stoploss = df.loc[date, 'stoploss']
                    ax.axhline(y=stoploss, xmin=(idx-0.3)/len(df), xmax=(idx+0.3)/len(df),
                               color='green', linestyle='-', linewidth=1, alpha=0.4)

    @staticmethod
    def _add_interactive_controls(fig, ax):
        """Add zoom and pan controls to the figure"""
        # Add buttons for zooming in and out
        ax_zoom_in = plt.axes([0.95, 0.01, 0.04, 0.05])
        ax_zoom_out = plt.axes([0.90, 0.01, 0.04, 0.05])

        btn_zoom_in = Button(ax_zoom_in, '+')
        btn_zoom_out = Button(ax_zoom_out, '-')

        # Get original data
        full_df = fig.full_df
        ticker = fig.ticker

        # Keep track of view state
        fig.start_idx = 0
        fig.end_idx = len(full_df)
        fig.window_size = len(full_df)

        def zoom_in(event):
            # Reduce window size by 25%
            center_idx = (fig.start_idx + fig.end_idx) // 2
            new_window_size = max(20, int(fig.window_size * 0.75))  # Don't go below 20 candles

            # Calculate new start and end indices
            new_start_idx = max(0, center_idx - new_window_size // 2)
            new_end_idx = min(len(full_df), new_start_idx + new_window_size)

            # Adjust if we hit boundaries
            if new_end_idx == len(full_df):
                new_start_idx = max(0, new_end_idx - new_window_size)
            if new_start_idx == 0:
                new_end_idx = min(len(full_df), new_window_size)

            # Update view state
            fig.start_idx = new_start_idx
            fig.end_idx = new_end_idx
            fig.window_size = new_window_size

            # Update display
            update_view(fig, ax)

        def zoom_out(event):
            # Increase window size by 25%
            center_idx = (fig.start_idx + fig.end_idx) // 2
            new_window_size = min(len(full_df), int(fig.window_size * 1.25))

            # Calculate new start and end indices
            new_start_idx = max(0, center_idx - new_window_size // 2)
            new_end_idx = min(len(full_df), new_start_idx + new_window_size)

            # Adjust if we hit boundaries
            if new_end_idx == len(full_df):
                new_start_idx = max(0, new_end_idx - new_window_size)
            if new_start_idx == 0:
                new_end_idx = min(len(full_df), new_window_size)

            # Update view state
            fig.start_idx = new_start_idx
            fig.end_idx = new_end_idx
            fig.window_size = new_window_size

            # Update display
            update_view(fig, ax)

        def update_view(fig, ax):
            # Get visible slice of data
            visible_df = full_df.iloc[fig.start_idx:fig.end_idx].copy()
            fig.visible_df = visible_df

            # Replot with visible data
            Visualizer._plot_price_chart(ax, visible_df, ticker)
            fig.canvas.draw_idle()

        # Add handlers for buttons
        btn_zoom_in.on_clicked(zoom_in)
        btn_zoom_out.on_clicked(zoom_out)

        # Add keyboard shortcuts for zooming
        def on_key_press(event):
            if event.key == '+':
                zoom_in(event)
            elif event.key == '-':
                zoom_out(event)

        fig.canvas.mpl_connect('key_press_event', on_key_press)

        # Store buttons to prevent garbage collection
        fig.btn_zoom_in = btn_zoom_in
        fig.btn_zoom_out = btn_zoom_out

        # Add dragging/panning capabilities - improved version
        def on_press(event):
            if event.inaxes == ax:  # Click in main axes
                fig.is_dragging = True
                fig.drag_start_x = event.xdata

        def on_release(event):
            fig.is_dragging = False

        def on_motion(event):
            if fig.is_dragging and event.inaxes == ax and hasattr(fig, 'drag_start_x'):
                # Calculate how many bars to shift
                dx = int(fig.drag_start_x - event.xdata)
                if abs(dx) < 1:  # Ignore very small movements
                    return

                # Calculate new indices with boundary checks
                new_start_idx = max(0, fig.start_idx + dx)
                new_end_idx = min(len(full_df), new_start_idx + fig.window_size)

                # Adjust start if needed
                if new_end_idx == len(full_df):
                    new_start_idx = max(0, new_end_idx - fig.window_size)

                # Only update if there's a change
                if new_start_idx != fig.start_idx:
                    fig.start_idx = new_start_idx
                    fig.end_idx = new_end_idx
                    update_view(fig, ax)
                    fig.drag_start_x = event.xdata  # Update starting point for next movement

        fig.canvas.mpl_connect('button_press_event', on_press)
        fig.canvas.mpl_connect('button_release_event', on_release)
        fig.canvas.mpl_connect('motion_notify_event', on_motion)

        # Store update_view function for later use
        fig.update_view = update_view

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