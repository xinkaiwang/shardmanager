import React, { useState, useEffect, useCallback, useMemo } from 'react';
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
  LinearProgress,
} from '@mui/material';
import * as api from '../services/api';
import { WorkerVm, AssignmentVm } from '../types/api';

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
  { value: 1, label: '1秒' },
  { value: 5, label: '5秒' },
  { value: 30, label: '30秒' },
  { value: 120, label: '2分钟' },
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

/**
 * 比较两个对象的内容是否相等
 * 用于避免不必要的重渲染
 */
const isEqual = <T extends Record<string, any>>(obj1: T, obj2: T): boolean => {
  return JSON.stringify(obj1) === JSON.stringify(obj2);
};

/**
 * 分片项组件 - 使用React.memo避免不必要的重渲染
 */
const AssignmentItem = React.memo(({ assignment }: { assignment: AssignmentVm }) => {
  return (
    <div>
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
  );
}, (prevProps, nextProps) => {
  // 自定义比较函数，只比较关键字段
  return (
    prevProps.assignment.current_state === nextProps.assignment.current_state &&
    prevProps.assignment.target_state === nextProps.assignment.target_state &&
    prevProps.assignment.shard_id === nextProps.assignment.shard_id &&
    prevProps.assignment.replica_idx === nextProps.assignment.replica_idx
  );
});

/**
 * 工作节点卡片组件 - 使用React.memo避免不必要的重渲染
 */
const WorkerCard = React.memo(({ 
  worker, 
  isSelected, 
  onSelect 
}: { 
  worker: WorkerVm; 
  isSelected: boolean;
  onSelect: (id: string) => void;
}) => {
  const handleClick = useCallback(() => {
    onSelect(worker.worker_full_id);
  }, [worker.worker_full_id, onSelect]);
  
  return (
    <Card 
      sx={{ 
        height: '350px', // 固定卡片高度
        display: 'flex',
        flexDirection: 'column',
        boxSizing: 'border-box',
        border: '2px solid',
        borderColor: isSelected ? '#1976d2' : 'transparent',
        transition: 'border-color 0.2s ease',
        cursor: 'pointer',
        '&:hover': {
          boxShadow: 3
        }
      }}
      onClick={handleClick}
    >
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
      <CardContent sx={{ flex: 1, overflow: 'hidden', display: 'flex', flexDirection: 'column' }}>
        <Typography variant="subtitle1" gutterBottom>
          分配任务：{worker.assignments?.length || 0}
        </Typography>
        {worker.assignments && worker.assignments.length > 0 ? (
          <List 
            disablePadding 
            sx={{ 
              flex: 1, 
              overflow: 'auto', 
              '&::-webkit-scrollbar': {
                width: '6px',
              },
              '&::-webkit-scrollbar-thumb': {
                backgroundColor: 'rgba(0,0,0,0.2)',
                borderRadius: '3px',
              },
            }}
          >
            {worker.assignments.map((assignment) => (
              <AssignmentItem key={assignment.assignment_id} assignment={assignment} />
            ))}
          </List>
        ) : (
          <Typography variant="body2" color="textSecondary">
            无分配任务
          </Typography>
        )}
      </CardContent>
    </Card>
  );
}, (prevProps, nextProps) => {
  // 自定义比较函数，判断工作节点和选中状态是否变化
  const workersEqual = isEqual(prevProps.worker, nextProps.worker);
  const selectionEqual = prevProps.isSelected === nextProps.isSelected;
  return workersEqual && selectionEqual;
});

/**
 * 页面标题栏组件 - 使用React.memo避免不必要的重渲染
 */
const PageHeader = React.memo(({
  refreshInterval,
  onRefreshIntervalChange,
  onRefresh
}: {
  refreshInterval: number;
  onRefreshIntervalChange: (event: SelectChangeEvent<number>) => void;
  onRefresh: () => void;
}) => {
  return (
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
            onChange={onRefreshIntervalChange}
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
          onClick={onRefresh}
          color="primary" 
          variant="outlined"
          sx={{ cursor: 'pointer' }}
        />
      </Box>
    </Box>
  );
});

