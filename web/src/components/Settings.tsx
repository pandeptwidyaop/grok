import { Box, Typography, Tabs, Tab } from '@mui/material';
import { useState } from 'react';
import TwoFASettings from './TwoFASettings';

interface TabPanelProps {
  children?: React.ReactNode;
  index: number;
  value: number;
}

function TabPanel(props: TabPanelProps) {
  const { children, value, index, ...other } = props;

  return (
    <div
      role="tabpanel"
      hidden={value !== index}
      id={`settings-tabpanel-${index}`}
      aria-labelledby={`settings-tab-${index}`}
      {...other}
    >
      {value === index && <Box sx={{ pt: 3 }}>{children}</Box>}
    </div>
  );
}

export default function Settings() {
  const [currentTab, setCurrentTab] = useState(0);

  const handleTabChange = (_event: React.SyntheticEvent, newValue: number) => {
    setCurrentTab(newValue);
  };

  return (
    <Box>
      <Box sx={{ mb: 4 }}>
        <Typography variant="h4" sx={{ fontWeight: 700, color: '#667eea', mb: 1 }}>
          Settings
        </Typography>
        <Typography variant="body1" color="text.secondary">
          Manage your account preferences and security settings
        </Typography>
      </Box>

      <Box sx={{ borderBottom: 1, borderColor: 'divider' }}>
        <Tabs value={currentTab} onChange={handleTabChange} aria-label="settings tabs">
          <Tab label="Security" id="settings-tab-0" aria-controls="settings-tabpanel-0" />
        </Tabs>
      </Box>

      <TabPanel value={currentTab} index={0}>
        <TwoFASettings />
      </TabPanel>
    </Box>
  );
}
