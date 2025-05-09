import { useState, useEffect, useRef, useCallback } from 'react';
import {
  Typography,
  Box,
  Paper,
  TextField,
  IconButton,
  List,
  ListItem,
  ListItemText,
  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
  Button,
  CircularProgress,
  Snackbar,
  Alert,
  styled,
  Fab,
  FormControl,
  InputLabel,
  Select,
  MenuItem,
  SelectChangeEvent,
  Chip,
  Container,
} from '@mui/material';
import { Delete as DeleteIcon, Edit as EditIcon, Add as AddIcon, Refresh as RefreshIcon } from '@mui/icons-material';
import * as api from '../services/api';
import { EtcdKeyResponse } from '../types/api';

// 刷新间隔选项（秒）
const refreshIntervalOptions = [
  { value: 0, label: '手动刷新' },
  { value: 1, label: '1秒' },
  { value: 5, label: '5秒' },
  { value: 30, label: '30秒' },
];

// 辅助函数：截断文本
const truncateText = (text: string | undefined | null, maxLength: number = 50): string => {
  if (text === undefined || text === null) {
    return '(无值)';
  }
  if (text.length <= maxLength) {
    return text;
  }
  return text.slice(0, maxLength) + '...';
};

// 自定义样式的 ListItem
const StyledListItem = styled(ListItem)(({ theme }) => ({
  position: 'relative',
  padding: theme.spacing(1, 2),
  '&:hover': {
    backgroundColor: theme.palette.action.hover,
    '& .action-buttons': {
      visibility: 'visible',
      opacity: 1,
    },
  },
  '& .action-buttons': {
    position: 'absolute',
    right: theme.spacing(2),
    top: '50%',
    transform: 'translateY(-50%)',
    visibility: 'hidden',
    opacity: 0,
    transition: 'all 0.2s ease-in-out',
    display: 'flex',
    zIndex: 2,
  },
  '& .MuiListItemText-root': {
    marginRight: theme.spacing(10), // 为按钮预留空间
    width: '100%',
  },
  '& .MuiListItemText-secondary': {
    whiteSpace: 'pre-line',
    overflow: 'hidden',
    textOverflow: 'ellipsis',
    display: '-webkit-box',
    WebkitLineClamp: 2,
    WebkitBoxOrient: 'vertical',
  },
}));

