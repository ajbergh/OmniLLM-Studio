import { execFileSync } from 'node:child_process';
import path from 'node:path';
import { expect, test } from '@playwright/test';

interface SeedFixture {
  title: string;
  conversation_id: string;
  session_id: string;
  node_id: string;
  asset_id: string;
  attachment_id: string;
}

function seedImageSessionFixture(title: string): SeedFixture {
  const repoRoot = process.cwd();
  const backendDir = path.join(repoRoot, 'backend');
  const fixtureImage = path.join(backendDir, 'cmd', 'desktop', 'build', 'appicon.png');

  const raw = execFileSync(
    'go',
    ['run', './cmd/playwrightseed', '--title', title, '--image', fixtureImage],
    {
      cwd: backendDir,
      encoding: 'utf-8',
      env: {
        ...process.env,
        OMNILLM_DB_PATH: process.env.OMNILLM_PLAYWRIGHT_DB_PATH,
        OMNILLM_ATTACHMENTS_DIR: process.env.OMNILLM_PLAYWRIGHT_ATTACHMENTS_DIR,
      },
    }
  );

  return JSON.parse(raw.trim()) as SeedFixture;
}

test('floating canvas toolbar responds in image edit mode', async ({ page, browserName }) => {
  const sessionTitle = `Toolbar Smoke ${browserName} ${Date.now()}`;
  const fixture = seedImageSessionFixture(sessionTitle);

  await page.addInitScript(() => {
    window.localStorage.clear();
  });

  await page.goto('/');

  await page.getByRole('button', { name: 'Image Studio' }).click();
  await expect(page.getByText(fixture.title)).toBeVisible();

  await page.getByText(fixture.title).click();
  await expect(page.getByText('Image Edit Studio')).toBeVisible();

  await page.getByRole('button', { name: 'Edit' }).click();

  const toolbar = page.getByTestId('image-canvas-toolbar');
  const zoomValue = page.getByTestId('canvas-zoom-value');
  const panButton = page.getByTestId('canvas-tool-pan');
  const maskButton = page.getByTestId('canvas-toggle-mask');
  const maskCanvas = page.getByTestId('image-mask-canvas');

  await expect(toolbar).toBeVisible();
  await expect(zoomValue).toHaveText('100%');
  await expect(maskCanvas).toHaveCSS('opacity', '1');

  await page.getByTestId('canvas-zoom-in').click();
  await expect(zoomValue).toHaveText('125%');

  await panButton.click();
  await expect(panButton).toHaveClass(/bg-primary\/20/);

  await maskButton.click();
  await expect(maskCanvas).toHaveCSS('opacity', '0');
});
