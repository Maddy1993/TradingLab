import { useState, useEffect, useRef } from 'react';

const BASE_URL = window.location.protocol === 'https:'
    ? `wss://${window.location.host}/api/ws`
    : `ws://${window.location.host}/api/ws`;

// Hook for subscribing to market data
export const useMarketData = (ticker) => {
  const [marketData, setMarketData] = useState(null);
  const { connected, subscribe, unsubscribe } = useWebSocketConnection();

  useEffect(() => {
    if (!ticker || !connected) return;

    const subject = `market.live.${ticker}`;
    const handleMarketData = (data) => {
      setMarketData(data);
    };

    subscribe(subject, handleMarketData);

    return () => {
      unsubscribe(subject, handleMarketData);
    };
  }, [ticker, connected, subscribe, unsubscribe]);

  return marketData;
};

// Hook for subscribing to signals
export const useSignals = (ticker) => {
  const [signals, setSignals] = useState([]);
  const { connected, subscribe, unsubscribe } = useWebSocketConnection();

  useEffect(() => {
    if (!ticker || !connected) return;

    const subject = `signals.${ticker}`;
    const handleSignal = (data) => {
      setSignals((prevSignals) => [...prevSignals, data]);
    };

    subscribe(subject, handleSignal);

    return () => {
      unsubscribe(subject, handleSignal);
    };
  }, [ticker, connected, subscribe, unsubscribe]);

  return signals;
};

// Main WebSocket connection hook
export const useWebSocketConnection = () => {
  const [connected, setConnected] = useState(false);
  const [error, setError] = useState(null);
  const socketRef = useRef(null);
  const subscriptionsRef = useRef({});
  const subscribersRef = useRef({});
  const reconnectTimeoutRef = useRef(null);
  const reconnectCountRef = useRef(0);

  // Function to send subscription/unsubscription request
  const sendSubscriptionRequest = (subject, isUnsubscribe = false) => {
    if (!socketRef.current || socketRef.current.readyState !== WebSocket.OPEN) {
      return false;
    }
    socketRef.current.send(
        JSON.stringify({
          action: isUnsubscribe ? 'unsubscribe' : 'subscribe',
          subject,
        })
    );
    return true;
  };

  // Subscribe to a subject
  const subscribe = (subject, callback) => {
    if (!subject || typeof callback !== 'function') {
      return false;
    }

    // Add to subscribers
    if (!subscribersRef.current[subject]) {
      subscribersRef.current[subject] = [];
    }
    subscribersRef.current[subject].push(callback);

    // Add to subscriptions and send request if connected
    if (!subscriptionsRef.current[subject]) {
      subscriptionsRef.current[subject] = true;

      if (connected) {
        sendSubscriptionRequest(subject);
      }
    }

    return true;
  };

  // Unsubscribe from a subject
  const unsubscribe = (subject, callback) => {
    if (!subject) {
      return false;
    }

    if (callback && subscribersRef.current[subject]) {
      // Remove specific callback
      subscribersRef.current[subject] = subscribersRef.current[subject].filter((cb) => cb !== callback);

      // If no more subscribers, unsubscribe from the subject
      if (subscribersRef.current[subject].length === 0) {
        delete subscribersRef.current[subject];
        delete subscriptionsRef.current[subject];

        if (connected) {
          sendSubscriptionRequest(subject, true);
        }
      }
    } else {
      // Remove all subscribers for this subject
      delete subscribersRef.current[subject];
      delete subscriptionsRef.current[subject];

      if (connected) {
        sendSubscriptionRequest(subject, true);
      }
    }

    return true;
  };

  const connect = () => {
    if (socketRef.current) {
      socketRef.current.close();
    }

    const socket = new WebSocket(BASE_URL);
    socketRef.current = socket;

    socket.onopen = () => {
      setConnected(true);
      setError(null);

      // Resubscribe to all subjects on reconnect
      Object.keys(subscriptionsRef.current).forEach((subject) => {
        sendSubscriptionRequest(subject);
      });
    };

    socket.onclose = () => {
      setConnected(false);

      // Attempt to reconnect
      if (reconnectCountRef.current < 5) {
        reconnectTimeoutRef.current = setTimeout(() => {
          reconnectCountRef.current += 1;
          connect();
        }, 3000);
      }
    };

    socket.onerror = (err) => {
      setError(err.message || 'WebSocket error');
    };

    socket.onmessage = (event) => {
      const message = JSON.parse(event.data);
      const { subject, data } = message;

      if (subscribersRef.current[subject]) {
        subscribersRef.current[subject].forEach((callback) => {
          callback(data);
        });
      }
    };
  };

  // Function to manually reconnect
  const reconnect = () => {
    if (socketRef.current) {
      socketRef.current.close();
    }
    reconnectCountRef.current = 0;
    connect();
  };

  // Clean up on unmount
  useEffect(() => {
    connect();

    return () => {
      if (reconnectTimeoutRef.current) {
        clearTimeout(reconnectTimeoutRef.current);
      }

      if (socketRef.current) {
        socketRef.current.close();
      }
    };
  }, []);

  return { connected, subscribe, unsubscribe, reconnect, error };
};