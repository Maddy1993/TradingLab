import pandas as pd
import numpy as np
from datetime import datetime
import matplotlib.pyplot as plt
from collections import defaultdict


class StrategyBacktester:
    """
    Class for backtesting trading strategies with profit targets and stop losses
    """

    def __init__(self, strategy_name="Default Strategy"):
        """
        Initialize the strategy backtester

        Parameters:
        strategy_name (str): Name of the strategy being tested
        """
        self.strategy_name = strategy_name
        self.results = {}
        self.trades = []
        self.summary_stats = {}

    def backtest(self, df, profit_targets=None, risk_reward_ratios=None,
                 profit_targets_dollar=None, commission=0.0, slippage=0.0,
                 position_size=1.0, contracts=1, contract_value=100):
        """
        Run a backtest on a DataFrame with entry signals

        Parameters:
        df (pandas.DataFrame): DataFrame with entry signals and stoploss values
        profit_targets (list): List of profit target percentages (e.g. [5, 10, 15])
        risk_reward_ratios (list): List of risk-reward ratios to test (e.g. [1, 2, 3])
        profit_targets_dollar (list): List of profit targets in dollar amounts (e.g. [100, 200, 500])
        commission (float): Commission per trade in percentage
        slippage (float): Assumed slippage per trade in percentage
        position_size (float): Position size as a percentage of account
        contracts (int): Number of contracts per trade
        contract_value (float): Dollar value per point/tick movement per contract

        Returns:
        dict: Dictionary with backtest results
        """
        # Make a copy to avoid modifying the original DataFrame
        df = df.copy()

        # Ensure required columns exist
        required_columns = ['entry_signal', 'signal_type', 'stoploss']
        missing_columns = [col for col in required_columns if col not in df.columns]
        if missing_columns:
            raise ValueError(f"Missing required columns: {missing_columns}")

        # Default profit targets if nothing is provided
        if profit_targets is None and risk_reward_ratios is None and profit_targets_dollar is None:
            profit_targets = [5, 10, 15]  # Default profit targets in percentage

        # Initialize results for each profit target or risk-reward ratio
        test_results = {}

        # Process based on profit targets (percentage)
        if profit_targets is not None:
            for target in profit_targets:
                results = self._process_profit_target(df, target, commission, slippage, position_size, contracts, contract_value)
                test_results[f"Target_{target}%"] = results

        # Process based on risk-reward ratios
        if risk_reward_ratios is not None:
            for rr_ratio in risk_reward_ratios:
                results = self._process_risk_reward(df, rr_ratio, commission, slippage, position_size, contracts, contract_value)
                test_results[f"RR_1:{rr_ratio}"] = results

        # Process based on dollar profit targets
        if profit_targets_dollar is not None:
            for dollar_target in profit_targets_dollar:
                results = self._process_dollar_target(df, dollar_target, commission, slippage, position_size, contracts, contract_value)
                test_results[f"${dollar_target}"] = results

        # Store and return results
        self.results = test_results
        return test_results

        # Process based on dollar profit targets
        if profit_targets_dollar is not None:
            for dollar_target in profit_targets_dollar:
                results = self._process_dollar_target(df, dollar_target, commission, slippage, position_size, contracts, contract_value)
                test_results[f"${dollar_target}"] = results

        # Store and return results
        self.results = test_results
        return test_results

    def _process_profit_target(self, df, profit_target, commission, slippage, position_size, contracts, contract_value):
        """
        Process backtest with fixed profit target percentage

        Parameters:
        df (pandas.DataFrame): DataFrame with signals
        profit_target (float): Profit target percentage
        commission (float): Commission percentage
        slippage (float): Slippage percentage
        position_size (float): Position size percentage
        contracts (int): Number of contracts per trade
        contract_value (float): Dollar value per point/tick movement per contract

        Returns:
        dict: Results for this profit target
        """
        # Initialize results
        results = {
            'profit_target': profit_target,
            'total_trades': 0,
            'winning_trades': 0,
            'losing_trades': 0,
            'winning_trades_dollar': 0,
            'losing_trades_dollar': 0,
            'win_rate': 0.0,
            'win_rate_dollar': 0.0,
            'avg_profit': 0.0,
            'avg_loss': 0.0,
            'profit_factor': 0.0,
            'total_return': 0.0,
            'total_return_pct': 0.0,
            'max_drawdown': 0.0,
            'max_drawdown_pct': 0.0,
            'trades': []
        }

        # Find all entry signals
        entry_signals = df[df['entry_signal']].copy()
        results['total_trades'] = len(entry_signals)

        if results['total_trades'] == 0:
            return results

        # Process each trade
        trades = []
        total_profit = 0.0
        total_win_profit = 0.0
        total_loss = 0.0

        for entry_date, entry_row in entry_signals.iterrows():
            # Skip if no clear signal type or stoploss
            if pd.isna(entry_row['signal_type']) or pd.isna(entry_row['stoploss']):
                continue

            signal_type = entry_row['signal_type']
            entry_price = entry_row['close']
            stop_loss = entry_row['stoploss']

            # Calculate profit target based on entry price and signal type
            if signal_type == 'LONG':
                profit_target_price = entry_price * (1 + profit_target / 100)
                risk_amount = entry_price - stop_loss
            else:  # SHORT
                profit_target_price = entry_price * (1 - profit_target / 100)
                risk_amount = stop_loss - entry_price

            # Look for exit in future data
            exit_date = None
            exit_price = None
            exit_type = None
            max_excursion = 0.0  # Maximum adverse excursion
            max_favorable = 0.0  # Maximum favorable excursion

            # Find the next index position after entry_date
            if entry_date in df.index:
                entry_idx = df.index.get_loc(entry_date)

                # Look at future bars to determine outcome
                for i in range(entry_idx + 1, len(df)):
                    future_date = df.index[i]
                    future_row = df.iloc[i]

                    # For LONG positions
                    if signal_type == 'LONG':
                        # Check for stop loss hit (low price touches or goes below stop)
                        if future_row['low'] <= stop_loss:
                            exit_date = future_date
                            exit_price = stop_loss  # Assume stopped out at stop price
                            exit_type = 'STOP'
                            break

                        # Check for profit target hit (high price touches or exceeds target)
                        if future_row['high'] >= profit_target_price:
                            exit_date = future_date
                            exit_price = profit_target_price  # Assume filled at target
                            exit_type = 'TARGET'
                            break

                        # Update maximum excursions
                        if future_row['low'] < entry_price:
                            current_excursion = (entry_price - future_row['low']) / entry_price * 100
                            max_excursion = max(max_excursion, current_excursion)

                        if future_row['high'] > entry_price:
                            current_favorable = (future_row['high'] - entry_price) / entry_price * 100
                            max_favorable = max(max_favorable, current_favorable)

                    # For SHORT positions
                    else:
                        # Check for stop loss hit (high price touches or exceeds stop)
                        if future_row['high'] >= stop_loss:
                            exit_date = future_date
                            exit_price = stop_loss  # Assume stopped out at stop price
                            exit_type = 'STOP'
                            break

                        # Check for profit target hit (low price touches or goes below target)
                        if future_row['low'] <= profit_target_price:
                            exit_date = future_date
                            exit_price = profit_target_price  # Assume filled at target
                            exit_type = 'TARGET'
                            break

                        # Update maximum excursions
                        if future_row['high'] > entry_price:
                            current_excursion = (future_row['high'] - entry_price) / entry_price * 100
                            max_excursion = max(max_excursion, current_excursion)

                        if future_row['low'] < entry_price:
                            current_favorable = (entry_price - future_row['low']) / entry_price * 100
                            max_favorable = max(max_favorable, current_favorable)

            # If we didn't find an exit, assume the trade is still open at the end of the data
            if exit_date is None:
                exit_date = df.index[-1]
                exit_price = df.iloc[-1]['close']
                exit_type = 'OPEN'

            # Calculate profit/loss
            if signal_type == 'LONG':
                trade_pl_pct = ((exit_price - entry_price) / entry_price * 100) - commission - slippage
            else:  # SHORT
                trade_pl_pct = ((entry_price - exit_price) / entry_price * 100) - commission - slippage

            # Calculate dollar value of profit/loss
            price_change = abs(exit_price - entry_price)
            dollar_value_per_point = contract_value
            dollar_value_per_contract = price_change * dollar_value_per_point
            total_dollar_value = dollar_value_per_contract * contracts

            # Determine profit/loss direction
            if (signal_type == 'LONG' and exit_price > entry_price) or (signal_type == 'SHORT' and exit_price < entry_price):
                dollar_pl = total_dollar_value
            else:
                dollar_pl = -total_dollar_value

            # Record trade details
            trade = {
                'entry_date': entry_date,
                'exit_date': exit_date,
                'signal_type': signal_type,
                'entry_price': entry_price,
                'exit_price': exit_price,
                'stop_loss': stop_loss,
                'profit_target': profit_target_price,
                'exit_type': exit_type,
                'profit_loss_pct': trade_pl_pct,
                'profit_loss_amount': trade_pl_pct * position_size / 100,
                'profit_loss_dollar': dollar_pl,
                'contracts': contracts,
                'contract_value': contract_value,
                'max_adverse_excursion': max_excursion,
                'max_favorable_excursion': max_favorable,
                'hold_time': (exit_date - entry_date).total_seconds() / 86400  # Days
            }

            trades.append(trade)

            # Update statistics
            total_profit += trade['profit_loss_amount']

            if trade_pl_pct > 0:
                results['winning_trades'] += 1
                total_win_profit += trade['profit_loss_amount']
            else:
                results['losing_trades'] += 1
                total_loss += abs(trade['profit_loss_amount'])

            # Track dollar wins/losses separately
            if dollar_pl > 0:
                results['winning_trades_dollar'] = results.get('winning_trades_dollar', 0) + 1
            else:
                results['losing_trades_dollar'] = results.get('losing_trades_dollar', 0) + 1

        # Calculate max drawdown
        max_drawdown = 0.0
        max_drawdown_pct = 0.0
        peak_equity = 0.0
        current_equity = 0.0

        # Sort trades by date for equity curve calculation
        sorted_trades = sorted(trades, key=lambda x: x['entry_date'])

        # Calculate equity curve and max drawdown
        for trade in sorted_trades:
            current_equity += trade['profit_loss_dollar']
            peak_equity = max(peak_equity, current_equity)
            drawdown = peak_equity - current_equity
            drawdown_pct = drawdown / peak_equity if peak_equity > 0 else 0
            max_drawdown = max(max_drawdown, drawdown)
            max_drawdown_pct = max(max_drawdown_pct, drawdown_pct)

        # Calculate total dollar profit/loss
        total_dollar_profit = sum(trade['profit_loss_dollar'] for trade in trades if trade['profit_loss_dollar'] > 0)
        total_dollar_loss = sum(abs(trade['profit_loss_dollar']) for trade in trades if trade['profit_loss_dollar'] < 0)
        total_dollar_return = sum(trade['profit_loss_dollar'] for trade in trades)

        # Calculate max drawdown
        max_drawdown = 0.0
        max_drawdown_pct = 0.0
        peak_equity = 0.0
        current_equity = 0.0

        # Sort trades by date for equity curve calculation
        sorted_trades = sorted(trades, key=lambda x: x['entry_date'])

        # Calculate equity curve and max drawdown
        for trade in sorted_trades:
            current_equity += trade['profit_loss_dollar']
            peak_equity = max(peak_equity, current_equity)
            drawdown = peak_equity - current_equity
            drawdown_pct = drawdown / peak_equity if peak_equity > 0 else 0
            max_drawdown = max(max_drawdown, drawdown)
            max_drawdown_pct = max(max_drawdown_pct, drawdown_pct)

        # Calculate total dollar profit/loss
        total_dollar_profit = sum(trade['profit_loss_dollar'] for trade in trades if trade['profit_loss_dollar'] > 0)
        total_dollar_loss = sum(abs(trade['profit_loss_dollar']) for trade in trades if trade['profit_loss_dollar'] < 0)
        total_dollar_return = sum(trade['profit_loss_dollar'] for trade in trades)

        # Update results
        results['trades'] = trades
        results['win_rate'] = results['winning_trades'] / results['total_trades'] if results['total_trades'] > 0 else 0
        results['win_rate_dollar'] = results['winning_trades_dollar'] / results['total_trades'] if results['total_trades'] > 0 else 0
        results['avg_profit'] = total_win_profit / results['winning_trades'] if results['winning_trades'] > 0 else 0
        results['avg_loss'] = total_loss / results['losing_trades'] if results['losing_trades'] > 0 else 0
        results['profit_factor'] = total_win_profit / total_loss if total_loss > 0 else float('inf')
        results['total_return'] = total_profit
        results['total_return_pct'] = (results['total_return'] * 100) if position_size > 0 else 0
        results['total_dollar_return'] = total_dollar_return
        results['total_dollar_profit'] = total_dollar_profit
        results['total_dollar_loss'] = total_dollar_loss
        results['avg_dollar_profit'] = total_dollar_profit / results['winning_trades_dollar'] if results['winning_trades_dollar'] > 0 else 0
        results['avg_dollar_loss'] = total_dollar_loss / results['losing_trades_dollar'] if results['losing_trades_dollar'] > 0 else 0
        results['profit_factor_dollar'] = total_dollar_profit / total_dollar_loss if total_dollar_loss > 0 else float('inf')
        results['max_drawdown'] = max_drawdown
        results['max_drawdown_pct'] = max_drawdown_pct * 100  # Convert to percentage
        results['total_return_pct'] = (results['total_return'] * 100) if position_size > 0 else 0
        results['total_dollar_return'] = total_dollar_return
        results['total_dollar_profit'] = total_dollar_profit
        results['total_dollar_loss'] = total_dollar_loss
        results['max_drawdown'] = max_drawdown
        results['max_drawdown_pct'] = max_drawdown_pct * 100  # Convert to percentage

        # Store trades for later analysis
        self.trades.extend(trades)

        return results

    def _process_risk_reward(self, df, rr_ratio, commission, slippage, position_size, contracts, contract_value):
        """
        Process backtest with fixed risk-reward ratio

        Parameters:
        df (pandas.DataFrame): DataFrame with signals
        rr_ratio (float): Risk-reward ratio (e.g., 2 means profit target is 2x the risk)
        commission (float): Commission percentage
        slippage (float): Slippage percentage
        position_size (float): Position size percentage
        contracts (int): Number of contracts per trade
        contract_value (float): Dollar value per point/tick movement per contract

        Returns:
        dict: Results for this risk-reward ratio
        """
        # Initialize results
        results = {
            'risk_reward_ratio': rr_ratio,
            'total_trades': 0,
            'winning_trades': 0,
            'losing_trades': 0,
            'winning_trades_dollar': 0,
            'losing_trades_dollar': 0,
            'win_rate': 0.0,
            'win_rate_dollar': 0.0,
            'avg_profit': 0.0,
            'avg_loss': 0.0,
            'profit_factor': 0.0,
            'total_return': 0.0,
            'total_return_pct': 0.0,
            'max_drawdown': 0.0,
            'max_drawdown_pct': 0.0,
            'trades': []
        }

        # Find all entry signals
        entry_signals = df[df['entry_signal']].copy()
        results['total_trades'] = len(entry_signals)

        if results['total_trades'] == 0:
            return results

        # Process each trade
        trades = []
        total_profit = 0.0
        total_win_profit = 0.0
        total_loss = 0.0

        for entry_date, entry_row in entry_signals.iterrows():
            # Skip if no clear signal type or stoploss
            if pd.isna(entry_row['signal_type']) or pd.isna(entry_row['stoploss']):
                continue

            signal_type = entry_row['signal_type']
            entry_price = entry_row['close']
            stop_loss = entry_row['stoploss']

            # Calculate risk and profit target based on risk-reward ratio
            if signal_type == 'LONG':
                risk_amount = entry_price - stop_loss
                profit_target_price = entry_price + (risk_amount * rr_ratio)
                profit_target_pct = (profit_target_price - entry_price) / entry_price * 100
            else:  # SHORT
                risk_amount = stop_loss - entry_price
                profit_target_price = entry_price - (risk_amount * rr_ratio)
                profit_target_pct = (entry_price - profit_target_price) / entry_price * 100

            # Look for exit in future data
            exit_date = None
            exit_price = None
            exit_type = None
            max_excursion = 0.0  # Maximum adverse excursion
            max_favorable = 0.0  # Maximum favorable excursion

            # Find the next index position after entry_date
            if entry_date in df.index:
                entry_idx = df.index.get_loc(entry_date)

                # Look at future bars to determine outcome
                for i in range(entry_idx + 1, len(df)):
                    future_date = df.index[i]
                    future_row = df.iloc[i]

                    # For LONG positions
                    if signal_type == 'LONG':
                        # Check for stop loss hit
                        if future_row['low'] <= stop_loss:
                            exit_date = future_date
                            exit_price = stop_loss
                            exit_type = 'STOP'
                            break

                        # Check for profit target hit
                        if future_row['high'] >= profit_target_price:
                            exit_date = future_date
                            exit_price = profit_target_price
                            exit_type = 'TARGET'
                            break

                        # Update maximum excursions
                        if future_row['low'] < entry_price:
                            current_excursion = (entry_price - future_row['low']) / entry_price * 100
                            max_excursion = max(max_excursion, current_excursion)

                        if future_row['high'] > entry_price:
                            current_favorable = (future_row['high'] - entry_price) / entry_price * 100
                            max_favorable = max(max_favorable, current_favorable)

                    # For SHORT positions
                    else:
                        # Check for stop loss hit
                        if future_row['high'] >= stop_loss:
                            exit_date = future_date
                            exit_price = stop_loss
                            exit_type = 'STOP'
                            break

                        # Check for profit target hit
                        if future_row['low'] <= profit_target_price:
                            exit_date = future_date
                            exit_price = profit_target_price
                            exit_type = 'TARGET'
                            break

                        # Update maximum excursions
                        if future_row['high'] > entry_price:
                            current_excursion = (future_row['high'] - entry_price) / entry_price * 100
                            max_excursion = max(max_excursion, current_excursion)

                        if future_row['low'] < entry_price:
                            current_favorable = (entry_price - future_row['low']) / entry_price * 100
                            max_favorable = max(max_favorable, current_favorable)

            # If we didn't find an exit, assume the trade is still open at the end of the data
            if exit_date is None:
                exit_date = df.index[-1]
                exit_price = df.iloc[-1]['close']
                exit_type = 'OPEN'

            # Calculate profit/loss
            if signal_type == 'LONG':
                trade_pl_pct = ((exit_price - entry_price) / entry_price * 100) - commission - slippage
            else:  # SHORT
                trade_pl_pct = ((entry_price - exit_price) / entry_price * 100) - commission - slippage

            # Calculate dollar value of profit/loss
            price_change = abs(exit_price - entry_price)
            dollar_value_per_contract = price_change * contract_value
            total_dollar_value = dollar_value_per_contract * contracts

            # Determine profit/loss direction
            if (signal_type == 'LONG' and exit_price > entry_price) or (signal_type == 'SHORT' and exit_price < entry_price):
                dollar_pl = total_dollar_value
            else:
                dollar_pl = -total_dollar_value

            # Record trade details
            trade = {
                'entry_date': entry_date,
                'exit_date': exit_date,
                'signal_type': signal_type,
                'entry_price': entry_price,
                'exit_price': exit_price,
                'stop_loss': stop_loss,
                'profit_target': profit_target_price,
                'profit_target_pct': profit_target_pct,
                'exit_type': exit_type,
                'profit_loss_pct': trade_pl_pct,
                'profit_loss_amount': trade_pl_pct * position_size / 100,
                'profit_loss_dollar': dollar_pl,
                'contracts': contracts,
                'contract_value': contract_value,
                'max_adverse_excursion': max_excursion,
                'max_favorable_excursion': max_favorable,
                'hold_time': (exit_date - entry_date).total_seconds() / 86400  # Days
            }

            trades.append(trade)

            # Update statistics
            total_profit += trade['profit_loss_amount']

            if trade_pl_pct > 0:
                results['winning_trades'] += 1
                total_win_profit += trade['profit_loss_amount']
            else:
                results['losing_trades'] += 1
                total_loss += abs(trade['profit_loss_amount'])

        # Update results
        results['trades'] = trades
        results['win_rate'] = results['winning_trades'] / results['total_trades'] if results['total_trades'] > 0 else 0
        results['avg_profit'] = total_win_profit / results['winning_trades'] if results['winning_trades'] > 0 else 0
        results['avg_loss'] = total_loss / results['losing_trades'] if results['losing_trades'] > 0 else 0
        results['profit_factor'] = total_win_profit / total_loss if total_loss > 0 else float('inf')
        results['total_return'] = total_profit

        # Store trades for later analysis
        self.trades.extend(trades)

        return results

    def get_summary_stats(self):
        """
        Calculate and return summary statistics across all tests

        Returns:
        dict: Dictionary of summary statistics
        """
        if not self.results:
            return {"error": "No backtest results available"}

        summary = {}

        # Aggregate results across all tests
        for test_name, test_results in self.results.items():
            summary[test_name] = {
                'win_rate': test_results['win_rate'] * 100,  # Convert to percentage
                'profit_factor': test_results['profit_factor'],
                'total_return': test_results['total_return'],
                'total_return_pct': test_results['total_return_pct'],
                'total_dollar_return': test_results.get('total_dollar_return', 0),
                'total_trades': test_results['total_trades'],
                'winning_trades': test_results['winning_trades'],
                'losing_trades': test_results['losing_trades'],
                'avg_profit': test_results['avg_profit'],
                'avg_loss': test_results['avg_loss'],
                'max_drawdown': test_results.get('max_drawdown', 0),
                'max_drawdown_pct': test_results.get('max_drawdown_pct', 0)
            }

        # Store summary stats
        self.summary_stats = summary
        return summary

    def plot_results(self, figsize=(15, 10)):
        """
        Plot backtest results

        Parameters:
        figsize (tuple): Figure size for the plot
        """
        if not self.results:
            print("No backtest results to plot")
            return

        # Get summary stats if not already calculated
        if not self.summary_stats:
            self.get_summary_stats()

        plt.figure(figsize=figsize)

        # Plot win rates
        plt.subplot(3, 2, 1)
        test_names = list(self.summary_stats.keys())
        win_rates = [stats['win_rate'] for stats in self.summary_stats.values()]

        plt.bar(test_names, win_rates)
        plt.title('Win Rate by Test')
        plt.ylabel('Win Rate (%)')
        plt.ylim(0, 100)
        plt.xticks(rotation=45)

        # Plot profit factors
        plt.subplot(3, 2, 2)
        profit_factors = [min(stats['profit_factor'], 5) for stats in self.summary_stats.values()]  # Cap at 5 for readability

        plt.bar(test_names, profit_factors)
        plt.title('Profit Factor by Test')
        plt.ylabel('Profit Factor')
        plt.xticks(rotation=45)

        # Plot dollar returns
        plt.subplot(3, 2, 3)
        dollar_returns = [stats.get('total_dollar_return', 0) for stats in self.summary_stats.values()]

        plt.bar(test_names, dollar_returns)
        plt.title('Total Dollar Return by Test')
        plt.ylabel('Dollar Return ($)')
        plt.xticks(rotation=45)

        # Plot trade counts
        plt.subplot(3, 2, 4)
        winning_trades = [stats['winning_trades'] for stats in self.summary_stats.values()]
        losing_trades = [stats['losing_trades'] for stats in self.summary_stats.values()]

        x = np.arange(len(test_names))
        width = 0.35

        plt.bar(x - width/2, winning_trades, width, label='Winning Trades')
        plt.bar(x + width/2, losing_trades, width, label='Losing Trades')
        plt.xticks(x, test_names, rotation=45)
        plt.title('Winning vs Losing Trades by Test')
        plt.ylabel('Number of Trades')
        plt.legend()

        # Plot max drawdown
        plt.subplot(3, 2, 5)
        max_drawdowns = [stats.get('max_drawdown', 0) for stats in self.summary_stats.values()]

        plt.bar(test_names, max_drawdowns)
        plt.title('Maximum Drawdown by Test')
        plt.ylabel('Drawdown ($)')
        plt.xticks(rotation=45)

        # Plot max drawdown percentage
        plt.subplot(3, 2, 6)
        max_drawdown_pcts = [stats.get('max_drawdown_pct', 0) for stats in self.summary_stats.values()]

        plt.bar(test_names, max_drawdown_pcts)
        plt.title('Maximum Drawdown Percentage by Test')
        plt.ylabel('Drawdown (%)')
        plt.xticks(rotation=45)

        plt.tight_layout()
        plt.show()

    def plot_trade_distribution(self, test_name=None, figsize=(12, 8)):
        """
        Plot distribution of trade results

        Parameters:
        test_name (str): Name of the test to plot (if None, uses first test)
        figsize (tuple): Figure size for the plot
        """
        if not self.results:
            print("No backtest results to plot")
            return

        # If test_name not specified, use the first test
        if test_name is None:
            test_name = list(self.results.keys())[0]

        if test_name not in self.results:
            print(f"Test '{test_name}' not found. Available tests: {list(self.results.keys())}")
            return

        test_results = self.results[test_name]
        trades = test_results['trades']

        if not trades:
            print(f"No trades found for test '{test_name}'")
            return

        plt.figure(figsize=figsize)

        # Plot profit/loss distribution
        plt.subplot(2, 2, 1)
        profits = [trade['profit_loss_pct'] for trade in trades]

        plt.hist(profits, bins=20)
        plt.title(f'Profit/Loss Distribution - {test_name}')
        plt.xlabel('Profit/Loss (%)')
        plt.ylabel('Number of Trades')

        # Plot exit types
        plt.subplot(2, 2, 2)
        exit_types = [trade['exit_type'] for trade in trades]
        exit_counts = defaultdict(int)
        for exit_type in exit_types:
            exit_counts[exit_type] += 1

        plt.bar(exit_counts.keys(), exit_counts.values())
        plt.title('Exit Types')
        plt.ylabel('Number of Trades')

        # Plot holding time distribution
        plt.subplot(2, 2, 3)
        hold_times = [min(trade['hold_time'], 30) for trade in trades]  # Cap at 30 days for readability

        plt.hist(hold_times, bins=15)
        plt.title('Holding Time Distribution')
        plt.xlabel('Holding Time (Days)')
        plt.ylabel('Number of Trades')

        # Plot max adverse excursion vs. profit
        plt.subplot(2, 2, 4)
        max_excursions = [trade['max_adverse_excursion'] for trade in trades]

        plt.scatter(max_excursions, profits)
        plt.title('Max Adverse Excursion vs. Profit')
        plt.xlabel('Maximum Adverse Excursion (%)')
        plt.ylabel('Profit/Loss (%)')
        plt.axhline(y=0, color='r', linestyle='-', alpha=0.3)

        plt.tight_layout()
        plt.show()

    def get_trade_metrics_by_profit_target(self):
        """
        metrics grouped by profit target

        Returns:
        pandas.DataFrame: DataFrame with metrics by profit target
        """
        if not self.results:
            return pd.DataFrame()

        metrics = []

        for test_name, test_results in self.results.items():
            metric = {
                'test_name': test_name,
                'win_rate': test_results['win_rate'] * 100,
                'win_rate_dollar': test_results.get('win_rate_dollar', 0) * 100,
                'profit_factor': test_results['profit_factor'],
                'profit_factor_dollar': test_results.get('profit_factor_dollar', 0),
                'total_return': test_results['total_return'],
                'total_dollar_return': test_results.get('total_dollar_return', 0),
                'total_trades': test_results['total_trades'],
                'winning_trades': test_results['winning_trades'],
                'losing_trades': test_results['losing_trades'],
                'targets_hit': sum(1 for trade in test_results['trades'] if trade['exit_type'] == 'TARGET'),
                'stops_hit': sum(1 for trade in test_results['trades'] if trade['exit_type'] == 'STOP'),
                'open_trades': sum(1 for trade in test_results['trades'] if trade['exit_type'] == 'OPEN'),
                'avg_holding_days': np.mean([trade['hold_time'] for trade in test_results['trades']]) if test_results['trades'] else 0,
                'max_drawdown': test_results.get('max_drawdown', 0),
                'max_drawdown_pct': test_results.get('max_drawdown_pct', 0)
            }

            # Calculate target hit rate
            if metric['total_trades'] > 0:
                metric['target_hit_rate'] = (metric['targets_hit'] / metric['total_trades']) * 100
            else:
                metric['target_hit_rate'] = 0

            metrics.append(metric)

        return pd.DataFrame(metrics)

    def export_trades_to_csv(self, filename=None):
        """
        Export all trade data to a CSV file

        Parameters:
        filename (str): Output filename (if None, uses strategy_name + timestamp)

        Returns:
        str: Path to the exported file
        """
        if not self.trades:
            print("No trades to export")
            return None

        # Create DataFrame from trades
        trades_df = pd.DataFrame(self.trades)

        # Generate filename if not provided
        if filename is None:
            timestamp = datetime.now().strftime("%Y%m%d_%H%M%S")
            filename = f"{self.strategy_name.replace(' ', '_')}_{timestamp}.csv"

        # Export to CSV
        trades_df.to_csv(filename, index=False)
        print(f"Trades exported to {filename}")

        return filename