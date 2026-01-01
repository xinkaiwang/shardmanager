import { useState } from 'react';
import { Routes, Route, Link } from 'react-router-dom';
import { 
  AppBar, 
  Box, 
  CssBaseline, 
  Drawer, 
  IconButton, 
  List, 
  ListItem, 
  ListItemButton, 
  ListItemIcon, 
  ListItemText, 
  Toolbar, 
  Typography,
  ThemeProvider,
  createTheme
} from '@mui/material';
import {
  Menu as MenuIcon,
  Settings as SettingsIcon,
  Computer as ComputerIcon,
  FormatListBulleted as FormatListBulletedIcon,
} from '@mui/icons-material';

// 扩展断点类型定义
declare module '@mui/material/styles' {
  interface BreakpointOverrides {
    xs: true;
    sm: true;
    md: true;
    lg: true;
    xl: true;
    xxl: true; // 添加新的XXL断点
  }
}

// 导入页面组件
import WorkersPage from './components/WorkersPage';
import ConfigPage from './components/ConfigPage';
import ShardPlanPage from './components/ShardPlanPage';
// 导入SVG logo
import logoSvg from './assets/logo.svg';

const drawerWidth = 240;

// 创建主题
const theme = createTheme({
  palette: {
    primary: {
      main: '#1976d2',
    },
    secondary: {
      main: '#dc004e',
    },
  },
  // 自定义断点配置
  breakpoints: {
    values: {
      xs: 0,      // 移动设备
      sm: 600,    // 平板竖屏 (≥600px)
      md: 900,    // 平板横屏 (≥900px)
      lg: 1200,   // 小型笔记本 (≥1200px)
      xl: 1700,   // 大型笔记本/桌面显示器 (≥1536px，如16英寸MBP的有效宽度)
      xxl: 1920,  // 全高清显示器 (≥1920px)
    },
  },
});

