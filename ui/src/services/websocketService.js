import { useState, useEffect, useRef, useCallback } from 'react';

// Fix for potential issues when running behind a load balancer or ingress
const getBaseUrl = () => {
  try {
    // Check if window is defined (for SSR compatibility)
    if (typeof window !== 'undefined' && window.location) {
      return window.location.protocol === 'https:'
        ? `wss://${window.location.host}/api/ws`
        : `ws://${window.location.host}/api/ws`;
    }
    // Fallback for when window is not available
    return '/api/ws';
  } catch (e) {
    console.error('Error constructing WebSocket URL:', e);
    // Fallback to relative URL which will use the same host
    return '/api/ws';
  }
};

// Lazy initialization of BASE_URL to prevent reference errors
let BASE_URL;
try {
  BASE_URL = getBaseUrl();
} catch (e) {
  console.error('Failed to initialize WebSocket URL:', e);
  BASE_URL = '/api/ws';
}

// Special subject for system status updates
const SYSTEM_STATUS_SUBJECT = 'system.status';

// Hook for subscribing to market data
export const useMarketData = (ticker) => {
  const [marketData, setMarketData] = useState(null);
  const { connected, subscribe, unsubscribe } = useWebSocketConnection();

  useEffect(() => {
    if (!ticker || !connected) return;

    const subject = `market.live.${ticker}`;
    const handleMarketData = (data) => {
      // Check if the data includes caching information
      if (data && data.metadata && data.metadata.isCached) {
        // Add caching information to the market data
        setMarketData({
          ...data,
          isCached: true,
          timestamp: data.metadata.timestamp || new Date().toISOString()
        });
      } else {
        setMarketData(data);
      }
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
      // Check if the data includes caching information
      if (data && data.metadata && data.metadata.isCached) {
        // Add caching flag to the signal data
        setSignals((prevSignals) => [
          ...prevSignals, 
          {
            ...data,
            isCached: true,
            timestamp: data.metadata.timestamp || new Date().toISOString()
          }
        ]);
      } else {
        setSignals((prevSignals) => [...prevSignals, data]);
      }
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
  
  // Always subscribe to system status by default
  useEffect(() => {
    if (connected && !subscriptionsRef.current[SYSTEM_STATUS_SUBJECT]) {
      // Auto-subscribe to system status
      sendSubscriptionRequest(SYSTEM_STATUS_SUBJECT);
      subscriptionsRef.current[SYSTEM_STATUS_SUBJECT] = true;
    }
  }, [connected]);

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

    // Don't allow unsubscribing from system status subject
    if (subject === SYSTEM_STATUS_SUBJECT) {
      console.warn('Cannot unsubscribe from system status subject');
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

  const connect = useCallback(() => {
    // Safety check to ensure we're in a browser environment
    if (typeof window === 'undefined' || !window.WebSocket) {
      console.error('WebSocket not supported in this environment');
      setError('WebSocket not supported');
      return;
    }

    if (socketRef.current) {
      try {
        socketRef.current.close();
      } catch (e) {
        console.error('Error closing existing socket:', e);
      }
    }

    try {
      // Re-fetch the BASE_URL in case it's changed
      const currentUrl = getBaseUrl();
      const socket = new WebSocket(currentUrl);
      socketRef.current = socket;
      
      console.log('Connecting to WebSocket at:', currentUrl);

      socket.onopen = () => {
        console.log('WebSocket connected successfully');
        setConnected(true);
        setError(null);
        reconnectCountRef.current = 0; // Reset reconnect count on successful connection

        // Subscribe to system status first
        if (!subscriptionsRef.current[SYSTEM_STATUS_SUBJECT]) {
          sendSubscriptionRequest(SYSTEM_STATUS_SUBJECT);
          subscriptionsRef.current[SYSTEM_STATUS_SUBJECT] = true;
        }

        // Then resubscribe to all subjects on reconnect
        Object.keys(subscriptionsRef.current).forEach((subject) => {
          if (subject !== SYSTEM_STATUS_SUBJECT) { // Already handled system status
            sendSubscriptionRequest(subject);
          }
        });
      };

      socket.onclose = (event) => {
        console.log('WebSocket closed:', event.code, event.reason);
        setConnected(false);

        // Attempt to reconnect with exponential backoff
        if (reconnectCountRef.current < 5) {
          const backoffTime = Math.min(3000 * Math.pow(1.5, reconnectCountRef.current), 30000);
          console.log(`Reconnecting (attempt ${reconnectCountRef.current + 1}/5) in ${backoffTime/1000}s...`);
          
          reconnectTimeoutRef.current = setTimeout(() => {
            reconnectCountRef.current += 1;
            connect();
          }, backoffTime);
        } else {
          setError('Maximum reconnection attempts reached');
        }
      };

      socket.onerror = (err) => {
        console.error('WebSocket error:', err);
        setError(err.message || 'WebSocket error');
      };

      socket.onmessage = (event) => {
        try {
          const message = JSON.parse(event.data);
          const { subject, data } = message;

          if (subject === SYSTEM_STATUS_SUBJECT) {
            // Always log system status messages to console
            console.log('System status update:', data);
          }

          if (subscribersRef.current[subject]) {
            subscribersRef.current[subject].forEach((callback) => {
              try {
                callback(data);
              } catch (callbackError) {
                console.error(`Error in subscriber callback for ${subject}:`, callbackError);
              }
            });
          }
        } catch (parseError) {
          console.error('Error parsing WebSocket message:', parseError, event.data);
        }
      };
    } catch (connectionError) {
      console.error('Failed to create WebSocket connection:', connectionError);
      setError(`Connection error: ${connectionError.message}`);
      
      // Try to reconnect even on connection errors with exponential backoff
      if (reconnectCountRef.current < 5) {
        const backoffTime = Math.min(3000 * Math.pow(1.5, reconnectCountRef.current), 30000);
        
        reconnectTimeoutRef.current = setTimeout(() => {
          reconnectCountRef.current += 1;
          connect();
        }, backoffTime);
      }
    }
  }, []);

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
    // Add a small delay before connecting to ensure the DOM is fully loaded
    const initTimeout = setTimeout(() => {
      connect();
      console.log('WebSocket: Attempting connection to', BASE_URL);
    }, 500);

    return () => {
      clearTimeout(initTimeout);
      
      if (reconnectTimeoutRef.current) {
        clearTimeout(reconnectTimeoutRef.current);
      }

      if (socketRef.current) {
        socketRef.current.close();
      }
    };
  }, [connect]);

  return { connected, subscribe, unsubscribe, reconnect, error };
};