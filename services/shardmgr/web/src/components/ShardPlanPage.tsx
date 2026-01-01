import React, { useState, useEffect, useCallback, useMemo } from 'react';
import {
  Typography,
  Box,
  Paper,
  Button,
  Alert,
  CircularProgress,
  Dialog,
  DialogTitle,
  DialogContent,
  DialogContentText,
  DialogActions,
  Chip,
} from '@mui/material';
import { 
  Save as SaveIcon, 
  Refresh as RefreshIcon,
  Build as BuildIcon,
  ExpandMore as ExpandMoreIcon,
} from '@mui/icons-material';
import * as api from '../services/api';

// Shard Plan 生成器配置
interface GeneratorConfig {
  shardCount: number;
  shardPrefix: string;
  minLength: number;
}

// 生成 shard plan
function generateShardPlan(config: GeneratorConfig): string {
  const lines: string[] = [];
  const maxValue = 0x100000000; // 2^32
  
  for (let i = 0; i < config.shardCount; i++) {
    const start = Math.floor(maxValue / config.shardCount * i);
    const end = Math.floor(maxValue / config.shardCount * (i + 1));
    lines.push(formatShardName(start, end, config));
  }
  
  return lines.join('\n');
}

function formatShardName(
  start: number,
  end: number,
  config: GeneratorConfig
): string {
  return `${config.shardPrefix}_${formatPos(start, config.minLength)}_${formatPos(end, config.minLength)}`;
}

function formatPos(pos: number, minLength: number): string {
  let str = pos.toString(16).toUpperCase();
  
  // 截取最后 8 位
  if (str.length > 8) {
    str = str.substring(str.length - 8);
  }
  
  // 补齐到 8 位
  str = str.padStart(8, '0');
  
  // 去除尾部的 0，但保持最小长度
  while (str.length > minLength && str.endsWith('0')) {
    str = str.substring(0, str.length - 1);
  }
  
  return str;
}

// 简单的 diff 计算函数
interface DiffLine {
  type: 'added' | 'removed' | 'unchanged';
  content: string;
  lineNumber?: number;
}

const computeDiff = (oldText: string, newText: string): DiffLine[] => {
  const oldLines = oldText.split('\n');
  const newLines = newText.split('\n');
  const diff: DiffLine[] = [];
  
  // 简单的逐行比较算法
  let oldIndex = 0;
  let newIndex = 0;
  
  while (oldIndex < oldLines.length || newIndex < newLines.length) {
    if (oldIndex >= oldLines.length) {
      // 只剩新行
      diff.push({ type: 'added', content: newLines[newIndex], lineNumber: newIndex + 1 });
      newIndex++;
    } else if (newIndex >= newLines.length) {
      // 只剩旧行
      diff.push({ type: 'removed', content: oldLines[oldIndex], lineNumber: oldIndex + 1 });
      oldIndex++;
    } else if (oldLines[oldIndex] === newLines[newIndex]) {
      // 相同行
      diff.push({ type: 'unchanged', content: oldLines[oldIndex], lineNumber: newIndex + 1 });
      oldIndex++;
      newIndex++;
    } else {
      // 不同行 - 简单处理：先删除旧行，再添加新行
      diff.push({ type: 'removed', content: oldLines[oldIndex], lineNumber: oldIndex + 1 });
      diff.push({ type: 'added', content: newLines[newIndex], lineNumber: newIndex + 1 });
      oldIndex++;
      newIndex++;
    }
  }
  
  return diff;
};