export default function App() {
  const [mobileOpen, setMobileOpen] = useState(false);
  const [sidebarCollapsed, setSidebarCollapsed] = useState(false);

  const handleDrawerToggle = () => {
    setMobileOpen(!mobileOpen);
  };

  const handleSidebarCollapse = () => {
    setSidebarCollapsed(!sidebarCollapsed);
  };
  
  // 计算侧边栏宽度
  const currentDrawerWidth = sidebarCollapsed ? 64 : drawerWidth;

  const drawer = (
    <div>
      <Toolbar>
        <Box sx={{ 
          display: 'flex', 
          alignItems: 'center', 
          gap: 1,
          justifyContent: sidebarCollapsed ? 'center' : 'flex-start',
          width: '100%'
        }}>
          <img src={logoSvg} alt="ShardManager Logo" style={{ width: '40px', height: '40px' }} />
          {!sidebarCollapsed && (
            <Typography variant="h6" noWrap component="div">
              ShardManager
            </Typography>
          )}
        </Box>
      </Toolbar>
      <List>
        <ListItem disablePadding component={Link} to="/" style={{ textDecoration: 'none', color: 'inherit' }}>
          <ListItemButton sx={{ justifyContent: sidebarCollapsed ? 'center' : 'flex-start' }}>
            <ListItemIcon sx={{ minWidth: sidebarCollapsed ? 0 : 56 }}>
              <ComputerIcon />
            </ListItemIcon>
            {!sidebarCollapsed && <ListItemText primary="Workers" />}
          </ListItemButton>
        </ListItem>
        <ListItem disablePadding component={Link} to="/shard-plan" style={{ textDecoration: 'none', color: 'inherit' }}>
          <ListItemButton sx={{ justifyContent: sidebarCollapsed ? 'center' : 'flex-start' }}>
            <ListItemIcon sx={{ minWidth: sidebarCollapsed ? 0 : 56 }}>
              <FormatListBulletedIcon />
            </ListItemIcon>
            {!sidebarCollapsed && <ListItemText primary="Shard Plan" />}
          </ListItemButton>
        </ListItem>
        <ListItem disablePadding component={Link} to="/config" style={{ textDecoration: 'none', color: 'inherit' }}>
          <ListItemButton sx={{ justifyContent: sidebarCollapsed ? 'center' : 'flex-start' }}>
            <ListItemIcon sx={{ minWidth: sidebarCollapsed ? 0 : 56 }}>
              <SettingsIcon />
            </ListItemIcon>
            {!sidebarCollapsed && <ListItemText primary="Config" />}
          </ListItemButton>
        </ListItem>
      </List>
    </div>
  );

  return (
    <ThemeProvider theme={theme}>
      <Box sx={{ display: 'flex' }}>
        <CssBaseline />
        <AppBar
          position="fixed"
          sx={{
            width: { md: `calc(100% - ${currentDrawerWidth}px)` },
            ml: { md: `${currentDrawerWidth}px` },
            transition: theme.transitions.create(['width', 'margin'], {
              easing: theme.transitions.easing.sharp,
              duration: theme.transitions.duration.leavingScreen,
            }),
          }}
        >
          <Toolbar>
            <IconButton
              color="inherit"
              aria-label="open drawer"
              edge="start"
              onClick={handleDrawerToggle}
              sx={{ mr: 2, display: { md: 'none' } }}
            >
              <MenuIcon />
            </IconButton>
            
            {/* 添加侧边栏折叠按钮 */}
            <IconButton
              color="inherit"
              aria-label="collapse sidebar"
              edge="start"
              onClick={handleSidebarCollapse}
              sx={{ mr: 2, display: { xs: 'none', md: 'block' } }}
            >
              <MenuIcon />
            </IconButton>
            
            <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
              <img 
                src={logoSvg} 
                alt="ShardManager Logo" 
                style={{ width: '32px', height: '32px', filter: 'brightness(0) invert(1)' }} 
              />
              <Typography variant="h6" noWrap component="div">
                ShardManager UI
              </Typography>
            </Box>
          </Toolbar>
        </AppBar>
        <Box
          component="nav"
          sx={{ 
            width: { md: currentDrawerWidth }, 
            flexShrink: { md: 0 },
            transition: theme.transitions.create('width', {
              easing: theme.transitions.easing.sharp,
              duration: theme.transitions.duration.enteringScreen,
            }),
          }}
          aria-label="mailbox folders"
        >
          {/* 移动端抽屉 */}
          <Drawer
            variant="temporary"
            open={mobileOpen}
            onClose={handleDrawerToggle}
            ModalProps={{
              keepMounted: true, // 改善移动端性能
            }}
            sx={{
              display: { xs: 'block', md: 'none' },
              '& .MuiDrawer-paper': { 
                boxSizing: 'border-box', 
                width: drawerWidth 
              },
            }}
          >
            {drawer}
          </Drawer>
          {/* 桌面端抽屉 */}
          <Drawer
            variant="permanent"
            sx={{
              display: { xs: 'none', md: 'block' },
              '& .MuiDrawer-paper': { 
                boxSizing: 'border-box', 
                width: currentDrawerWidth,
                transition: theme.transitions.create('width', {
                  easing: theme.transitions.easing.sharp,
                  duration: theme.transitions.duration.enteringScreen,
                }),
                overflowX: 'hidden'
              },
            }}
            open
          >
            {drawer}
          </Drawer>
        </Box>
        <Box
          component="main"
          sx={{ 
            flexGrow: 1, 
            p: 3, 
            width: { md: `calc(100% - ${currentDrawerWidth}px)` },
            transition: theme.transitions.create(['width', 'margin'], {
              easing: theme.transitions.easing.sharp,
              duration: theme.transitions.duration.enteringScreen,
            }),
          }}
        >
          <Toolbar />
          <Routes>
            <Route path="/" element={<WorkersPage />} />
            <Route path="/shard-plan" element={<ShardPlanPage />} />
            <Route path="/config" element={<ConfigPage />} />
          </Routes>
        </Box>
      </Box>
    </ThemeProvider>
  );
} 