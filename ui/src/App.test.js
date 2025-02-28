import { render, screen } from '@testing-library/react';
import App from './App';
import * as test from "node:test";

test('renders trading lab heading', () => {
  render(<App />);
  const headingElement = screen.getByText(/TradingLab/i);
  expect(headingElement).toBeInTheDocument();
});