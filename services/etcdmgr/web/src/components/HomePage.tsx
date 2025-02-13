import React, { useState, useEffect } from 'react';
import {
  Container,
  Typography,
  Box,
  Paper,
  TextField,
  IconButton,
  List,
  ListItem,
  ListItemText,
  ListItemSecondaryAction,
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
} from '@mui/material';
import { Delete as DeleteIcon, Edit as EditIcon, Add as AddIcon } from '@mui/icons-material';
import * as api from '../services/api';
import { EtcdKeyResponse } from '../types/api';

// 辅助函数：截断文本
const truncateText = (text: string, maxLength: number = 50) => {
  if (text.length <= maxLength) return text;
  return text.slice(0, maxLength) + '...';
};

// 自定义样式的 ListItem
const StyledListItem = styled(ListItem)(({ theme }) => ({
  position: 'relative',
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
    visibility: 'hidden',
    opacity: 0,
    transition: 'all 0.2s ease-in-out',
  },
  // 添加样式以处理长文本
  '& .MuiListItemText-secondary': {
    whiteSpace: 'pre-line',
    overflow: 'hidden',
    textOverflow: 'ellipsis',
    display: '-webkit-box',
    WebkitLineClamp: 2,
    WebkitBoxOrient: 'vertical',
  },
}));

export default function HomePage() {
  const [keys, setKeys] = useState<EtcdKeyResponse[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [searchPrefix, setSearchPrefix] = useState('');
  const [editKey, setEditKey] = useState<EtcdKeyResponse | null>(null);
  const [editValue, setEditValue] = useState('');
  const [showSnackbar, setShowSnackbar] = useState(false);
  const [snackbarMessage, setSnackbarMessage] = useState('');
  const [snackbarSeverity, setSnackbarSeverity] = useState<'success' | 'error'>('success');
  const [showAddDialog, setShowAddDialog] = useState(false);
  const [newKey, setNewKey] = useState('');
  const [newValue, setNewValue] = useState('');

  // 加载键值对列表
  const loadKeys = async () => {
    try {
      setLoading(true);
      const response = await api.getKeys(searchPrefix);
      setKeys(response.keys);
      setError(null);
    } catch (err) {
      setError('加载键值对列表失败');
      console.error('Failed to load keys:', err);
    } finally {
      setLoading(false);
    }
  };

  // 初始加载和搜索时重新加载
  useEffect(() => {
    loadKeys();
  }, [searchPrefix]);

  // 处理编辑
  const handleEdit = (key: EtcdKeyResponse) => {
    setEditKey(key);
    setEditValue(key.value);
  };

  // 保存编辑
  const handleSave = async () => {
    if (!editKey) return;

    try {
      await api.setKey(editKey.key, editValue);
      setEditKey(null);
      await loadKeys();
      showMessage('保存成功', 'success');
    } catch (err) {
      showMessage('保存失败', 'error');
      console.error('Failed to save key:', err);
    }
  };

  // 处理删除
  const handleDelete = async (key: string) => {
    try {
      await api.deleteKey(key);
      await loadKeys();
      showMessage('删除成功', 'success');
    } catch (err) {
      showMessage('删除失败', 'error');
      console.error('Failed to delete key:', err);
    }
  };

  // 显示消息
  const showMessage = (message: string, severity: 'success' | 'error') => {
    setSnackbarMessage(message);
    setSnackbarSeverity(severity);
    setShowSnackbar(true);
  };

  // 处理添加新键值对
  const handleAdd = async () => {
    if (!newKey.trim()) {
      showMessage('键不能为空', 'error');
      return;
    }

    try {
      await api.setKey(newKey, newValue);
      setShowAddDialog(false);
      setNewKey('');
      setNewValue('');
      await loadKeys();
      showMessage('添加成功', 'success');
    } catch (err) {
      showMessage('添加失败', 'error');
      console.error('Failed to add key:', err);
    }
  };

  return (
    <Box
      sx={{
        display: 'flex',
        flexDirection: 'column',
        height: '100vh',
        width: '100vw',
        overflow: 'hidden',
        bgcolor: 'background.default',
      }}
    >
      <Box
        sx={{
          flex: 1,
          display: 'flex',
          flexDirection: 'column',
          p: 3,
          overflow: 'hidden',
          alignItems: 'center',
        }}
      >
        <Paper
          elevation={2}
          sx={{
            flex: 1,
            display: 'flex',
            flexDirection: 'column',
            overflow: 'hidden',
            maxWidth: '1000px',
            width: '100%',
          }}
        >
          <Box sx={{ p: 3, borderBottom: 1, borderColor: 'divider' }}>
            <Typography variant="h4" component="h1" gutterBottom align="center">
              etcd 管理器
            </Typography>

            {/* 搜索框 */}
            <TextField
              fullWidth
              label="搜索前缀"
              value={searchPrefix}
              onChange={(e) => setSearchPrefix(e.target.value)}
              variant="outlined"
              size="small"
              sx={{ mt: 2 }}
            />
          </Box>

          <Box sx={{ flex: 1, overflow: 'auto', p: 2 }}>
            {/* 错误提示 */}
            {error && (
              <Alert severity="error" sx={{ mb: 2 }}>
                {error}
              </Alert>
            )}

            {/* 加载中 */}
            {loading ? (
              <Box display="flex" justifyContent="center" p={4}>
                <CircularProgress />
              </Box>
            ) : (
              /* 键值对列表 */
              <List>
                {keys.map((item) => (
                  <StyledListItem key={item.key}>
                    <ListItemText
                      primary={item.key}
                      secondary={`值: ${truncateText(item.value)} (版本: ${item.version})`}
                      secondaryTypographyProps={{
                        component: 'div',
                      }}
                    />
                    <Box className="action-buttons">
                      <IconButton size="small" onClick={() => handleEdit(item)}>
                        <EditIcon />
                      </IconButton>
                      <IconButton size="small" onClick={() => handleDelete(item.key)}>
                        <DeleteIcon />
                      </IconButton>
                    </Box>
                  </StyledListItem>
                ))}
                {keys.length === 0 && (
                  <ListItem>
                    <ListItemText primary="没有找到键值对" />
                  </ListItem>
                )}
              </List>
            )}
          </Box>
        </Paper>
      </Box>

      {/* 编辑对话框 */}
      <Dialog 
        open={!!editKey} 
        onClose={() => setEditKey(null)}
        maxWidth="md"
        fullWidth
      >
        <DialogTitle>编辑键值对</DialogTitle>
        <DialogContent>
          <Box sx={{ pt: 1 }}>
            <Typography variant="subtitle1" gutterBottom>
              键: {editKey?.key}
            </Typography>
            <TextField
              fullWidth
              label="值"
              value={editValue}
              onChange={(e) => setEditValue(e.target.value)}
              variant="outlined"
              multiline
              minRows={4}
              maxRows={12}
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

      {/* 提示消息 */}
      <Snackbar
        open={showSnackbar}
        autoHideDuration={3000}
        onClose={() => setShowSnackbar(false)}
      >
        <Alert severity={snackbarSeverity} onClose={() => setShowSnackbar(false)}>
          {snackbarMessage}
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
    </Box>
  );
} 