import { defineConfig } from '@playwright/test';
export default defineConfig({
  testDir: './test/browser',
  workers: 1,
  timeout: 30000,
  use: { browserName: 'chromium', viewport: { width: 1440, height: 1000 }, trace: 'retain-on-failure' },
});