export default function ShardPlanPage() {
  const [shardPlan, setShardPlan] = useState('');
  const [originalShardPlan, setOriginalShardPlan] = useState('');
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);
  const [confirmDialogOpen, setConfirmDialogOpen] = useState(false);
  const [generatorOpen, setGeneratorOpen] = useState(false);
  const [generatorConfig, setGeneratorConfig] = useState<GeneratorConfig>({
    shardCount: 256,
    shardPrefix: 'shard',
    minLength: 4,
  });
  const [generateConfirmOpen, setGenerateConfirmOpen] = useState(false);
  const [advancedMode, setAdvancedMode] = useState(false);

  // 计算统计信息
  const getStats = useCallback((text: string) => {
    const lines = text.split('\n');
    const lineCount = lines.length;
    
    let shardCount = 0;
    for (const line of lines) {
      const trimmed = line.trim();
      // 跳过空行和注释行
      if (trimmed.length === 0 || trimmed.startsWith('#')) {
        continue;
      }
      // 如果行中有注释，只取注释前的部分
      const commentIndex = trimmed.indexOf('#');
      const content = commentIndex >= 0 ? trimmed.substring(0, commentIndex).trim() : trimmed;
      if (content.length > 0) {
        shardCount++;
      }
    }
    
    return { lineCount, shardCount };
  }, []);

  const stats = getStats(shardPlan);
  const hasChanges = shardPlan !== originalShardPlan;
  
  // 计算 diff
  const diff = useMemo(() => {
    if (!hasChanges) return [];
    return computeDiff(originalShardPlan, shardPlan);
  }, [originalShardPlan, shardPlan, hasChanges]);
  
  // 统计变更数量
  const changeStats = useMemo(() => {
    const added = diff.filter(d => d.type === 'added').length;
    const removed = diff.filter(d => d.type === 'removed').length;
    return { added, removed };
  }, [diff]);

  // 加载 shard plan
  const loadShardPlan = useCallback(async () => {
    try {
      setLoading(true);
      setError(null);
      const plan = await api.getShardPlan();
      setShardPlan(plan);
      setOriginalShardPlan(plan);
    } catch (err) {
      console.error('Failed to load shard plan:', err);
      setError('加载 Shard Plan 失败');
    } finally {
      setLoading(false);
    }
  }, []);

  // 保存 shard plan
  const saveShardPlan = useCallback(async () => {
    try {
      setSaving(true);
      setError(null);
      setSuccess(null);
      await api.setShardPlan(shardPlan);
      setOriginalShardPlan(shardPlan);
      setSuccess('Shard Plan 保存成功！');
      setConfirmDialogOpen(false);
      
      // 3秒后清除成功消息
      setTimeout(() => setSuccess(null), 3000);
    } catch (err) {
      console.error('Failed to save shard plan:', err);
      setError('保存 Shard Plan 失败');
      setConfirmDialogOpen(false);
    } finally {
      setSaving(false);
    }
  }, [shardPlan]);

  // 初始加载
  useEffect(() => {
    loadShardPlan();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // 处理保存按钮点击
  const handleSaveClick = () => {
    setConfirmDialogOpen(true);
  };

  // 处理刷新按钮点击
  const handleRefresh = () => {
    if (hasChanges) {
      if (window.confirm('有未保存的更改，确定要刷新吗？')) {
        loadShardPlan();
      }
    } else {
      loadShardPlan();
    }
  };

  // 生成 shard plan
  const handleGenerate = () => {
    if (hasChanges || shardPlan.trim().length > 0) {
      // 如果有内容，先确认
      setGenerateConfirmOpen(true);
    } else {
      // 直接生成
      doGenerate();
    }
  };

  const doGenerate = () => {
    const generated = generateShardPlan(generatorConfig);
    setShardPlan(generated);
    setGenerateConfirmOpen(false);
    setGeneratorOpen(false);
    setSuccess(`已生成 ${generatorConfig.shardCount} 个 shard`);
    setTimeout(() => setSuccess(null), 3000);
  };

  return (
    <div>
      {/* 页面标题 */}
      <Box display="flex" justifyContent="space-between" alignItems="center" mb={3}>
        <Typography variant="h4" component="h1">
          Shard Plan 管理
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
            startIcon={<BuildIcon />}
            endIcon={<ExpandMoreIcon sx={{ transform: generatorOpen ? 'rotate(180deg)' : 'rotate(0deg)', transition: 'transform 0.3s' }} />}
            onClick={() => setGeneratorOpen(!generatorOpen)}
            disabled={loading}
          >
            生成器
          </Button>
          <Button
            variant="contained"
            startIcon={<SaveIcon />}
            onClick={() => setConfirmDialogOpen(true)}
            disabled={!hasChanges || saving}
          >
            保存
          </Button>
        </Box>
      </Box>

      {/* 错误提示 */}
      {error && (
        <Alert severity="error" sx={{ mb: 2 }} onClose={() => setError(null)}>
          {error}
        </Alert>
      )}

      {/* 成功提示 */}
      {success && (
        <Alert severity="success" sx={{ mb: 2 }} onClose={() => setSuccess(null)}>
          {success}
        </Alert>
      )}

      {/* 生成器面板 */}
      {generatorOpen && (
        <Paper sx={{ p: 2, mb: 2, bgcolor: '#f5f5f5' }}>
          <Typography variant="h6" gutterBottom sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
            <BuildIcon />
            Shard Plan 生成器
          </Typography>
          <Box sx={{ display: 'flex', gap: 2, flexWrap: 'wrap', alignItems: 'flex-end' }}>
            <Box sx={{ minWidth: 150 }}>
              <Typography variant="body2" color="text.secondary" gutterBottom>
                Shard 数量
              </Typography>
              {advancedMode ? (
                <input
                  type="number"
                  min={1}
                  max={10000}
                  value={generatorConfig.shardCount}
                  onChange={(e) => setGeneratorConfig({ ...generatorConfig, shardCount: Number(e.target.value) || 1 })}
                  style={{
                    width: '100%',
                    padding: '8px',
                    fontSize: '14px',
                    borderRadius: '4px',
                    border: '1px solid #ccc',
                  }}
                  placeholder="输入自定义数量"
                />
              ) : (
                <select
                  value={generatorConfig.shardCount}
                  onChange={(e) => setGeneratorConfig({ ...generatorConfig, shardCount: Number(e.target.value) })}
                  style={{
                    width: '100%',
                    padding: '8px',
                    fontSize: '14px',
                    borderRadius: '4px',
                    border: '1px solid #ccc',
                  }}
                >
                  <option value={16}>16</option>
                  <option value={32}>32</option>
                  <option value={64}>64</option>
                  <option value={128}>128</option>
                  <option value={256}>256</option>
                  <option value={512}>512</option>
                  <option value={1024}>1024</option>
                </select>
              )}
            </Box>
            {advancedMode && (
              <>
                <Box sx={{ minWidth: 150 }}>
                  <Typography variant="body2" color="text.secondary" gutterBottom>
                    Shard 前缀
                  </Typography>
                  <input
                    type="text"
                    value={generatorConfig.shardPrefix}
                    onChange={(e) => setGeneratorConfig({ ...generatorConfig, shardPrefix: e.target.value })}
                    style={{
                      width: '100%',
                      padding: '8px',
                      fontSize: '14px',
                      borderRadius: '4px',
                      border: '1px solid #ccc',
                    }}
                  />
                </Box>
                <Box sx={{ minWidth: 100 }}>
                  <Typography variant="body2" color="text.secondary" gutterBottom>
                    最小长度
                  </Typography>
                  <select
                    value={generatorConfig.minLength}
                    onChange={(e) => setGeneratorConfig({ ...generatorConfig, minLength: Number(e.target.value) })}
                    style={{
                      width: '100%',
                      padding: '8px',
                      fontSize: '14px',
                      borderRadius: '4px',
                      border: '1px solid #ccc',
                    }}
                  >
                    <option value={1}>1</option>
                    <option value={2}>2</option>
                    <option value={3}>3</option>
                    <option value={4}>4</option>
                    <option value={5}>5</option>
                    <option value={6}>6</option>
                    <option value={7}>7</option>
                    <option value={8}>8</option>
                  </select>
                </Box>
              </>
            )}
            <Button
              variant="contained"
              color="primary"
              onClick={handleGenerate}
              sx={{ height: '40px' }}
            >
              生成
            </Button>
          </Box>
          <Box sx={{ mt: 1, display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
            <Typography variant="caption" color="text.secondary">
              将生成 {generatorConfig.shardCount} 个 shard，格式：{generatorConfig.shardPrefix}_START_END
            </Typography>
            <Button
              size="small"
              onClick={() => setAdvancedMode(!advancedMode)}
              sx={{ textTransform: 'none', fontSize: '12px' }}
            >
              {advancedMode ? '← 使用预设值' : '高级选项 →'}
            </Button>
          </Box>
        </Paper>
      )}

      {/* 统计信息 */}
      <Box display="flex" gap={2} mb={2}>
        <Chip label={`总行数: ${stats.lineCount}`} color="default" />
        <Chip label={`Shard 数量: ${stats.shardCount}`} color="primary" />
        {hasChanges && <Chip label="未保存" color="warning" />}
      </Box>

      {/* 编辑器区域 */}
      {loading ? (
        <Box display="flex" justifyContent="center" p={4}>
          <CircularProgress />
        </Box>
      ) : (
        <Paper sx={{ p: 0, overflow: 'hidden' }}>
          <textarea
            value={shardPlan}
            onChange={(e) => setShardPlan(e.target.value)}
            disabled={saving}
            style={{
              width: '100%',
              minHeight: '500px',
              padding: '16px',
              fontFamily: 'Monaco, Consolas, "Courier New", monospace',
              fontSize: '14px',
              lineHeight: '1.5',
              border: 'none',
              outline: 'none',
              resize: 'vertical',
              boxSizing: 'border-box',
            }}
            placeholder="输入 shard plan，每行一个 shard 名称..."
          />
        </Paper>
      )}

      {/* 帮助信息 */}
      <Paper sx={{ p: 2, mt: 2, bgcolor: '#f5f5f5' }}>
        <Typography variant="subtitle2" gutterBottom>
          格式说明：
        </Typography>
        <Typography variant="body2" component="div">
          • 每行一个 shard 名称
          <br />
          • 支持注释：以 # 开头的行或行内 # 后的内容
          <br />
          • 可选配置：shard-name|{'{'}&#34;min_replica_count&#34;:2{'}'}
          <br />
          • 示例：
          <pre style={{ margin: '8px 0', padding: '8px', background: 'white', borderRadius: '4px' }}>
{`shard-1
shard-2|{"min_replica_count":2}
# 这是注释
shard-3  # 行内注释`}
          </pre>
        </Typography>
      </Paper>

      {/* 确认对话框 - 显示 diff */}
      <Dialog
        open={confirmDialogOpen}
        onClose={() => !saving && setConfirmDialogOpen(false)}
        maxWidth="md"
        fullWidth
      >
        <DialogTitle>确认保存 - 变更预览</DialogTitle>
        <DialogContent>
          <DialogContentText sx={{ mb: 2 }}>
            确定要保存 Shard Plan 吗？这将更新 etcd 中的配置。
          </DialogContentText>
          
          {/* 变更统计 */}
          <Box sx={{ mb: 2, display: 'flex', gap: 1 }}>
            <Chip 
              label={`新增: ${changeStats.added} 行`} 
              color="success" 
              size="small" 
            />
            <Chip 
              label={`删除: ${changeStats.removed} 行`} 
              color="error" 
              size="small" 
            />
            <Chip 
              label={`Shard 数量: ${stats.shardCount}`} 
              color="primary" 
              size="small" 
            />
          </Box>
          
          {/* Diff 视图 */}
          <Paper 
            sx={{ 
              maxHeight: '400px', 
              overflow: 'auto',
              bgcolor: '#f5f5f5',
              p: 0,
            }}
          >
            <Box
              component="pre"
              sx={{
                margin: 0,
                padding: 2,
                fontFamily: 'Monaco, Consolas, "Courier New", monospace',
                fontSize: '12px',
                lineHeight: '1.5',
                whiteSpace: 'pre-wrap',
                wordBreak: 'break-all',
              }}
            >
              {diff.map((line, index) => (
                <Box
                  key={index}
                  sx={{
                    bgcolor: 
                      line.type === 'added' ? '#e6ffed' :
                      line.type === 'removed' ? '#ffebe9' :
                      'transparent',
                    color:
                      line.type === 'added' ? '#24292e' :
                      line.type === 'removed' ? '#24292e' :
                      '#6a737d',
                    px: 1,
                    borderLeft: line.type === 'added' ? '3px solid #28a745' :
                                line.type === 'removed' ? '3px solid #d73a49' :
                                '3px solid transparent',
                  }}
                >
                  <span style={{ 
                    display: 'inline-block', 
                    width: '20px',
                    marginRight: '8px',
                    color: '#6a737d',
                  }}>
                    {line.type === 'added' ? '+' : line.type === 'removed' ? '-' : ' '}
                  </span>
                  {line.content || ' '}
                </Box>
              ))}
            </Box>
          </Paper>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setConfirmDialogOpen(false)} disabled={saving}>
            取消
          </Button>
          <Button onClick={saveShardPlan} disabled={saving} variant="contained" color="primary" autoFocus>
            {saving ? <CircularProgress size={24} /> : '确认保存'}
          </Button>
        </DialogActions>
      </Dialog>

      {/* 生成确认对话框 */}
      <Dialog
        open={generateConfirmOpen}
        onClose={() => setGenerateConfirmOpen(false)}
      >
        <DialogTitle>确认生成</DialogTitle>
        <DialogContent>
          <DialogContentText>
            当前编辑器中有内容，生成新的 shard plan 将会<strong>替换</strong>现有内容。
            <br />
            <br />
            将生成 <strong>{generatorConfig.shardCount}</strong> 个 shard。
            <br />
            确定要继续吗？
          </DialogContentText>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setGenerateConfirmOpen(false)}>
            取消
          </Button>
          <Button onClick={doGenerate} variant="contained" color="primary" autoFocus>
            确认生成
          </Button>
        </DialogActions>
      </Dialog>
    </div>
  );
}
