import React, { useState, useEffect, useCallback, useMemo } from 'react';
import {
  Typography,
  Box,
  Paper,
  TextField,
  Grid,
  Card,
  CardContent,
  Chip,
  Divider,
  CircularProgress,
  Alert,
  FormControl,
  InputLabel,
  Select,
  MenuItem,
  SelectChangeEvent,
  LinearProgress,
  Tooltip,
} from '@mui/material';
import * as api from '../services/api';
import { WorkerVm, AssignmentVm } from '../types/api';

// 状态到颜色的映射
const stateColorMap: Record<string, string> = {
  // 大写状态（兼容旧版）
  'UNASSIGNED': 'default',
  'ASSIGNED': 'primary',
  'STARTING': 'warning',
  'RUNNING': 'success',
  'STOPPING': 'warning',
  'STOPPED': 'error',
  'FAILED': 'error',
  // 小写状态（新版）
  'ready': 'info',
  'starting': 'warning',
  'stopping': 'warning',
  'failed': 'error',
  'offline': 'error',
  'adding': 'info',
  'dropping': 'warning',
  'dropped': 'default',
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
const formatState = (state: string | undefined): string => {
  if (!state) return '未知';
  return state.charAt(0) + state.slice(1).toLowerCase();
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
 * 单行显示每个分配任务，优化空间利用
 */
const AssignmentItem = React.memo(({ assignment }: { assignment: AssignmentVm }) => {
  // 获取状态对应的颜色
  const getStatusColor = () => {
    // 为"ready"状态使用自定义的浅绿色
    if (assignment.status === 'ready') {
      return 'success'; // 标准的绿色
    }
    return stateColorMap[assignment.status] || 'default';
  };
  
  // 获取自定义样式
  const getCustomStyle = () => {
    // 为"ready"状态使用自定义的浅绿色背景
    if (assignment.status === 'ready') {
      return { 
        bgcolor: '#e0f2f1', // 浅绿色背景
        color: '#00695c',   // 深绿色文字
        '& .MuiChip-label': { 
          px: 1,
          py: 0
        }
      };
    }
    
    return { 
      height: '20px', 
      fontSize: '0.7rem',
      '& .MuiChip-label': { 
        px: 1,
        py: 0
      }
    };
  };
  
  return (
    <Box 
      sx={{ 
        display: 'flex', 
        alignItems: 'center', 
        justifyContent: 'space-between',
        py: 0.75,
        px: 1,
        borderBottom: '1px solid rgba(0,0,0,0.08)',
        '&:last-child': {
          borderBottom: 'none',
        }
      }}
    >
      {/* 分片ID和副本索引，使用简洁格式 */}
      <Typography variant="body2" sx={{ fontWeight: 500, fontSize: '0.8rem' }}>
        {`${assignment.shard_id}:${assignment.replica_idx}`}
      </Typography>
      
      {/* 状态标签 */}
      <Chip 
        label={formatState(assignment.status)} 
        size="small"
        color={getStatusColor() as any}
        sx={getCustomStyle()}
      />
    </Box>
  );
}, (prevProps, nextProps) => {
  // 自定义比较函数，只比较关键字段
  return (
    prevProps.assignment.status === nextProps.assignment.status &&
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
  
  // 跟踪节点是否为新创建的状态
  const [isNew, setIsNew] = useState(false);
  
  // 检查和更新节点是否为新创建（小于60秒）
  useEffect(() => {
    if (!worker.worker_start_time_ms) return;
    
    // 初始检查
    const checkIfNew = () => {
      const currentTimeMs = Date.now();
      const workerAgeMs = currentTimeMs - worker.worker_start_time_ms;
      return workerAgeMs < 60 * 1000; // 小于60秒视为新节点
    };
    
    // 设置初始状态
    setIsNew(checkIfNew());
    
    // 如果是新节点，设置定时器在适当的时间后更新状态
    if (checkIfNew()) {
      const remainingTimeMs = 60 * 1000 - (Date.now() - worker.worker_start_time_ms);
      const timerId = setTimeout(() => {
        setIsNew(false);
      }, Math.max(0, remainingTimeMs));
      
      // 清理定时器
      return () => clearTimeout(timerId);
    }
  }, [worker.worker_start_time_ms]);
  
  // 用于强制刷新tooltip内容的状态
  const [ageUpdateTrigger, setAgeUpdateTrigger] = useState(0);
  
  // 定期更新年龄显示（每秒更新一次）
  useEffect(() => {
    if (!worker.worker_start_time_ms || !isNew) return;
    
    const intervalId = setInterval(() => {
      setAgeUpdateTrigger(prev => prev + 1);
    }, 1000);
    
    return () => clearInterval(intervalId);
  }, [worker.worker_start_time_ms, isNew]);
  
  // 获取格式化的创建时间
  const formattedCreationTime = useMemo(() => {
    if (!worker.worker_start_time_ms) return '';
    
    const date = new Date(worker.worker_start_time_ms);
    return date.toLocaleString('zh-CN', {
      year: 'numeric',
      month: '2-digit',
      day: '2-digit',
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit',
    });
  }, [worker.worker_start_time_ms]);
  
  // 获取节点已存在的时长
  const getNodeAge = useCallback(() => {
    if (!worker.worker_start_time_ms) return '';
    
    // ageUpdateTrigger用于强制函数重新计算
    void ageUpdateTrigger; // 使用void操作符"消费"变量以避免TS警告
    
    const ageMs = Date.now() - worker.worker_start_time_ms;
    
    // 小于1分钟显示秒数
    if (ageMs < 60 * 1000) {
      const seconds = Math.floor(ageMs / 1000);
      return `${seconds}秒`;
    }
    
    // 小于1小时显示分钟
    if (ageMs < 60 * 60 * 1000) {
      const minutes = Math.floor(ageMs / (60 * 1000));
      return `${minutes}分钟`;
    }
    
    // 小于1天显示小时
    if (ageMs < 24 * 60 * 60 * 1000) {
      const hours = Math.floor(ageMs / (60 * 60 * 1000));
      return `${hours}小时`;
    }
    
    // 大于1天显示天数
    const days = Math.floor(ageMs / (24 * 60 * 60 * 1000));
    return `${days}天`;
  }, [worker.worker_start_time_ms, ageUpdateTrigger]);
  
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
      {/* 卡片标题 - worker_id居中 */}
      <Box sx={{ textAlign: 'center', pt: 2, pb: 1 }}>
        <Typography variant="h6" component="h2">
          {worker.worker_id}
        </Typography>
      </Box>
      
      {/* 状态标签区 */}
      <Box 
        sx={{ 
          display: 'flex', 
          flexWrap: 'wrap', 
          justifyContent: 'center',
          gap: 1,
          px: 2,
          pb: 1
        }}
      >
        {/* 新节点标签 */}
        {isNew && (
          <Tooltip 
            title={
              <React.Fragment>
                <Typography variant="body2">创建时间: {formattedCreationTime}</Typography>
                <Typography variant="body2">已存在: {getNodeAge()}</Typography>
              </React.Fragment>
            }
            arrow
            placement="top"
          >
            <Chip 
              label="新" 
              size="small"
              sx={{ 
                fontSize: '0.75rem',
                bgcolor: '#e3f2fd', 
                color: '#0d47a1',
                fontWeight: 'bold',
                animation: 'pulse 2s infinite'
              }}
            />
          </Tooltip>
        )}
        
        {/* 在线状态 */}
        {worker.is_offline === 0 && (
          <Chip 
            label="在线" 
            size="small" 
            color="success" 
            sx={{ fontSize: '0.75rem' }}
          />
        )}
        
        {/* 离线状态 */}
        {worker.is_offline === 1 && (
          <Chip 
            label="离线" 
            size="small" 
            color="error" 
            sx={{ fontSize: '0.75rem' }}
          />
        )}
        
        {/* 关闭请求状态 */}
        {worker.is_shutdown_req === 1 && (
          <Chip 
            label="关机请求" 
            size="small" 
            color="warning" 
            sx={{ fontSize: '0.75rem' }}
          />
        )}
        
        {/* 耗尽状态 */}
        {worker.is_draining === 1 && (
          <Chip 
            label="正在排空" 
            size="small" 
            color="warning" 
            sx={{ fontSize: '0.75rem' }}
          />
        )}

        {/* 允许关机 - 罕见状态，只在允许时显示 */}
        {worker.is_shutdown_permitted === 1 && (
          <Chip 
            label="允许关机" 
            size="small" 
            color="success" 
            sx={{ fontSize: '0.75rem', bgcolor: '#81c784', color: '#1b5e20' }}
          />
        )}
      </Box>
      
      <Divider />
      
      <CardContent sx={{ flex: 1, overflow: 'hidden', display: 'flex', flexDirection: 'column', pt: 1 }}>
        {/* 任务列表标题和计数 */}
        <Box 
          sx={{ 
            display: 'flex', 
            justifyContent: 'space-between', 
            alignItems: 'center',
            mb: 1
          }}
        >
          <Typography variant="subtitle2">分配任务</Typography>
          <Typography variant="caption" color="text.secondary">
            {worker.assignments?.length || 0} 个
          </Typography>
        </Box>
        
        {worker.assignments && worker.assignments.length > 0 ? (
          <Box 
            sx={{ 
              flex: 1, 
              overflow: 'auto', 
              '&::-webkit-scrollbar': {
                width: '4px',
              },
              '&::-webkit-scrollbar-thumb': {
                backgroundColor: 'rgba(0,0,0,0.2)',
                borderRadius: '2px',
              },
            }}
          >
            {worker.assignments.map((assignment) => (
              <AssignmentItem 
                key={assignment.assignment_id} 
                assignment={assignment} 
              />
            ))}
          </Box>
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

  // 定义脉冲动画样式
  const pulseAnimation = `
    @keyframes pulse {
      0% {
        box-shadow: 0 0 0 0 rgba(13, 71, 161, 0.4);
      }
      70% {
        box-shadow: 0 0 0 6px rgba(13, 71, 161, 0);
      }
      100% {
        box-shadow: 0 0 0 0 rgba(13, 71, 161, 0);
      }
    }
  `;

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
    <div style={{ position: 'relative', paddingTop: 4 }}>
      {/* 添加脉冲动画样式 */}
      <style>{pulseAnimation}</style>
      
      {/* 进度条 - 使用绝对定位固定在页面顶部，完全脱离文档流 */}
      <Box
        sx={{
          position: 'fixed',
          top: 0,
          left: 0,
          right: 0,
          zIndex: 9999,
          height: '4px',
          visibility: refreshing ? 'visible' : 'hidden'
        }}
      >
        <LinearProgress sx={{ height: '100%' }} />
      </Box>
      
      {/* 页面标题栏 */}
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