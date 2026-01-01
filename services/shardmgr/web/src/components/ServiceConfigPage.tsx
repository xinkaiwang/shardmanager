import React, { useState, useEffect, useCallback, useMemo } from 'react';
import {
  Box,
  Typography,
  Button,
  TextField,
  Select,
  MenuItem,
  FormControl,
  Paper,
  Alert,
  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
  Accordion,
  AccordionSummary,
  AccordionDetails,
  InputAdornment,
  Chip,
} from '@mui/material';
import {
  Refresh as RefreshIcon,
  Save as SaveIcon,
  ExpandMore as ExpandMoreIcon,
  Settings as SettingsIcon,
} from '@mui/icons-material';
import * as api from '../services/api';

const ServiceConfigPage: React.FC = () => {
  const [config, setConfig] = useState<api.ServiceConfig>({});
  const [originalConfig, setOriginalConfig] = useState<api.ServiceConfig>({});
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);
  const [confirmDialogOpen, setConfirmDialogOpen] = useState(false);
  const [advancedMode, setAdvancedMode] = useState(false);
  const [showFullConfig, setShowFullConfig] = useState(false);

  const hasChanges = useMemo(() => {
    return JSON.stringify(config) !== JSON.stringify(originalConfig);
  }, [config, originalConfig]);

  // 计算配置差异
  const configDiff = useMemo(() => {
    const diff: Array<{ path: string; oldValue: any; newValue: any }> = [];
    
    const compareObjects = (obj1: any, obj2: any, path: string = '') => {
      const allKeys = new Set([...Object.keys(obj1 || {}), ...Object.keys(obj2 || {})]);
      
      allKeys.forEach(key => {
        const currentPath = path ? `${path}.${key}` : key;
        const val1 = obj1?.[key];
        const val2 = obj2?.[key];
        
        if (typeof val1 === 'object' && val1 !== null && typeof val2 === 'object' && val2 !== null) {
          compareObjects(val1, val2, currentPath);
        } else if (val1 !== val2) {
          diff.push({
            path: currentPath,
            oldValue: val1,
            newValue: val2,
          });
        }
      });
    };
    
    compareObjects(originalConfig, config);
    return diff;
  }, [config, originalConfig]);

  const loadConfig = useCallback(async () => {
    try {
      setLoading(true);
      setError(null);
      const cfg = await api.getServiceConfig();
      setConfig(cfg);
      setOriginalConfig(cfg);
    } catch (err) {
      console.error('Failed to load service config:', err);
      setError('加载配置失败');
    } finally {
      setLoading(false);
    }
  }, []);

  const saveConfig = useCallback(async () => {
    try {
      setSaving(true);
      setError(null);
      setSuccess(null);
      await api.setServiceConfig(config);
      setOriginalConfig(config);
      setSuccess('配置保存成功！');
      setConfirmDialogOpen(false);
      setTimeout(() => setSuccess(null), 3000);
    } catch (err) {
      console.error('Failed to save service config:', err);
      setError('保存配置失败');
      setConfirmDialogOpen(false);
    } finally {
      setSaving(false);
    }
  }, [config]);

  useEffect(() => {
    loadConfig();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const handleRefresh = () => {
    if (hasChanges) {
      if (window.confirm('有未保存的更改，确定要刷新吗？')) {
        loadConfig();
      }
    } else {
      loadConfig();
    }
  };

  const handleSaveClick = () => {
    setConfirmDialogOpen(true);
  };

  const updateShardConfig = (field: string, value: any) => {
    setConfig({
      ...config,
      shard_config: {
        ...config.shard_config,
        [field]: value,
      },
    });
  };

  const updateWorkerConfig = (field: string, value: any) => {
    setConfig({
      ...config,
      worker_config: {
        ...config.worker_config,
        [field]: value,
      },
    });
  };

  const updateSystemLimit = (field: string, value: any) => {
    setConfig({
      ...config,
      system_limit: {
        ...config.system_limit,
        [field]: value,
      },
    });
  };

  const updateCostFuncCfg = (field: string, value: any) => {
    setConfig({
      ...config,
      cost_func_cfg: {
        ...config.cost_func_cfg,
        [field]: value,
      },
    });
  };

  const updateSolverConfig = (solverType: string, field: string, value: any) => {
    setConfig({
      ...config,
      solver_config: {
        ...config.solver_config,
        [solverType]: {
          ...config.solver_config?.[solverType as keyof typeof config.solver_config],
          [field]: value,
        },
      },
    });
  };

  const updateDynamicThreshold = (field: string, value: any) => {
    setConfig({
      ...config,
      dynamic_threshold: {
        ...config.dynamic_threshold,
        [field]: value,
      },
    });
  };

  const updateFaultTolerance = (field: string, value: any) => {
    setConfig({
      ...config,
      fault_tolerance: {
        ...config.fault_tolerance,
        [field]: value,
      },
    });
  };

  return (
    <div>
      <Box display="flex" justifyContent="space-between" alignItems="center" mb={3}>
        <Typography variant="h4" component="h1">
          Service Config 管理
        </Typography>
        
        <Box display="flex" gap={2}>
          <Button
            variant="outlined"
            startIcon={<RefreshIcon />}
            onClick={handleRefresh}
            disabled={loading}
          >
            刷新
          </Button>
          <Button
            variant="outlined"
            startIcon={<SettingsIcon />}
            onClick={() => setAdvancedMode(!advancedMode)}
            disabled={loading}
          >
            {advancedMode ? '简单模式' : '高级模式'}
          </Button>
          <Button
            variant="contained"
            color="primary"
            startIcon={<SaveIcon />}
            onClick={handleSaveClick}
            disabled={loading || saving || !hasChanges}
          >
            保存
          </Button>
        </Box>
      </Box>

      {error && (
        <Alert severity="error" sx={{ mb: 2 }} onClose={() => setError(null)}>
          {error}
        </Alert>
      )}

      {success && (
        <Alert severity="success" sx={{ mb: 2 }} onClose={() => setSuccess(null)}>
          {success}
        </Alert>
      )}

      {hasChanges && (
        <Alert severity="warning" sx={{ mb: 2 }}>
          有未保存的更改
        </Alert>
      )}

      <Accordion defaultExpanded>
        <AccordionSummary expandIcon={<ExpandMoreIcon />}>
          <Typography variant="h6">📦 Shard Config</Typography>
        </AccordionSummary>
        <AccordionDetails>
          <Box sx={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
            {/* Move Policy */}
            <Box sx={{ display: 'flex', alignItems: 'center', gap: 2 }}>
              <Box sx={{ minWidth: 180 }}>
                <Typography variant="body2" fontWeight="bold">Move Policy</Typography>
              </Box>
              <FormControl size="small" sx={{ width: 250 }}>
                <Select
                  value={config.shard_config?.move_policy || 'start_before_kill'}
                  onChange={(e) => updateShardConfig('move_policy', e.target.value)}
                  disabled={loading}
                >
                  <MenuItem value="start_before_kill">Start Before Kill</MenuItem>
                  <MenuItem value="kill_before_start">Kill Before Start</MenuItem>
                  <MenuItem value="concurrent">Concurrent</MenuItem>
                </Select>
              </FormControl>
              <Box sx={{ display: 'flex', gap: 1, flex: 1 }}>
                <Chip label="默认: start_before_kill" size="small" variant="outlined" />
                <Typography variant="caption" color="text.secondary" sx={{ alignSelf: 'center' }}>
                  迁移策略
                </Typography>
              </Box>
            </Box>

            {/* Min Replica Count */}
            <Box sx={{ display: 'flex', alignItems: 'center', gap: 2 }}>
              <Box sx={{ minWidth: 180 }}>
                <Typography variant="body2" fontWeight="bold">Min Replica Count</Typography>
              </Box>
              <TextField
                size="small"
                type="number"
                sx={{ width: 250 }}
                value={config.shard_config?.min_replica_count ?? 1}
                onChange={(e) => updateShardConfig('min_replica_count', parseInt(e.target.value) || 1)}
                disabled={loading}
                InputProps={{
                  inputProps: { min: 1, max: 100 }
                }}
              />
              <Box sx={{ display: 'flex', gap: 1, flex: 1 }}>
                <Chip label="默认: 1" size="small" variant="outlined" />
                <Chip label="范围: 0-100" size="small" color="primary" variant="outlined" />
                <Typography variant="caption" color="text.secondary" sx={{ alignSelf: 'center' }}>
                  最小副本数
                </Typography>
              </Box>
            </Box>

            {/* Max Replica Count */}
            <Box sx={{ display: 'flex', alignItems: 'center', gap: 2 }}>
              <Box sx={{ minWidth: 180 }}>
                <Typography variant="body2" fontWeight="bold">Max Replica Count</Typography>
              </Box>
              <TextField
                size="small"
                type="number"
                sx={{ width: 250 }}
                value={config.shard_config?.max_replica_count ?? 10}
                onChange={(e) => updateShardConfig('max_replica_count', parseInt(e.target.value) || 10)}
                disabled={loading}
                InputProps={{
                  inputProps: { min: 1, max: 100 }
                }}
              />
              <Box sx={{ display: 'flex', gap: 1, flex: 1 }}>
                <Chip label="默认: 1" size="small" variant="outlined" />
                <Chip label="范围: >=Min Replica Count" size="small" color="primary" variant="outlined" />
                <Typography variant="caption" color="text.secondary" sx={{ alignSelf: 'center' }}>
                  最大副本数
                </Typography>
              </Box>
            </Box>
          </Box>
        </AccordionDetails>
      </Accordion>

      <Accordion defaultExpanded>
        <AccordionSummary expandIcon={<ExpandMoreIcon />}>
          <Typography variant="h6">👷 Worker Config</Typography>
        </AccordionSummary>
        <AccordionDetails>
          <Box sx={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
            {/* Max Assignment Count Per Worker */}
            <Box sx={{ display: 'flex', alignItems: 'center', gap: 2 }}>
              <Box sx={{ minWidth: 180 }}>
                <Typography variant="body2" fontWeight="bold">Max Assignments Per Worker</Typography>
              </Box>
              <TextField
                size="small"
                type="number"
                sx={{ width: 250 }}
                value={config.worker_config?.max_assignment_count_per_worker ?? 100}
                onChange={(e) => updateWorkerConfig('max_assignment_count_per_worker', parseInt(e.target.value) || 100)}
                disabled={loading}
                InputProps={{
                  inputProps: { min: 1, max: 1000 }
                }}
              />
              <Box sx={{ display: 'flex', gap: 1, flex: 1 }}>
                <Chip label="默认: 100" size="small" variant="outlined" />
                <Chip label="范围: 1-1000" size="small" color="primary" variant="outlined" />
                <Typography variant="caption" color="text.secondary" sx={{ alignSelf: 'center' }}>
                  每个 Worker 最大 Shard 数
                </Typography>
              </Box>
            </Box>

            {/* Offline Grace Period */}
            <Box sx={{ display: 'flex', alignItems: 'center', gap: 2 }}>
              <Box sx={{ minWidth: 180 }}>
                <Typography variant="body2" fontWeight="bold">Offline Grace Period</Typography>
              </Box>
              <TextField
                size="small"
                type="number"
                sx={{ width: 250 }}
                value={config.worker_config?.offline_grace_period_sec ?? 10}
                onChange={(e) => updateWorkerConfig('offline_grace_period_sec', parseInt(e.target.value) || 10)}
                disabled={loading}
                InputProps={{
                  endAdornment: <InputAdornment position="end">秒</InputAdornment>,
                  inputProps: { min: 0, max: 3600 }
                }}
              />
              <Box sx={{ display: 'flex', gap: 1, flex: 1 }}>
                <Chip label="默认: 10秒" size="small" variant="outlined" />
                <Chip label="范围: 0-3600秒" size="small" color="primary" variant="outlined" />
                <Typography variant="caption" color="text.secondary" sx={{ alignSelf: 'center' }}>
                  下线宽限期
                </Typography>
              </Box>
            </Box>
          </Box>
        </AccordionDetails>
      </Accordion>

      <Accordion defaultExpanded>
        <AccordionSummary expandIcon={<ExpandMoreIcon />}>
          <Typography variant="h6">🔧 System Limit</Typography>
        </AccordionSummary>
        <AccordionDetails>
          <Box sx={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
            {/* Max Shards Count Limit */}
            <Box sx={{ display: 'flex', alignItems: 'center', gap: 2 }}>
              <Box sx={{ minWidth: 180 }}>
                <Typography variant="body2" fontWeight="bold">Max Shards Count</Typography>
              </Box>
              <TextField
                size="small"
                type="number"
                sx={{ width: 250 }}
                value={config.system_limit?.max_shards_count_limit ?? 1000}
                onChange={(e) => updateSystemLimit('max_shards_count_limit', parseInt(e.target.value) || 1000)}
                disabled={loading}
                InputProps={{
                  inputProps: { min: 1, max: 100000 }
                }}
              />
              <Box sx={{ display: 'flex', gap: 1, flex: 1 }}>
                <Chip label="默认: 1000" size="small" variant="outlined" />
                <Chip label="推荐: >=1000" size="small" color="primary" variant="outlined" />
                <Typography variant="caption" color="text.secondary" sx={{ alignSelf: 'center' }}>
                  最大 Shard 数量
                </Typography>
              </Box>
            </Box>

            {/* Max Replica Count Limit */}
            <Box sx={{ display: 'flex', alignItems: 'center', gap: 2 }}>
              <Box sx={{ minWidth: 180 }}>
                <Typography variant="body2" fontWeight="bold">Max Replica Count</Typography>
              </Box>
              <TextField
                size="small"
                type="number"
                sx={{ width: 250 }}
                value={config.system_limit?.max_replica_count_limit ?? 1000}
                onChange={(e) => updateSystemLimit('max_replica_count_limit', parseInt(e.target.value) || 1000)}
                disabled={loading}
              />
              <Box sx={{ display: 'flex', gap: 1, flex: 1 }}>
                <Chip label="默认: 1000" size="small" variant="outlined" />
                <Chip label="推荐: >=1000" size="small" color="primary" variant="outlined" />
                <Typography variant="caption" color="text.secondary" sx={{ alignSelf: 'center' }}>
                  最大 replica 总数
                </Typography>
              </Box>
            </Box>

            {/* Max Assignment Count Limit */}
            <Box sx={{ display: 'flex', alignItems: 'center', gap: 2 }}>
              <Box sx={{ minWidth: 180 }}>
                <Typography variant="body2" fontWeight="bold">Max Assignment Count</Typography>
              </Box>
              <TextField
                size="small"
                type="number"
                sx={{ width: 250 }}
                value={config.system_limit?.max_assignment_count_limit ?? 1000}
                onChange={(e) => updateSystemLimit('max_assignment_count_limit', parseInt(e.target.value) || 1000)}
                disabled={loading}
              />
              <Box sx={{ display: 'flex', gap: 1, flex: 1 }}>
                <Chip label="默认: 1000" size="small" variant="outlined" />
                <Chip label="推荐: >=1000" size="small" color="primary" variant="outlined" />
                <Typography variant="caption" color="text.secondary" sx={{ alignSelf: 'center' }}>
                  最大 Assignment 总数
                </Typography>
              </Box>
            </Box>

            {advancedMode && (
              <>
                {/* Max Concurrent Move Count Limit */}
                <Box sx={{ display: 'flex', alignItems: 'center', gap: 2 }}>
                  <Box sx={{ minWidth: 180 }}>
                    <Typography variant="body2" fontWeight="bold">Max Concurrent Moves</Typography>
                  </Box>
                  <TextField
                    size="small"
                    type="number"
                    sx={{ width: 250 }}
                    value={config.system_limit?.max_concurrent_move_count_limit ?? 30}
                    onChange={(e) => updateSystemLimit('max_concurrent_move_count_limit', parseInt(e.target.value) || 30)}
                    disabled={loading}
                    InputProps={{
                      inputProps: { min: 1, max: 1000 }
                    }}
                  />
                  <Box sx={{ display: 'flex', gap: 1, flex: 1 }}>
                    <Chip label="默认: 30" size="small" variant="outlined" />
                    <Chip label="范围: 1-1000" size="small" color="primary" variant="outlined" />
                    <Chip label="高级" size="small" color="primary" />
                    <Typography variant="caption" color="text.secondary" sx={{ alignSelf: 'center' }}>
                      最大并发迁移数
                    </Typography>
                  </Box>
                </Box>

                {/* Max Hat Count Limit */}
                <Box sx={{ display: 'flex', alignItems: 'center', gap: 2 }}>
                  <Box sx={{ minWidth: 180 }}>
                    <Typography variant="body2" fontWeight="bold">Max Hat Count</Typography>
                  </Box>
                  <TextField
                    size="small"
                    type="number"
                    sx={{ width: 250 }}
                    value={config.system_limit?.max_hat_count_count ?? 10}
                    onChange={(e) => updateSystemLimit('max_hat_count_count', parseInt(e.target.value) || 10)}
                    disabled={loading}
                  />
                  <Box sx={{ display: 'flex', gap: 1, flex: 1 }}>
                    <Chip label="默认: 10" size="small" variant="outlined" />
                    <Chip label="推荐: 10-100" size="small" color="primary" variant="outlined" />
                    <Chip label="高级" size="small" color="primary" />
                    <Typography variant="caption" color="text.secondary" sx={{ alignSelf: 'center' }}>
                      最大 shutdown Hat 数量
                    </Typography>
                  </Box>
                </Box>
              </>
            )}
          </Box>
        </AccordionDetails>
      </Accordion>

      {advancedMode && (
        <>
          <Accordion>
            <AccordionSummary expandIcon={<ExpandMoreIcon />}>
              <Typography variant="h6">🎯 Cost Function Config</Typography>
              <Chip label="高级" size="small" color="primary" sx={{ ml: 2 }} />
            </AccordionSummary>
            <AccordionDetails>
              <Box sx={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
                {/* Shard Count Cost Enable */}
                <Box sx={{ display: 'flex', alignItems: 'center', gap: 2 }}>
                  <Box sx={{ minWidth: 180 }}>
                    <Typography variant="body2" fontWeight="bold">Shard Count Cost Enable</Typography>
                  </Box>
                  <FormControl size="small" sx={{ width: 250 }}>
                    <Select
                      value={config.cost_func_cfg?.shard_count_cost_enable ?? 1}
                      onChange={(e) => updateCostFuncCfg('shard_count_cost_enable', Number(e.target.value))}
                      disabled={loading}
                    >
                      <MenuItem value={0}>禁用 (0)</MenuItem>
                      <MenuItem value={1}>启用 (1)</MenuItem>
                    </Select>
                  </FormControl>
                  <Box sx={{ display: 'flex', gap: 1, flex: 1 }}>
                    <Chip label="默认: 启用" size="small" variant="outlined" />
                    <Typography variant="caption" color="text.secondary" sx={{ alignSelf: 'center' }}>
                      是否启用基于 Shard 数量的负载均衡
                    </Typography>
                  </Box>
                </Box>

                {/* Shard Count Cost Norm */}
                <Box sx={{ display: 'flex', alignItems: 'center', gap: 2 }}>
                  <Box sx={{ minWidth: 180 }}>
                    <Typography variant="body2" fontWeight="bold">Shard Count Cost Norm</Typography>
                  </Box>
                  <TextField
                    size="small"
                    type="number"
                    sx={{ width: 250 }}
                    value={config.cost_func_cfg?.shard_count_cost_norm ?? 10}
                    onChange={(e) => updateCostFuncCfg('shard_count_cost_norm', parseInt(e.target.value) || 10)}
                    disabled={loading || config.cost_func_cfg?.shard_count_cost_enable === 0}
                  />
                  <Box sx={{ display: 'flex', gap: 1, flex: 1 }}>
                    <Chip label="默认: 10" size="small" variant="outlined" />
                    <Typography variant="caption" color="text.secondary" sx={{ alignSelf: 'center' }}>
                      Shard 数量成本归一化值
                    </Typography>
                  </Box>
                </Box>

                {/* Worker Max Assignments */}
                <Box sx={{ display: 'flex', alignItems: 'center', gap: 2 }}>
                  <Box sx={{ minWidth: 180 }}>
                    <Typography variant="body2" fontWeight="bold">Worker Max Assignments</Typography>
                  </Box>
                  <TextField
                    size="small"
                    type="number"
                    sx={{ width: 250 }}
                    value={config.cost_func_cfg?.worker_max_assignments ?? 20}
                    onChange={(e) => updateCostFuncCfg('worker_max_assignments', parseInt(e.target.value) || 20)}
                    disabled={loading}
                  />
                  <Box sx={{ display: 'flex', gap: 1, flex: 1 }}>
                    <Chip label="默认: 20" size="small" variant="outlined" />
                    <Typography variant="caption" color="text.secondary" sx={{ alignSelf: 'center' }}>
                      Worker 最大任务数
                    </Typography>
                  </Box>
                </Box>
              </Box>
            </AccordionDetails>
          </Accordion>

          <Accordion>
            <AccordionSummary expandIcon={<ExpandMoreIcon />}>
              <Typography variant="h6">🤖 Solver Config</Typography>
              <Chip label="高级" size="small" color="primary" sx={{ ml: 2 }} />
            </AccordionSummary>
            <AccordionDetails>
              <Box sx={{ display: 'flex', flexDirection: 'column', gap: 3 }}>
                {/* Soft Solver Config */}
                <Box>
                  <Typography variant="subtitle1" fontWeight="bold" sx={{ mb: 1 }}>🔹 Soft Solver</Typography>
                  <Box sx={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
                    <Box sx={{ display: 'flex', alignItems: 'center', gap: 2 }}>
                      <Box sx={{ minWidth: 180 }}>
                        <Typography variant="body2" fontWeight="bold">Solver Enabled</Typography>
                      </Box>
                      <FormControl size="small" sx={{ width: 250 }}>
                        <Select
                          value={config.solver_config?.soft_solver_config?.solver_enabled ?? 1}
                          onChange={(e) => updateSolverConfig('soft_solver_config', 'solver_enabled', Number(e.target.value))}
                          disabled={loading}
                        >
                          <MenuItem value={0}>禁用 (0)</MenuItem>
                          <MenuItem value={1}>启用 (1)</MenuItem>
                        </Select>
                      </FormControl>
                      <Box sx={{ display: 'flex', gap: 1, flex: 1 }}>
                        <Chip label="默认: 启用" size="small" variant="outlined" />
                        <Typography variant="caption" color="text.secondary" sx={{ alignSelf: 'center' }}>
                          是否启用
                        </Typography>
                      </Box>
                    </Box>
                    <Box sx={{ display: 'flex', alignItems: 'center', gap: 2 }}>
                      <Box sx={{ minWidth: 180 }}>
                        <Typography variant="body2" fontWeight="bold">Run Per Minute</Typography>
                      </Box>
                      <TextField
                        size="small"
                        type="number"
                        sx={{ width: 250 }}
                        value={config.solver_config?.soft_solver_config?.run_per_minute ?? 10}
                        onChange={(e) => updateSolverConfig('soft_solver_config', 'run_per_minute', parseInt(e.target.value) || 10)}
                        disabled={loading || config.solver_config?.soft_solver_config?.solver_enabled === 0}
                      />
                      <Box sx={{ display: 'flex', gap: 1, flex: 1 }}>
                        <Chip label="默认: 10" size="small" variant="outlined" />
                        <Typography variant="caption" color="text.secondary" sx={{ alignSelf: 'center' }}>
                          每分钟运行次数
                        </Typography>
                      </Box>
                    </Box>
                    <Box sx={{ display: 'flex', alignItems: 'center', gap: 2 }}>
                      <Box sx={{ minWidth: 180 }}>
                        <Typography variant="body2" fontWeight="bold">Explore Per Run</Typography>
                      </Box>
                      <TextField
                        size="small"
                        type="number"
                        sx={{ width: 250 }}
                        value={config.solver_config?.soft_solver_config?.explore_per_run ?? 10}
                        onChange={(e) => updateSolverConfig('soft_solver_config', 'explore_per_run', parseInt(e.target.value) || 10)}
                        disabled={loading || config.solver_config?.soft_solver_config?.solver_enabled === 0}
                      />
                      <Box sx={{ display: 'flex', gap: 1, flex: 1 }}>
                        <Chip label="默认: 10" size="small" variant="outlined" />
                        <Typography variant="caption" color="text.secondary" sx={{ alignSelf: 'center' }}>
                          每次探索数
                        </Typography>
                      </Box>
                    </Box>
                  </Box>
                </Box>

                {/* Assign Solver Config */}
                <Box>
                  <Typography variant="subtitle1" fontWeight="bold" sx={{ mb: 1 }}>🔹 Assign Solver</Typography>
                  <Box sx={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
                    <Box sx={{ display: 'flex', alignItems: 'center', gap: 2 }}>
                      <Box sx={{ minWidth: 180 }}>
                        <Typography variant="body2" fontWeight="bold">Solver Enabled</Typography>
                      </Box>
                      <FormControl size="small" sx={{ width: 250 }}>
                        <Select
                          value={config.solver_config?.assign_solver_config?.solver_enabled ?? 1}
                          onChange={(e) => updateSolverConfig('assign_solver_config', 'solver_enabled', Number(e.target.value))}
                          disabled={loading}
                        >
                          <MenuItem value={0}>禁用 (0)</MenuItem>
                          <MenuItem value={1}>启用 (1)</MenuItem>
                        </Select>
                      </FormControl>
                      <Box sx={{ display: 'flex', gap: 1, flex: 1 }}>
                        <Chip label="默认: 启用" size="small" variant="outlined" />
                        <Typography variant="caption" color="text.secondary" sx={{ alignSelf: 'center' }}>
                          是否启用
                        </Typography>
                      </Box>
                    </Box>
                    <Box sx={{ display: 'flex', alignItems: 'center', gap: 2 }}>
                      <Box sx={{ minWidth: 180 }}>
                        <Typography variant="body2" fontWeight="bold">Run Per Minute</Typography>
                      </Box>
                      <TextField
                        size="small"
                        type="number"
                        sx={{ width: 250 }}
                        value={config.solver_config?.assign_solver_config?.run_per_minute ?? 10}
                        onChange={(e) => updateSolverConfig('assign_solver_config', 'run_per_minute', parseInt(e.target.value) || 10)}
                        disabled={loading || config.solver_config?.assign_solver_config?.solver_enabled === 0}
                      />
                      <Box sx={{ display: 'flex', gap: 1, flex: 1 }}>
                        <Chip label="默认: 10" size="small" variant="outlined" />
                        <Typography variant="caption" color="text.secondary" sx={{ alignSelf: 'center' }}>
                          每分钟运行次数
                        </Typography>
                      </Box>
                    </Box>
                    <Box sx={{ display: 'flex', alignItems: 'center', gap: 2 }}>
                      <Box sx={{ minWidth: 180 }}>
                        <Typography variant="body2" fontWeight="bold">Explore Per Run</Typography>
                      </Box>
                      <TextField
                        size="small"
                        type="number"
                        sx={{ width: 250 }}
                        value={config.solver_config?.assign_solver_config?.explore_per_run ?? 10}
                        onChange={(e) => updateSolverConfig('assign_solver_config', 'explore_per_run', parseInt(e.target.value) || 10)}
                        disabled={loading || config.solver_config?.assign_solver_config?.solver_enabled === 0}
                      />
                      <Box sx={{ display: 'flex', gap: 1, flex: 1 }}>
                        <Chip label="默认: 10" size="small" variant="outlined" />
                        <Typography variant="caption" color="text.secondary" sx={{ alignSelf: 'center' }}>
                          每次探索数
                        </Typography>
                      </Box>
                    </Box>
                  </Box>
                </Box>

                {/* Unassign Solver Config */}
                <Box>
                  <Typography variant="subtitle1" fontWeight="bold" sx={{ mb: 1 }}>🔹 Unassign Solver</Typography>
                  <Box sx={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
                    <Box sx={{ display: 'flex', alignItems: 'center', gap: 2 }}>
                      <Box sx={{ minWidth: 180 }}>
                        <Typography variant="body2" fontWeight="bold">Solver Enabled</Typography>
                      </Box>
                      <FormControl size="small" sx={{ width: 250 }}>
                        <Select
                          value={config.solver_config?.unassign_solver_config?.solver_enabled ?? 1}
                          onChange={(e) => updateSolverConfig('unassign_solver_config', 'solver_enabled', Number(e.target.value))}
                          disabled={loading}
                        >
                          <MenuItem value={0}>禁用 (0)</MenuItem>
                          <MenuItem value={1}>启用 (1)</MenuItem>
                        </Select>
                      </FormControl>
                      <Box sx={{ display: 'flex', gap: 1, flex: 1 }}>
                        <Chip label="默认: 启用" size="small" variant="outlined" />
                        <Typography variant="caption" color="text.secondary" sx={{ alignSelf: 'center' }}>
                          是否启用
                        </Typography>
                      </Box>
                    </Box>
                    <Box sx={{ display: 'flex', alignItems: 'center', gap: 2 }}>
                      <Box sx={{ minWidth: 180 }}>
                        <Typography variant="body2" fontWeight="bold">Run Per Minute</Typography>
                      </Box>
                      <TextField
                        size="small"
                        type="number"
                        sx={{ width: 250 }}
                        value={config.solver_config?.unassign_solver_config?.run_per_minute ?? 10}
                        onChange={(e) => updateSolverConfig('unassign_solver_config', 'run_per_minute', parseInt(e.target.value) || 10)}
                        disabled={loading || config.solver_config?.unassign_solver_config?.solver_enabled === 0}
                      />
                      <Box sx={{ display: 'flex', gap: 1, flex: 1 }}>
                        <Chip label="默认: 10" size="small" variant="outlined" />
                        <Typography variant="caption" color="text.secondary" sx={{ alignSelf: 'center' }}>
                          每分钟运行次数
                        </Typography>
                      </Box>
                    </Box>
                    <Box sx={{ display: 'flex', alignItems: 'center', gap: 2 }}>
                      <Box sx={{ minWidth: 180 }}>
                        <Typography variant="body2" fontWeight="bold">Explore Per Run</Typography>
                      </Box>
                      <TextField
                        size="small"
                        type="number"
                        sx={{ width: 250 }}
                        value={config.solver_config?.unassign_solver_config?.explore_per_run ?? 10}
                        onChange={(e) => updateSolverConfig('unassign_solver_config', 'explore_per_run', parseInt(e.target.value) || 10)}
                        disabled={loading || config.solver_config?.unassign_solver_config?.solver_enabled === 0}
                      />
                      <Box sx={{ display: 'flex', gap: 1, flex: 1 }}>
                        <Chip label="默认: 10" size="small" variant="outlined" />
                        <Typography variant="caption" color="text.secondary" sx={{ alignSelf: 'center' }}>
                          每次探索数
                        </Typography>
                      </Box>
                    </Box>
                  </Box>
                </Box>
              </Box>
            </AccordionDetails>
          </Accordion>

          <Accordion>
            <AccordionSummary expandIcon={<ExpandMoreIcon />}>
              <Typography variant="h6">📊 Dynamic Threshold</Typography>
              <Chip label="高级" size="small" color="primary" sx={{ ml: 2 }} />
            </AccordionSummary>
            <AccordionDetails>
              <Box sx={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
                {/* Dynamic Threshold Max */}
                <Box sx={{ display: 'flex', alignItems: 'center', gap: 2 }}>
                  <Box sx={{ minWidth: 180 }}>
                    <Typography variant="body2" fontWeight="bold">Dynamic Threshold Max</Typography>
                  </Box>
                  <TextField
                    size="small"
                    type="number"
                    sx={{ width: 250 }}
                    value={config.dynamic_threshold?.dynamic_threshold_max ?? 100}
                    onChange={(e) => updateDynamicThreshold('dynamic_threshold_max', parseInt(e.target.value) || 100)}
                    disabled={loading}
                  />
                  <Box sx={{ display: 'flex', gap: 1, flex: 1 }}>
                    <Chip label="默认: 100" size="small" variant="outlined" />
                    <Typography variant="caption" color="text.secondary" sx={{ alignSelf: 'center' }}>
                      动态阈值最大值
                    </Typography>
                  </Box>
                </Box>

                {/* Dynamic Threshold Min */}
                <Box sx={{ display: 'flex', alignItems: 'center', gap: 2 }}>
                  <Box sx={{ minWidth: 180 }}>
                    <Typography variant="body2" fontWeight="bold">Dynamic Threshold Min</Typography>
                  </Box>
                  <TextField
                    size="small"
                    type="number"
                    sx={{ width: 250 }}
                    value={config.dynamic_threshold?.dynamic_threshold_min ?? 10}
                    onChange={(e) => updateDynamicThreshold('dynamic_threshold_min', parseInt(e.target.value) || 10)}
                    disabled={loading}
                  />
                  <Box sx={{ display: 'flex', gap: 1, flex: 1 }}>
                    <Chip label="默认: 10" size="small" variant="outlined" />
                    <Typography variant="caption" color="text.secondary" sx={{ alignSelf: 'center' }}>
                      动态阈值最小值
                    </Typography>
                  </Box>
                </Box>

                {/* Half Decay Time */}
                <Box sx={{ display: 'flex', alignItems: 'center', gap: 2 }}>
                  <Box sx={{ minWidth: 180 }}>
                    <Typography variant="body2" fontWeight="bold">Half Decay Time</Typography>
                  </Box>
                  <TextField
                    size="small"
                    type="number"
                    sx={{ width: 250 }}
                    value={config.dynamic_threshold?.half_decay_time_sec ?? 300}
                    onChange={(e) => updateDynamicThreshold('half_decay_time_sec', parseInt(e.target.value) || 300)}
                    disabled={loading}
                    InputProps={{
                      endAdornment: <InputAdornment position="end">秒</InputAdornment>
                    }}
                  />
                  <Box sx={{ display: 'flex', gap: 1, flex: 1 }}>
                    <Chip label="默认: 300秒" size="small" variant="outlined" />
                    <Typography variant="caption" color="text.secondary" sx={{ alignSelf: 'center' }}>
                      半衰期时间
                    </Typography>
                  </Box>
                </Box>

                {/* Increase Per Move */}
                <Box sx={{ display: 'flex', alignItems: 'center', gap: 2 }}>
                  <Box sx={{ minWidth: 180 }}>
                    <Typography variant="body2" fontWeight="bold">Increase Per Move</Typography>
                  </Box>
                  <TextField
                    size="small"
                    type="number"
                    sx={{ width: 250 }}
                    value={config.dynamic_threshold?.increase_per_move ?? 10}
                    onChange={(e) => updateDynamicThreshold('increase_per_move', parseInt(e.target.value) || 10)}
                    disabled={loading}
                  />
                  <Box sx={{ display: 'flex', gap: 1, flex: 1 }}>
                    <Chip label="默认: 10" size="small" variant="outlined" />
                    <Typography variant="caption" color="text.secondary" sx={{ alignSelf: 'center' }}>
                      每次迁移增加值
                    </Typography>
                  </Box>
                </Box>
              </Box>
            </AccordionDetails>
          </Accordion>

          <Accordion>
            <AccordionSummary expandIcon={<ExpandMoreIcon />}>
              <Typography variant="h6">🛡️ Fault Tolerance</Typography>
              <Chip label="高级" size="small" color="primary" sx={{ ml: 2 }} />
            </AccordionSummary>
            <AccordionDetails>
              <Box sx={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
                {/* Grace Period Before Drain */}
                <Box sx={{ display: 'flex', alignItems: 'center', gap: 2 }}>
                  <Box sx={{ minWidth: 180 }}>
                    <Typography variant="body2" fontWeight="bold">Grace Period Before Drain</Typography>
                  </Box>
                  <TextField
                    size="small"
                    type="number"
                    sx={{ width: 250 }}
                    value={config.fault_tolerance?.grace_period_sec_before_drain ?? 0}
                    onChange={(e) => updateFaultTolerance('grace_period_sec_before_drain', parseInt(e.target.value) || 0)}
                    disabled={loading}
                    InputProps={{
                      endAdornment: <InputAdornment position="end">秒</InputAdornment>
                    }}
                  />
                  <Box sx={{ display: 'flex', gap: 1, flex: 1 }}>
                    <Chip label="默认: 0秒" size="small" variant="outlined" />
                    <Typography variant="caption" color="text.secondary" sx={{ alignSelf: 'center' }}>
                      Drain 前宽限期
                    </Typography>
                  </Box>
                </Box>

                {/* Grace Period Before Dirty Purge */}
                <Box sx={{ display: 'flex', alignItems: 'center', gap: 2 }}>
                  <Box sx={{ minWidth: 180 }}>
                    <Typography variant="body2" fontWeight="bold">Grace Period Before Dirty Purge</Typography>
                  </Box>
                  <TextField
                    size="small"
                    type="number"
                    sx={{ width: 250 }}
                    value={config.fault_tolerance?.grace_period_sec_before_dirty_purge ?? 86400}
                    onChange={(e) => updateFaultTolerance('grace_period_sec_before_dirty_purge', parseInt(e.target.value) || 86400)}
                    disabled={loading}
                    InputProps={{
                      endAdornment: <InputAdornment position="end">秒</InputAdornment>
                    }}
                  />
                  <Box sx={{ display: 'flex', gap: 1, flex: 1 }}>
                    <Chip label="默认: 86400秒 (24h)" size="small" variant="outlined" />
                    <Typography variant="caption" color="text.secondary" sx={{ alignSelf: 'center' }}>
                      Dirty Purge 前宽限期
                    </Typography>
                  </Box>
                </Box>
              </Box>
            </AccordionDetails>
          </Accordion>
        </>
      )}

      <Dialog open={confirmDialogOpen} onClose={() => setConfirmDialogOpen(false)} maxWidth="lg" fullWidth>
        <DialogTitle>
          <Box display="flex" justifyContent="space-between" alignItems="center">
            <Typography variant="h6">确认保存配置</Typography>
            <Button
              size="small"
              variant="outlined"
              onClick={() => setShowFullConfig(!showFullConfig)}
            >
              {showFullConfig ? '显示 Diff' : '显示完整配置'}
            </Button>
          </Box>
        </DialogTitle>
        <DialogContent>
          {!showFullConfig ? (
            <>
              <Typography variant="body2" sx={{ mb: 2 }}>
                检测到 <strong>{configDiff.length}</strong> 项配置变更：
              </Typography>
              {configDiff.length === 0 ? (
                <Alert severity="info">没有配置变更</Alert>
              ) : (
                <Paper sx={{ p: 2, bgcolor: '#f5f5f5', maxHeight: 500, overflow: 'auto' }}>
                  {configDiff.map((diff, index) => (
                    <Box key={index} sx={{ mb: 2, pb: 2, borderBottom: index < configDiff.length - 1 ? '1px solid #ddd' : 'none' }}>
                      <Typography variant="body2" fontWeight="bold" sx={{ mb: 0.5 }}>
                        {diff.path}
                      </Typography>
                      <Box sx={{ display: 'flex', gap: 2, alignItems: 'center' }}>
                        <Box sx={{ flex: 1 }}>
                          <Typography variant="caption" color="text.secondary">原值：</Typography>
                          <Paper sx={{ p: 1, bgcolor: '#ffebee', border: '1px solid #ef5350' }}>
                            <Typography variant="body2" component="code" sx={{ color: '#c62828' }}>
                              {diff.oldValue === undefined ? 'undefined' : JSON.stringify(diff.oldValue)}
                            </Typography>
                          </Paper>
                        </Box>
                        <Typography variant="h6" color="text.secondary">→</Typography>
                        <Box sx={{ flex: 1 }}>
                          <Typography variant="caption" color="text.secondary">新值：</Typography>
                          <Paper sx={{ p: 1, bgcolor: '#e8f5e9', border: '1px solid #66bb6a' }}>
                            <Typography variant="body2" component="code" sx={{ color: '#2e7d32' }}>
                              {diff.newValue === undefined ? 'undefined' : JSON.stringify(diff.newValue)}
                            </Typography>
                          </Paper>
                        </Box>
                      </Box>
                    </Box>
                  ))}
                </Paper>
              )}
            </>
          ) : (
            <>
              <Typography variant="body2" sx={{ mb: 2 }}>
                完整配置（将保存）：
              </Typography>
              <Paper sx={{ p: 2, bgcolor: '#f5f5f5', maxHeight: 500, overflow: 'auto' }}>
                <pre style={{ margin: 0, fontSize: '12px', whiteSpace: 'pre-wrap', wordBreak: 'break-word' }}>
                  {JSON.stringify(config, null, 2)}
                </pre>
              </Paper>
            </>
          )}
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setConfirmDialogOpen(false)} disabled={saving}>
            取消
          </Button>
          <Button onClick={saveConfig} variant="contained" color="primary" disabled={saving || configDiff.length === 0}>
            {saving ? '保存中...' : `确认保存 (${configDiff.length} 项变更)`}
          </Button>
        </DialogActions>
      </Dialog>
    </div>
  );
};

export default ServiceConfigPage;
