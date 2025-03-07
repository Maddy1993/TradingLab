import React from 'react';
import ReactDOM from 'react-dom/client';
import './index.css';
import App from './App';
import reportWebVitals from './reportWebVitals';

// Simple fallback component if the app fails to mount
class RootErrorBoundary extends React.Component {
  constructor(props) {
    super(props);
    this.state = { hasError: false };
  }

  static getDerivedStateFromError(error) {
    return { hasError: true };
  }

  componentDidCatch(error, info) {
    console.error('Root level error:', error, info);
  }

  render() {
    if (this.state.hasError) {
      return (
        <div style={{ 
          display: 'flex', 
          flexDirection: 'column', 
          alignItems: 'center', 
          justifyContent: 'center', 
          height: '100vh', 
          color: 'white', 
          backgroundColor: '#121212', 
          padding: '20px' 
        }}>
          <h2>Something went wrong loading TradingLab</h2>
          <p>Please try refreshing the page. If the issue persists, contact support.</p>
          <a 
            href="/" 
            style={{ 
              backgroundColor: '#3f51b5', 
              color: 'white', 
              padding: '10px 20px', 
              borderRadius: '4px', 
              textDecoration: 'none',
              marginTop: '20px'
            }}
          >
            Reload Application
          </a>
        </div>
      );
    }
    return this.props.children;
  }
}

// Create the root element first
let root;

// Wrap the initialization in a DOMContentLoaded event to ensure the DOM is ready
document.addEventListener('DOMContentLoaded', () => {
  // Try to safely mount the application
  try {
    root = ReactDOM.createRoot(document.getElementById('root'));
    root.render(
      <RootErrorBoundary>
        <App />
      </RootErrorBoundary>
    );
    
    // Report web vitals
    reportWebVitals();
  } catch (error) {
    console.error('Failed to render application:', error);
    
    // Fallback if the app can't even mount
    document.getElementById('root').innerHTML = `
      <div style="display: flex; flex-direction: column; align-items: center; justify-content: center; height: 100vh; color: white; background-color: #121212; padding: 20px;">
        <h2>Unable to load TradingLab</h2>
        <p>A critical error occurred while starting the application. Please try again later.</p>
        <a href="/" style="background-color: #3f51b5; color: white; padding: 10px 20px; border-radius: 4px; text-decoration: none; margin-top: 20px;">Reload Application</a>
      </div>
    `;
  }
});