const HomePage = () => {
  const [keys, setKeys] = useState<EtcdKeyResponse[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [searchPrefix, setSearchPrefix] = useState<string>('');
  const [editKey, setEditKey] = useState<EtcdKeyResponse | null>(null);
  const [editValue, setEditValue] = useState('');
  const [snackbar, setSnackbar] = useState<{ open: boolean; message: string }>({ open: false, message: '' });
  const [showAddDialog, setShowAddDialog] = useState(false);
  const [newKey, setNewKey] = useState('');
  const [newValue, setNewValue] = useState('');
  const [refreshInterval, setRefreshInterval] = useState<number>(5);
  const [isRefreshing, setIsRefreshing] = useState(false);
  const lastRefreshTime = useRef<number>(0);
  const refreshTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const loadKeys = useCallback(async () => {
    try {
      setIsRefreshing(true);
      const response = await api.getKeys(searchPrefix);
      setKeys(response.keys);
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load keys');
    } finally {
      setLoading(false);
      setIsRefreshing(false);
      lastRefreshTime.current = Date.now();
    }
  }, [searchPrefix]);

  const handleRefreshIntervalChange = (event: SelectChangeEvent<number>) => {
    const newInterval = Number(event.target.value);
    setRefreshInterval(newInterval);
  };

  const handleRefresh = () => {
    loadKeys();
  };

  // 使用 useEffect 处理自动刷新
  useEffect(() => {
    if (refreshInterval > 0) {
      const scheduleNextRefresh = () => {
        const now = Date.now();
        const timeSinceLastRefresh = now - lastRefreshTime.current;
        const nextRefreshDelay = Math.max(0, refreshInterval * 1000 - timeSinceLastRefresh);
        
        refreshTimeoutRef.current = setTimeout(() => {
          loadKeys();
          scheduleNextRefresh();
        }, nextRefreshDelay);
      };

      scheduleNextRefresh();
    }

    return () => {
      if (refreshTimeoutRef.current) {
        clearTimeout(refreshTimeoutRef.current);
      }
    };
  }, [refreshInterval, loadKeys]);

  // 初始加载
  useEffect(() => {
    loadKeys();
  }, [loadKeys]);

  // 处理编辑
  const handleEdit = async (key: EtcdKeyResponse) => {
    try {
      // 首先设置 editKey 打开对话框
      setEditKey(key);
      
      // 从后端重新获取最新的值
      const latestKey = await api.getKey(key.key);
      
      // 更新编辑框的值为最新的值
      setEditValue(latestKey.value);
    } catch (err) {
      // 如果获取失败，使用缓存的值并显示错误
      setEditValue(key.value);
      setError(err instanceof Error ? err.message : '获取最新值失败');
    }
  };

  // 处理删除
  const handleDelete = async (key: string) => {
    try {
      await api.deleteKey(key);
      await loadKeys();
      setSnackbar({ open: true, message: '删除成功' });
    } catch (err) {
      setError(err instanceof Error ? err.message : '删除失败');
    }
  };

  // 处理保存
  const handleSave = async () => {
    if (!editKey) return;

    try {
      await api.setKey(editKey.key, editValue);
      setEditKey(null);
      await loadKeys();
      setSnackbar({ open: true, message: '保存成功' });
    } catch (err) {
      setError(err instanceof Error ? err.message : '保存失败');
    }
  };

  // 处理添加新键值对
  const handleAdd = async () => {
    if (!newKey.trim()) {
      setSnackbar({ open: true, message: '键不能为空' });
      return;
    }

    try {
      await api.setKey(newKey, newValue);
      setShowAddDialog(false);
      setNewKey('');
      setNewValue('');
      await loadKeys();
      setSnackbar({ open: true, message: '添加成功' });
    } catch (err) {
      console.error('Failed to add key:', err);
      setSnackbar({ open: true, message: '添加失败' });
    }
  };

  return (
    <Container maxWidth="lg" sx={{ mt: 4, mb: 4 }}>
      <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', mb: 2 }}>
        <TextField
          label="Search Prefix"
          variant="outlined"
          size="small"
          value={searchPrefix}
          onChange={(e) => setSearchPrefix(e.target.value)}
          sx={{ width: 300 }}
        />
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 2 }}>
          <FormControl size="small" sx={{ minWidth: 120 }}>
            <InputLabel>Auto Refresh</InputLabel>
            <Select
              value={refreshInterval}
              label="Auto Refresh"
              onChange={handleRefreshIntervalChange}
            >
              {refreshIntervalOptions.map((option) => (
                <MenuItem key={option.value} value={option.value}>
                  {option.label}
                </MenuItem>
              ))}
            </Select>
          </FormControl>
          <Chip
            icon={<RefreshIcon />}
            label="Refresh"
            onClick={handleRefresh}
            color="primary"
            variant="outlined"
            sx={{ 
              animation: isRefreshing ? 'pulse 1s infinite' : 'none',
              '&:hover': {
                backgroundColor: 'primary.light',
                color: 'white'
              }
            }}
          />
        </Box>
      </Box>

      {loading && keys.length === 0 ? (
        <Box sx={{ display: 'flex', justifyContent: 'center', mt: 4 }}>
          <CircularProgress />
        </Box>
      ) : error ? (
        <Alert severity="error" sx={{ mt: 2 }}>
          {error}
        </Alert>
      ) : (
        <List>
          {keys.map((kv) => (
            <StyledListItem key={kv.key}>
              <ListItemText
                primary={truncateText(kv.key, 100)}
                secondary={truncateText(kv.value, 200)}
              />
              <Box className="action-buttons">
                <IconButton onClick={() => handleEdit(kv)}>
                  <EditIcon />
                </IconButton>
                <IconButton onClick={() => handleDelete(kv.key)}>
                  <DeleteIcon />
                </IconButton>
              </Box>
            </StyledListItem>
          ))}
        </List>
      )}

      {/* 编辑对话框 */}
      <Dialog open={editKey !== null} onClose={() => setEditKey(null)} maxWidth="md" fullWidth>
        <DialogTitle>编辑键值对</DialogTitle>
        <DialogContent>
          <Box sx={{ pt: 1 }}>
            <Typography variant="subtitle1" gutterBottom>
              键: {editKey?.key}
            </Typography>
            <TextField
              fullWidth
              multiline
              rows={4}
              value={editValue}
              onChange={(e) => setEditValue(e.target.value)}
              variant="outlined"
              label="值"
            />
          </Box>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setEditKey(null)}>取消</Button>
          <Button onClick={handleSave} variant="contained" color="primary">
            保存
          </Button>
        </DialogActions>
      </Dialog>

      {/* 消息提示 */}
      <Snackbar
        open={snackbar.open}
        autoHideDuration={3000}
        onClose={() => setSnackbar({ ...snackbar, open: false })}
        message={snackbar.message}
      />

      {/* 添加键值对的浮动按钮 */}
      <Fab
        color="primary"
        sx={{
          position: 'fixed',
          bottom: 24,
          right: 24,
        }}
        onClick={() => setShowAddDialog(true)}
      >
        <AddIcon />
      </Fab>

      {/* 添加键值对的对话框 */}
      <Dialog 
        open={showAddDialog} 
        onClose={() => setShowAddDialog(false)}
        maxWidth="md"
        fullWidth
      >
        <DialogTitle>添加键值对</DialogTitle>
        <DialogContent>
          <Box sx={{ pt: 1 }}>
            <TextField
              fullWidth
              label="键"
              value={newKey}
              onChange={(e) => setNewKey(e.target.value)}
              variant="outlined"
              sx={{ mb: 2 }}
              placeholder="例如: /smg/config/service_info"
            />
            <TextField
              fullWidth
              label="值"
              value={newValue}
              onChange={(e) => setNewValue(e.target.value)}
              variant="outlined"
              multiline
              minRows={4}
              maxRows={12}
            />
          </Box>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setShowAddDialog(false)}>取消</Button>
          <Button onClick={handleAdd} variant="contained" color="primary">
            添加
          </Button>
        </DialogActions>
      </Dialog>
    </Container>
  );
};

// 添加脉冲动画样式
const pulseKeyframes = `
@keyframes pulse {
  0% {
    opacity: 0.4;
  }
  50% {
    opacity: 1;
  }
  100% {
    opacity: 0.4;
  }
}
`;

// 添加样式到文档
const style = document.createElement('style');
style.textContent = pulseKeyframes;
document.head.appendChild(style);

export default HomePage; 