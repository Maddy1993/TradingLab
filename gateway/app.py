#!/usr/bin/env python3
"""
API Gateway for TradingLab UI
This service provides a REST API that proxies requests to the TradingLab gRPC service.
"""
import os
import grpc
import sys
import json
import logging
from flask import Flask, request, jsonify, send_from_directory
from flask_cors import CORS
from werkzeug.middleware.proxy_fix import ProxyFix

# Configure logging
logging.basicConfig(
        level=logging.INFO,
        format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)

# Import generated gRPC code
try:
    sys.path.append("/app/proto")
    import trading_pb2
    import trading_pb2_grpc
except ImportError as e:
    logger.error(f"Could not import proto modules: {e}")
    raise

# Initialize Flask app
app = Flask(__name__, static_folder='/app/ui/build')
app.wsgi_app = ProxyFix(app.wsgi_app, x_for=1, x_proto=1, x_host=1, x_prefix=1)
CORS(app)  # Enable CORS for all routes

# Get environment variables
TRADINGLAB_HOST = os.getenv('TRADINGLAB_HOST', 'tradinglab-service')
TRADINGLAB_PORT = os.getenv('TRADINGLAB_PORT', '50052')
TRADINGLAB_ADDRESS = f"{TRADINGLAB_HOST}:{TRADINGLAB_PORT}"

DEFAULT_TICKERS = ['SPY', 'AAPL', 'MSFT', 'GOOGL', 'AMZN']

# Create gRPC channel and stub
def get_grpc_stub():
    """Create a gRPC connection to the TradingLab service"""
    channel = grpc.insecure_channel(TRADINGLAB_ADDRESS)
    return trading_pb2_grpc.TradingServiceStub(channel)

# API Routes
@app.route('/api/health', methods=['GET'])
def health_check():
    """Health check endpoint"""
    try:
        # Try to connect to the gRPC service
        stub = get_grpc_stub()
        # Make a simple request to check connection
        request = trading_pb2.HistoricalDataRequest(ticker="SPY", days=1)
        response = stub.GetHistoricalData(request)
        return jsonify({"status": "healthy", "grpc_connected": True})
    except Exception as e:
        logger.error(f"Health check failed: {str(e)}")
        return jsonify({"status": "unhealthy", "error": str(e)}), 500

@app.route('/api/tickers', methods=['GET'])
def get_tickers():
    """Get list of available tickers"""
    # This is a mock endpoint - in a real app, you might fetch this from a database
    return jsonify(DEFAULT_TICKERS)

@app.route('/api/historical-data', methods=['GET'])
def get_historical_data():
    """Get historical price data for a ticker"""
    try:
        ticker = request.args.get('ticker', 'SPY')
        days = int(request.args.get('days', 30))

        stub = get_grpc_stub()
        request_proto = trading_pb2.HistoricalDataRequest(ticker=ticker, days=days)
        response = stub.GetHistoricalData(request_proto)

        # Convert protobuf to JSON-serializable format
        data = []
        for candle in response.candles:
            data.append({
                'date': candle.date,
                'open': candle.open,
                'high': candle.high,
                'low': candle.low,
                'close': candle.close,
                'volume': candle.volume
            })

        return jsonify(data)
    except Exception as e:
        logger.error(f"Error getting historical data: {str(e)}")
        return jsonify({"error": str(e)}), 500

@app.route('/api/signals', methods=['GET'])
def get_signals():
    """Get trading signals for a ticker"""
    try:
        ticker = request.args.get('ticker', 'SPY')
        days = int(request.args.get('days', 30))
        strategy = request.args.get('strategy', 'RedCandle')

        stub = get_grpc_stub()
        request_proto = trading_pb2.SignalRequest(
                ticker=ticker,
                days=days,
                strategy=strategy
        )
        response = stub.GenerateSignals(request_proto)

        # Convert protobuf to JSON-serializable format
        signals = []
        for signal in response.signals:
            signals.append({
                'date': signal.date,
                'signal_type': signal.signal_type,
                'entry_price': signal.entry_price,
                'stoploss': signal.stoploss
            })

        return jsonify(signals)
    except Exception as e:
        logger.error(f"Error getting signals: {str(e)}")
        return jsonify({"error": str(e)}), 500

