import { Typography, Paper, Box } from '@mui/material';

export default function ConfigPage() {
  return (
    <div>
      <Typography variant="h4" component="h1" gutterBottom>
        配置管理
      </Typography>
      
      <Paper sx={{ p: 3, mt: 3 }}>
        <Box sx={{ mb: 2 }}>
          <Typography variant="h6" gutterBottom>
            配置功能即将推出
          </Typography>
          <Typography variant="body1">
            这是配置管理页面的占位内容。该功能正在开发中，敬请期待。
          </Typography>
        </Box>
      </Paper>
    </div>
  );
} 