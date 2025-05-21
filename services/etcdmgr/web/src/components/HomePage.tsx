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
  borderRadius: theme.shape.borderRadius,
  marginBottom: theme.spacing(1),
  transition: 'background-color 0.2s ease',
  '&:hover': {
    backgroundColor: theme.palette.action.hover,
  },
  '&:hover .action-buttons': {
    opacity: 1,
  },
  '& .action-buttons': {
    opacity: 0,
    transition: 'opacity 0.2s ease',
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
  // 分页相关状态
  const [nextToken, setNextToken] = useState<string | undefined>(undefined);
  const [hasMore, setHasMore] = useState(true);
  const [loadingMore, setLoadingMore] = useState(false);
  
  const lastRefreshTime = useRef<number>(0);
  const refreshTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  // 添加防抖定时器引用
  const searchDebounceRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  // 使用ref存储当前的searchPrefix值，避免依赖项循环
  const searchPrefixRef = useRef<string>(searchPrefix);
  
  // 使用ref存储当前的nextToken值，避免依赖项循环
  const nextTokenRef = useRef<string | undefined>(nextToken);
  
  // 存储最新的loadKeys函数引用 - 修复：初始化为空函数
  const loadKeysRef = useRef<(reset: boolean) => Promise<void>>((reset: boolean) => Promise.resolve());

  // 定义一个标记来防止多次调用
  const isLoadingRef = useRef(false);

  // 在一个useEffect中设置loadKeysRef.current，确保只运行一次
  useEffect(() => {
    // 加载键（支持重置和加载更多）
    loadKeysRef.current = async (reset = true) => {
      // 如果已经在加载中，跳过
      if (isLoadingRef.current) {
        console.log('跳过加载：已有加载操作正在进行');
        return;
      }
      
      try {
        console.log(`开始加载，reset=${reset}, isRefreshing=${isRefreshing}, loadingMore=${loadingMore}`);
        isLoadingRef.current = true;
        
        // 重置模式或首次加载时
        if (reset) {
          setIsRefreshing(true);
          setLoadingMore(false);
          // 重置时清除nextToken
          setNextToken(undefined);
          nextTokenRef.current = undefined;
        } else {
          // 加载更多模式
          setLoadingMore(true);
        }

        // 每页加载 20 个条目
        const pageSize = 20;
        
        // 使用ref中的值，避免依赖变化
        const currentPrefix = searchPrefixRef.current;
        const tokenToUse = reset ? undefined : nextTokenRef.current;
        
        console.log('加载键值对:', {
          prefix: currentPrefix,
          pageSize,
          reset,
          hasNextToken: !!tokenToUse
        });
        
        // 加载键
        const response = await api.getKeys(
          currentPrefix, 
          pageSize, 
          tokenToUse
        );
        
        console.log('API 响应:', {
          keyCount: response.keys.length,
          hasNextToken: !!response.nextToken
        });
        
        // 更新状态
        if (reset) {
          setKeys(response.keys);
        } else {
          // 追加到当前列表
          setKeys(prevKeys => [...prevKeys, ...response.keys]);
        }
        
        // 更新分页状态
        setNextToken(response.nextToken);
        nextTokenRef.current = response.nextToken;
        setHasMore(!!response.nextToken);
        setError(null);
      } catch (err) {
        setError(err instanceof Error ? err.message : 'Failed to load keys');
      } finally {
        isLoadingRef.current = false;
        setLoading(false);
        setIsRefreshing(false);
        setLoadingMore(false);
        lastRefreshTime.current = Date.now();
      }
    };
    
    // 初始加载
    loadKeysRef.current(true);
  }, []);
  
  // 更新refs - 使用单独的useEffect，避免与loadKeys逻辑混在一起
  useEffect(() => {
    searchPrefixRef.current = searchPrefix;
  }, [searchPrefix]);
  
  useEffect(() => {
    nextTokenRef.current = nextToken;
  }, [nextToken]);

  // 加载更多
  const handleLoadMore = () => {
    if (loadingMore || !hasMore) return;
    loadKeysRef.current(false);
  };

  // 处理搜索
  const handleSearchChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const newValue = e.target.value;
    setSearchPrefix(newValue);
    
    // 清除之前的定时器
    if (searchDebounceRef.current) {
      clearTimeout(searchDebounceRef.current);
    }
    
    // 设置新的定时器，在用户停止输入300毫秒后执行搜索
    searchDebounceRef.current = setTimeout(() => {
      console.log('执行实时搜索:', newValue);
      loadKeysRef.current(true);
    }, 100);
  };

  const handleSearchSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    // 清除可能存在的定时器
    if (searchDebounceRef.current) {
      clearTimeout(searchDebounceRef.current);
      searchDebounceRef.current = null;
    }
    // 立即执行搜索
    loadKeysRef.current(true);
  };

  const handleRefreshIntervalChange = (event: SelectChangeEvent<number>) => {
    const newInterval = Number(event.target.value);
    setRefreshInterval(newInterval);
  };

  const handleRefresh = () => {
    loadKeysRef.current(true);
  };

  // 完全独立的自动刷新逻辑，使用intervalId
  useEffect(() => {
    // 清除现有定时器
    if (refreshTimeoutRef.current) {
      clearTimeout(refreshTimeoutRef.current);
      refreshTimeoutRef.current = null;
    }
    
    // 如果不需要自动刷新，直接返回
    if (refreshInterval <= 0) {
      console.log('关闭自动刷新');
      return;
    }
    
    console.log(`设置自动刷新：${refreshInterval}秒`);
    
    // 开始自动刷新
    const intervalId = setInterval(() => {
      // 避免重叠刷新
      if (isRefreshing || loadingMore || isLoadingRef.current) {
        console.log('跳过自动刷新：之前的操作尚未完成');
        return;
      }
      
      console.log(`执行自动刷新 (间隔: ${refreshInterval}秒)`);
      loadKeysRef.current(true);
    }, refreshInterval * 1000);
    
    // 清理函数
    return () => {
      console.log('清理自动刷新定时器');
      clearInterval(intervalId);
    };
  }, [refreshInterval]);

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
      await loadKeysRef.current(true);
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
      await loadKeysRef.current(true);
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
      await loadKeysRef.current(true);
      setSnackbar({ open: true, message: '添加成功' });
    } catch (err) {
      console.error('Failed to add key:', err);
      setSnackbar({ open: true, message: '添加失败' });
    }
  };

  // 清理函数 - 确保在组件卸载时清除所有定时器
  useEffect(() => {
    return () => {
      // 清除刷新定时器
      if (refreshTimeoutRef.current) {
        clearTimeout(refreshTimeoutRef.current);
        refreshTimeoutRef.current = null;
      }
      
      // 清除搜索防抖定时器
      if (searchDebounceRef.current) {
        clearTimeout(searchDebounceRef.current);
        searchDebounceRef.current = null;
      }
    };
  }, []);

  return (
    <Container maxWidth="lg" sx={{ 
      mt: 4, 
      mb: 4,
      display: 'flex',
      flexDirection: 'column',
      height: 'calc(100vh - 64px)' // 减去顶部可能的AppBar高度
    }}>
      <Paper sx={{ 
        p: 2, 
        display: 'flex', 
        flexDirection: 'column',
        flexGrow: 1,
        height: '100%'
      }}>
        <Box sx={{ display: 'flex', mb: 3, alignItems: 'center' }}>
          <Typography variant="h6" component="h2" sx={{ flexGrow: 1 }}>
            ETCD 键值浏览器
          </Typography>
          <Fab
            color="primary"
            size="small"
            onClick={() => setShowAddDialog(true)}
            sx={{ ml: 1 }}
          >
            <AddIcon />
          </Fab>
        </Box>

        <Box sx={{ mb: 3, display: 'flex', alignItems: 'center' }}>
          <form onSubmit={handleSearchSubmit} style={{ display: 'flex', width: '100%' }}>
            <TextField
              label="Search Prefix"
              variant="outlined"
              value={searchPrefix}
              onChange={handleSearchChange}
              fullWidth
              size="small"
              sx={{ mr: 2 }}
            />
          </form>

          <Box sx={{ display: 'flex', alignItems: 'center', whiteSpace: 'nowrap' }}>
            <Typography variant="body2" sx={{ mr: 1 }}>
              Auto Refresh
            </Typography>
            <FormControl size="small" sx={{ minWidth: 120 }}>
              <Select
                value={refreshInterval}
                onChange={handleRefreshIntervalChange}
                displayEmpty
              >
                {refreshIntervalOptions.map((option) => (
                  <MenuItem key={option.value} value={option.value}>
                    {option.label}
                  </MenuItem>
                ))}
              </Select>
            </FormControl>

            <IconButton 
              onClick={handleRefresh} 
              sx={{ ml: 1 }}
              disabled={isRefreshing}
            >
              <RefreshIcon 
                className={isRefreshing ? 'spin' : ''} 
                sx={{
                  animation: isRefreshing ? 'spin 1s linear infinite' : 'none',
                  '@keyframes spin': {
                    '0%': { transform: 'rotate(0deg)' },
                    '100%': { transform: 'rotate(360deg)' }
                  }
                }}
              />
            </IconButton>
          </Box>
        </Box>

        {error && (
          <Alert severity="error" sx={{ mb: 2 }}>
            {error}
          </Alert>
        )}

        {loading && keys.length === 0 ? (
          <Box sx={{ display: 'flex', justifyContent: 'center', p: 3 }}>
            <CircularProgress />
          </Box>
        ) : keys.length === 0 ? (
          <Typography variant="body1" align="center" sx={{ py: 4 }}>
            No keys found
          </Typography>
        ) : (
          <Box sx={{ 
            flexGrow: 1,
            display: 'flex',
            flexDirection: 'column',
            overflow: 'hidden',
            border: '1px solid rgba(0, 0, 0, 0.12)',
            borderRadius: 1,
            bgcolor: 'background.paper'
          }}>
            <List sx={{ 
              overflow: 'auto',
              flexGrow: 1,
              maxHeight: '100%'
            }}>
              {keys.map((kv) => (
                <StyledListItem key={kv.key} divider>
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
            
            {/* 加载更多按钮 */}
            {hasMore && (
              <Box sx={{ textAlign: 'center', py: 2 }}>
                <Button 
                  variant="outlined" 
                  onClick={handleLoadMore} 
                  disabled={loadingMore}
                  startIcon={loadingMore ? <CircularProgress size={20} /> : null}
                >
                  {loadingMore ? 'Loading more...' : 'Load More'}
                </Button>
              </Box>
            )}
          </Box>
        )}
      </Paper>

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
        autoHideDuration={6000}
        onClose={() => setSnackbar({ ...snackbar, open: false })}
      >
        <Alert 
          onClose={() => setSnackbar({ ...snackbar, open: false })} 
          severity="success" 
          sx={{ width: '100%' }}
        >
          {snackbar.message}
        </Alert>
      </Snackbar>

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