@app.route('/api/backtest', methods=['GET'])
def run_backtest():
    """Run a backtest for a ticker and strategy"""
    try:
        ticker = request.args.get('ticker', 'SPY')
        days = int(request.args.get('days', 30))
        strategy = request.args.get('strategy', 'RedCandle')

        # Parse profit targets (comma-separated list)
        profit_targets_str = request.args.get('profit_targets', '')
        profit_targets = [float(pt) for pt in profit_targets_str.split(',') if pt.strip()] if profit_targets_str else [5, 10, 15]

        # Parse risk-reward ratios (comma-separated list)
        rr_ratios_str = request.args.get('risk_reward_ratios', '')
        rr_ratios = [float(rr) for rr in rr_ratios_str.split(',') if rr.strip()] if rr_ratios_str else []

        # Parse dollar profit targets (comma-separated list)
        dollar_targets_str = request.args.get('profit_targets_dollar', '')
        dollar_targets = [float(dt) for dt in dollar_targets_str.split(',') if dt.strip()] if dollar_targets_str else [100, 250, 500]

        stub = get_grpc_stub()
        request_proto = trading_pb2.BacktestRequest(
                ticker=ticker,
                days=days,
                strategy=strategy,
                profit_targets=profit_targets,
                risk_reward_ratios=rr_ratios,
                profit_targets_dollar=dollar_targets
        )
        response = stub.RunBacktest(request_proto)

        # Convert protobuf map to JSON-serializable format
        results = {}
        for name, result in response.results.items():
            results[name] = {
                'win_rate': result.win_rate,
                'profit_factor': result.profit_factor,
                'total_return': result.total_return,
                'total_return_pct': result.total_return_pct,
                'total_trades': result.total_trades,
                'winning_trades': result.winning_trades,
                'losing_trades': result.losing_trades,
                'max_drawdown': result.max_drawdown,
                'max_drawdown_pct': result.max_drawdown_pct
            }

        return jsonify(results)
    except Exception as e:
        logger.error(f"Error running backtest: {str(e)}")
        return jsonify({"error": str(e)}), 500

@app.route('/api/recommendations', methods=['GET'])
def get_recommendations():
    """Get options recommendations for a ticker"""
    try:
        ticker = request.args.get('ticker', 'SPY')
        days = int(request.args.get('days', 30))
        strategy = request.args.get('strategy', 'RedCandle')

        stub = get_grpc_stub()
        request_proto = trading_pb2.RecommendationRequest(
                ticker=ticker,
                days=days,
                strategy=strategy
        )
        response = stub.GetOptionsRecommendations(request_proto)

        # Convert protobuf to JSON-serializable format
        recommendations = []
        for rec in response.recommendations:
            recommendations.append({
                'date': rec.date,
                'signal_type': rec.signal_type,
                'stock_price': rec.stock_price,
                'stoploss': rec.stoploss,
                'option_type': rec.option_type,
                'strike': rec.strike,
                'expiration': rec.expiration,
                'delta': rec.delta,
                'iv': rec.iv,
                'price': rec.price
            })

        return jsonify(recommendations)
    except Exception as e:
        logger.error(f"Error getting recommendations: {str(e)}")
        return jsonify({"error": str(e)}), 500

# Serve React frontend
@app.route('/', defaults={'path': ''})
@app.route('/<path:path>')
def serve_frontend(path):
    """Serve the React frontend"""
    if path and os.path.exists(os.path.join(app.static_folder, path)):
        return send_from_directory(app.static_folder, path)
    else:
        return send_from_directory(app.static_folder, 'index.html')

if __name__ == '__main__':
    # Get port from environment variable or use default
    port = int(os.getenv('PORT', 5000))

    # Log configuration
    logger.info(f"Starting API Gateway on port {port}")
    logger.info(f"Connecting to TradingLab service at {TRADINGLAB_ADDRESS}")

    # Start the server
    app.run(host='0.0.0.0', port=port, debug=False)