import { useState, useEffect, useCallback } from 'react';
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
import { Save as SaveIcon, Refresh as RefreshIcon } from '@mui/icons-material';
import * as api from '../services/api';

export default function ShardPlanPage() {
  const [shardPlan, setShardPlan] = useState('');
  const [originalShardPlan, setOriginalShardPlan] = useState('');
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);
  const [confirmDialogOpen, setConfirmDialogOpen] = useState(false);

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
  }, [loadShardPlan]);

  // 处理保存按钮点击
  const handleSaveClick = () => {
    setConfirmDialogOpen(true);
  };

  // 处理刷新按钮点击
  const handleRefreshClick = () => {
    if (hasChanges) {
      if (window.confirm('有未保存的更改，确定要刷新吗？')) {
        loadShardPlan();
      }
    } else {
      loadShardPlan();
    }
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
            onClick={handleRefreshClick}
            disabled={loading || saving}
          >
            刷新
          </Button>
          <Button
            variant="contained"
            startIcon={<SaveIcon />}
            onClick={handleSaveClick}
            disabled={loading || saving || !hasChanges}
            color="primary"
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

      {/* 确认对话框 */}
      <Dialog
        open={confirmDialogOpen}
        onClose={() => !saving && setConfirmDialogOpen(false)}
      >
        <DialogTitle>确认保存</DialogTitle>
        <DialogContent>
          <DialogContentText>
            确定要保存 Shard Plan 吗？这将更新 etcd 中的配置。
            <br />
            <br />
            <strong>统计信息：</strong>
            <br />
            • 总行数: {stats.lineCount}
            <br />
            • Shard 数量: {stats.shardCount}
          </DialogContentText>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setConfirmDialogOpen(false)} disabled={saving}>
            取消
          </Button>
          <Button onClick={saveShardPlan} disabled={saving} variant="contained" autoFocus>
            {saving ? <CircularProgress size={24} /> : '确认保存'}
          </Button>
        </DialogActions>
      </Dialog>
    </div>
  );
}
