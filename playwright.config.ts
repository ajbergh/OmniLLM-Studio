import fs from 'node:fs';
import path from 'node:path';
import { defineConfig, devices } from '@playwright/test';

const backendPort = 8090;
const frontendPort = 4173;
const backendUrl = `http://127.0.0.1:${backendPort}`;
const frontendUrl = `http://127.0.0.1:${frontendPort}`;
const smokeRoot = path.resolve(__dirname, 'backend', 'test-results', 'playwright-smoke');
const smokeDbPath = path.join(smokeRoot, 'smoke.db');
const smokeAttachmentsDir = path.join(smokeRoot, 'attachments');

if (process.env.TEST_WORKER_INDEX === undefined) {
  fs.mkdirSync(smokeRoot, { recursive: true });
  
  // Helper function to remove file with retry logic
  const removeFileWithRetry = (filePath: string, maxRetries = 5) => {
    for (let i = 0; i < maxRetries; i++) {
      try {
        if (fs.existsSync(filePath)) {
          fs.rmSync(filePath, { force: true });
        }
        return;
      } catch (error: any) {
        if (i < maxRetries - 1 && error.code === 'EBUSY') {
          // Wait before retrying
          const delay = Math.min(100 * Math.pow(2, i), 1000);
          const start = Date.now();
          while (Date.now() - start < delay) {
            // Busy wait
          }
        } else {
          throw error;
        }
      }
    }
  };
  
  for (const dbFile of [smokeDbPath, `${smokeDbPath}-shm`, `${smokeDbPath}-wal`]) {
    removeFileWithRetry(dbFile);
  }
  fs.rmSync(smokeAttachmentsDir, { recursive: true, force: true });
  fs.mkdirSync(smokeAttachmentsDir, { recursive: true });
  fs.closeSync(fs.openSync(smokeDbPath, 'w'));
}

process.env.OMNILLM_PLAYWRIGHT_BACKEND_URL = backendUrl;
process.env.OMNILLM_PLAYWRIGHT_FRONTEND_URL = frontendUrl;
process.env.OMNILLM_PLAYWRIGHT_DB_PATH = smokeDbPath;
process.env.OMNILLM_PLAYWRIGHT_ATTACHMENTS_DIR = smokeAttachmentsDir;

/**
 * Read environment variables from file.
 * https://github.com/motdotla/dotenv
 */
// import dotenv from 'dotenv';
// import path from 'path';
// dotenv.config({ path: path.resolve(__dirname, '.env') });

/**
 * See https://playwright.dev/docs/test-configuration.
 */
export default defineConfig({
  testDir: './tests',
  /* Run tests in files in parallel */
  fullyParallel: true,
  /* Fail the build on CI if you accidentally left test.only in the source code. */
  forbidOnly: !!process.env.CI,
  /* Retry on CI only */
  retries: process.env.CI ? 2 : 0,
  /* Opt out of parallel tests on CI. */
  workers: process.env.CI ? 1 : undefined,
  /* Reporter to use. See https://playwright.dev/docs/test-reporters */
  reporter: 'html',
  /* Shared settings for all the projects below. See https://playwright.dev/docs/api/class-testoptions. */
  use: {
    /* Base URL to use in actions like `await page.goto('')`. */
    baseURL: frontendUrl,

    /* Collect trace when retrying the failed test. See https://playwright.dev/docs/trace-viewer */
    trace: 'on-first-retry',
  },

  /* Configure projects for major browsers */
  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
    },

    {
      name: 'firefox',
      use: { ...devices['Desktop Firefox'] },
    },

    // {
    //   name: 'webkit',
    //   use: { ...devices['Desktop Safari'] },
    // },

    /* Test against mobile viewports. */
    // {
    //   name: 'Mobile Chrome',
    //   use: { ...devices['Pixel 5'] },
    // },
    // {
    //   name: 'Mobile Safari',
    //   use: { ...devices['iPhone 12'] },
    // },

    /* Test against branded browsers. */
    // {
    //   name: 'Microsoft Edge',
    //   use: { ...devices['Desktop Edge'], channel: 'msedge' },
    // },
    // {
    //   name: 'Google Chrome',
    //   use: { ...devices['Desktop Chrome'], channel: 'chrome' },
    // },
  ],

  /* Run your local dev server before starting the tests */
  webServer: [
    {
      command: 'go run ./cmd/server',
      cwd: path.resolve(__dirname, 'backend'),
      url: `${backendUrl}/v1/health`,
      reuseExistingServer: false,
      timeout: 120_000,
      env: {
        ...process.env,
        OMNILLM_PORT: String(backendPort),
        OMNILLM_BIND_ADDRESS: '127.0.0.1',
        OMNILLM_DB_PATH: smokeDbPath,
        OMNILLM_ATTACHMENTS_DIR: smokeAttachmentsDir,
      },
    },
    {
      command: `npm run dev -- --host 127.0.0.1 --port ${frontendPort}`,
      cwd: path.resolve(__dirname, 'frontend'),
      url: frontendUrl,
      reuseExistingServer: false,
      timeout: 120_000,
      env: {
        ...process.env,
        OMNILLM_API_PROXY_TARGET: backendUrl,
      },
    },
  ],
});