export default function WorkersPage() {
  // 状态定义
  const [workers, setWorkers] = useState<WorkerVm[]>([]);
  const [initialLoading, setInitialLoading] = useState(true); // 初始加载状态
  const [refreshing, setRefreshing] = useState(false); // 刷新中状态
  const [error, setError] = useState<string | null>(null);
  const [searchText, setSearchText] = useState('');
  const [refreshInterval, setRefreshInterval] = useState<number>(30); // 默认30秒
  const [selectedWorkerId, setSelectedWorkerId] = useState<string | null>(null); // 当前选中的工作节点ID

  // 处理工作节点选择
  const handleSelectWorker = useCallback((workerId: string) => {
    setSelectedWorkerId(prev => prev === workerId ? null : workerId);
  }, []);

  /**
   * 智能更新工作节点数据
   * 保持未变化节点的引用相等性，减少不必要的重渲染
   */
  const updateWorkers = useCallback((newWorkers: WorkerVm[], prevWorkers: WorkerVm[]) => {
    // 如果数组长度或工作节点ID集合不同，使用新数组
    if (prevWorkers.length !== newWorkers.length) {
      return newWorkers;
    }
    
    // 检查工作节点ID集合是否相同
    const prevIds = new Set(prevWorkers.map(w => w.worker_full_id));
    const newIds = new Set(newWorkers.map(w => w.worker_full_id));
    
    if (prevIds.size !== newIds.size) {
      return newWorkers;
    }
    
    for (const id of prevIds) {
      if (!newIds.has(id)) {
        return newWorkers; // ID集合不同，使用新数组
      }
    }
    
    // ID集合相同，智能更新保持引用相等性
    return prevWorkers.map(prevWorker => {
      const newWorker = newWorkers.find(w => w.worker_full_id === prevWorker.worker_full_id);
      if (!newWorker) {
        return prevWorker; // 应该不会发生，但以防万一
      }
      
      // 如果工作节点内容没有变化，保持原引用
      if (isEqual(prevWorker, newWorker)) {
        return prevWorker;
      }
      
      return newWorker;
    });
  }, []);

  // 加载数据
  const loadData = useCallback(async (isInitialLoad = false) => {
    try {
      if (isInitialLoad) {
        setInitialLoading(true);
      } else {
        setRefreshing(true);
      }
      setError(null);
      
      const stateData = await api.getState();
      const newWorkers = stateData.workers || [];
      
      setWorkers(prev => updateWorkers(newWorkers, prev));
    } catch (err) {
      console.error('Failed to load state:', err);
      setError('加载工作节点状态失败');
    } finally {
      if (isInitialLoad) {
        setInitialLoading(false);
      } else {
        setRefreshing(false);
      }
    }
  }, [updateWorkers]);

  // 处理刷新间隔变更
  const handleRefreshIntervalChange = useCallback((event: SelectChangeEvent<number>) => {
    const newInterval = Number(event.target.value);
    setRefreshInterval(newInterval);
  }, []);

  // 初始加载
  useEffect(() => {
    loadData(true);
  }, [loadData]);

  // 设置定时刷新
  useEffect(() => {
    // 如果刷新间隔为0，则不设置定时器
    if (refreshInterval === 0) {
      return;
    }

    // 设置定时刷新
    const intervalId = setInterval(() => {
      loadData(false);
    }, refreshInterval * 1000);

    return () => clearInterval(intervalId);
  }, [refreshInterval, loadData]);

  // 过滤工作节点
  const filteredWorkers = useMemo(() => {
    return workers.filter(worker => 
      worker.worker_full_id.toLowerCase().includes(searchText.toLowerCase())
    );
  }, [workers, searchText]);

  // 处理手动刷新
  const handleManualRefresh = useCallback(() => {
    loadData(false);
  }, [loadData]);

  // 处理搜索文本变更
  const handleSearchChange = useCallback((e: React.ChangeEvent<HTMLInputElement>) => {
    setSearchText(e.target.value);
  }, []);

  return (
    <div style={{ position: 'relative' }}>
      {/* 进度条 - 始终存在但透明度变化，避免布局变化导致的闪烁 */}
      <Box 
        sx={{ 
          height: '4px', // 固定高度
          width: '100%', 
          position: 'absolute', 
          top: 0, 
          left: 0, 
          zIndex: 1000,
          overflow: 'hidden' // 确保内容不溢出
        }}
      >
        <LinearProgress 
          sx={{ 
            opacity: refreshing ? 1 : 0,
            transition: 'opacity 0.2s ease',
            height: '100%'
          }} 
        />
      </Box>
      
      {/* 提取为记忆化组件的页面标题栏 */}
      <PageHeader 
        refreshInterval={refreshInterval}
        onRefreshIntervalChange={handleRefreshIntervalChange}
        onRefresh={handleManualRefresh}
      />
      
      {/* 搜索框 - 单独处理以保持状态 */}
      <TextField
        fullWidth
        label="搜索工作节点"
        variant="outlined"
        margin="normal"
        value={searchText}
        onChange={handleSearchChange}
        sx={{ mb: 3 }}
      />

      {error && (
        <Alert severity="error" sx={{ mb: 2 }}>
          {error}
        </Alert>
      )}

      {initialLoading ? (
        <Box display="flex" justifyContent="center" p={4}>
          <CircularProgress />
        </Box>
      ) : (
        <Grid container spacing={3}>
          {filteredWorkers.length > 0 ? (
            filteredWorkers.map((worker) => (
              <Grid item xs={12} md={6} lg={4} key={worker.worker_full_id}>
                <WorkerCard 
                  worker={worker} 
                  isSelected={selectedWorkerId === worker.worker_full_id}
                  onSelect={handleSelectWorker}
                />
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