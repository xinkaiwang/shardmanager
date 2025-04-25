import { useState, useEffect } from 'react';
import {
  Typography,
  Box,
  Paper,
  TextField,
  Grid,
  Card,
  CardContent,
  CardHeader,
  Chip,
  List,
  ListItem,
  ListItemText,
  Divider,
  CircularProgress,
  Alert,
  FormControl,
  InputLabel,
  Select,
  MenuItem,
  SelectChangeEvent,
} from '@mui/material';
import * as api from '../services/api';
import { WorkerVm } from '../types/api';

// 状态到颜色的映射
const stateColorMap: Record<string, string> = {
  'UNASSIGNED': 'default',
  'ASSIGNED': 'primary',
  'STARTING': 'warning',
  'RUNNING': 'success',
  'STOPPING': 'warning',
  'STOPPED': 'error',
  'FAILED': 'error',
};

// 刷新间隔选项（秒）
const refreshIntervalOptions = [
  { value: 0, label: '手动刷新' },
  { value: 5, label: '5秒' },
  { value: 10, label: '10秒' },
  { value: 30, label: '30秒' },
  { value: 60, label: '1分钟' },
  { value: 300, label: '5分钟' },
];

// 状态文本格式化
const formatState = (state: string): string => {
  return state.charAt(0) + state.slice(1).toLowerCase();
};

// 格式化工作节点ID
const formatWorkerId = (workerId: string): string => {
  // 截取最后部分
  const parts = workerId.split('/');
  return parts[parts.length - 1];
};

export default function WorkersPage() {
  const [workers, setWorkers] = useState<WorkerVm[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [searchText, setSearchText] = useState('');
  const [refreshInterval, setRefreshInterval] = useState<number>(30); // 默认30秒

  // 加载数据
  const loadData = async () => {
    try {
      setLoading(true);
      setError(null);
      const stateData = await api.getState();
      setWorkers(stateData.workers || []);
    } catch (err) {
      console.error('Failed to load state:', err);
      setError('加载工作节点状态失败');
    } finally {
      setLoading(false);
    }
  };

  // 处理刷新间隔变更
  const handleRefreshIntervalChange = (event: SelectChangeEvent<number>) => {
    const newInterval = Number(event.target.value);
    setRefreshInterval(newInterval);
  };

  // 初始加载和设置刷新
  useEffect(() => {
    loadData();

    // 如果刷新间隔为0，则不设置定时器
    if (refreshInterval === 0) {
      return;
    }

    // 设置定时刷新
    const intervalId = setInterval(() => {
      loadData();
    }, refreshInterval * 1000);

    return () => clearInterval(intervalId);
  }, [refreshInterval]); // 当刷新间隔变化时重新设置定时器

  // 过滤工作节点
  const filteredWorkers = workers.filter(worker => 
    worker.worker_full_id.toLowerCase().includes(searchText.toLowerCase())
  );

  return (
    <div>
      <Box display="flex" justifyContent="space-between" alignItems="center" mb={3}>
        <Typography variant="h4" component="h1">
          工作节点状态
        </Typography>
        
        <Box display="flex" alignItems="center" gap={2}>
          <FormControl variant="outlined" size="small" sx={{ minWidth: 150 }}>
            <InputLabel id="refresh-interval-label">刷新间隔</InputLabel>
            <Select
              labelId="refresh-interval-label"
              value={refreshInterval}
              onChange={handleRefreshIntervalChange}
              label="刷新间隔"
            >
              {refreshIntervalOptions.map(option => (
                <MenuItem key={option.value} value={option.value}>
                  {option.label}
                </MenuItem>
              ))}
            </Select>
          </FormControl>
          
          <Chip 
            label="刷新" 
            onClick={loadData} 
            color="primary" 
            variant="outlined"
            sx={{ cursor: 'pointer' }}
          />
        </Box>
      </Box>
      
      {/* 搜索框 */}
      <TextField
        fullWidth
        label="搜索工作节点"
        variant="outlined"
        margin="normal"
        value={searchText}
        onChange={(e) => setSearchText(e.target.value)}
        sx={{ mb: 3 }}
      />

      {error && (
        <Alert severity="error" sx={{ mb: 2 }}>
          {error}
        </Alert>
      )}

      {loading ? (
        <Box display="flex" justifyContent="center" p={4}>
          <CircularProgress />
        </Box>
      ) : (
        <Grid container spacing={3}>
          {filteredWorkers.length > 0 ? (
            filteredWorkers.map((worker) => (
              <Grid item xs={12} md={6} lg={4} key={worker.worker_full_id}>
                <Card sx={{ height: '100%' }}>
                  <CardHeader
                    title={formatWorkerId(worker.worker_full_id)}
                    subheader={`工作节点 ID: ${worker.worker_full_id}`}
                    titleTypographyProps={{ variant: 'h6' }}
                    subheaderTypographyProps={{ 
                      variant: 'body2',
                      style: { 
                        whiteSpace: 'nowrap',
                        overflow: 'hidden',
                        textOverflow: 'ellipsis'
                      }
                    }}
                  />
                  <CardContent>
                    <Typography variant="subtitle1" gutterBottom>
                      分配任务：{worker.assignments?.length || 0}
                    </Typography>
                    {worker.assignments && worker.assignments.length > 0 ? (
                      <List disablePadding>
                        {worker.assignments.map((assignment) => (
                          <div key={assignment.assignment_id}>
                            <ListItem disablePadding sx={{ py: 1 }}>
                              <ListItemText
                                primary={
                                  <Box display="flex" justifyContent="space-between" alignItems="center">
                                    <Typography variant="body2">
                                      分片: {assignment.shard_id} (副本 {assignment.replica_idx})
                                    </Typography>
                                    <Chip 
                                      label={formatState(assignment.current_state)} 
                                      size="small"
                                      color={stateColorMap[assignment.current_state] as any}
                                      sx={{ ml: 1 }}
                                    />
                                  </Box>
                                }
                                secondary={
                                  <Typography variant="caption" color="textSecondary">
                                    目标状态: {formatState(assignment.target_state)}
                                  </Typography>
                                }
                              />
                            </ListItem>
                            <Divider component="li" />
                          </div>
                        ))}
                      </List>
                    ) : (
                      <Typography variant="body2" color="textSecondary">
                        无分配任务
                      </Typography>
                    )}
                  </CardContent>
                </Card>
              </Grid>
            ))
          ) : (
            <Grid item xs={12}>
              <Paper sx={{ p: 3, textAlign: 'center' }}>
                <Typography variant="body1">
                  {searchText ? '没有找到匹配的工作节点' : '暂无工作节点数据'}
                </Typography>
              </Paper>
            </Grid>
          )}
        </Grid>
      )}
    </div>
  );
} 