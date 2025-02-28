import React, { useState } from 'react';
import { Drawer, Box, List, ListItem, ListItemButton, ListItemIcon, ListItemText, Divider } from '@mui/material';
import { useNavigate, useLocation } from 'react-router-dom';
import DashboardIcon from '@mui/icons-material/Dashboard';
import TimelineIcon from '@mui/icons-material/Timeline';
import SignalCellularAltIcon from '@mui/icons-material/SignalCellularAlt';
import AssessmentIcon from '@mui/icons-material/Assessment';
import RecommendIcon from '@mui/icons-material/Recommend';

const Sidebar = ({ drawerWidth }) => {
  const navigate = useNavigate();
  const location = useLocation();
  const [mobileOpen, setMobileOpen] = useState(false);

  const menuItems = [
    { text: 'Dashboard', icon: <DashboardIcon />, path: '/' },
    { text: 'Historical Data', icon: <TimelineIcon />, path: '/historical' },
    { text: 'Trading Signals', icon: <SignalCellularAltIcon />, path: '/signals' },
    { text: 'Backtest Results', icon: <AssessmentIcon />, path: '/backtest' },
    { text: 'Recommendations', icon: <RecommendIcon />, path: '/recommendations' },
  ];

  const handleDrawerToggle = () => {
    setMobileOpen(!mobileOpen);
  };

  const drawer = (
      <div>
        <Box sx={{ p: 2, height: 64, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
          <img src="/logo192.png" alt="TradingLab Logo" style={{ height: '40px' }} />
        </Box>
        <Divider />
        <List>
          {menuItems.map((item) => (
              <ListItem key={item.text} disablePadding>
                <ListItemButton
                    selected={location.pathname === item.path}
                    onClick={() => {
                      navigate(item.path);
                      setMobileOpen(false);
                    }}
                >
                  <ListItemIcon>{item.icon}</ListItemIcon>
                  <ListItemText primary={item.text} />
                </ListItemButton>
              </ListItem>
          ))}
        </List>
        <Divider />
        <Box sx={{ p: 2, position: 'absolute', bottom: 0, width: '100%' }}>
          <Divider />
          <Box sx={{ p: 2, textAlign: 'center' }}>
            <ListItemText primary="TradingLab v1.0" secondary="© 2025" />
          </Box>
        </Box>
      </div>
  );

  return (
      <Box
          component="nav"
          sx={{ width: { sm: drawerWidth }, flexShrink: { sm: 0 } }}
          aria-label="mailbox folders"
      >
        {/* The implementation can be swapped with js to avoid SEO duplication of links. */}
        <Drawer
            variant="temporary"
            open={mobileOpen}
            onClose={handleDrawerToggle}
            ModalProps={{
              keepMounted: true, // Better open performance on mobile.
            }}
            sx={{
              display: { xs: 'block', sm: 'none' },
              '& .MuiDrawer-paper': { boxSizing: 'border-box', width: drawerWidth },
            }}
        >
          {drawer}
        </Drawer>
        <Drawer
            variant="permanent"
            sx={{
              display: { xs: 'none', sm: 'block' },
              '& .MuiDrawer-paper': { boxSizing: 'border-box', width: drawerWidth },
            }}
            open
        >
          {drawer}
        </Drawer>
      </Box>
  );
};

export default Sidebar;