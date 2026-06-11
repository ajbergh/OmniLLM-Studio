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

  // Parallel workers seed against the same SQLite file, so retry on SQLITE_BUSY.
  let lastError: unknown;
  for (let attempt = 0; attempt < 5; attempt++) {
    try {
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
    } catch (error) {
      lastError = error;
      const message = String(error);
      if (!message.includes('database is locked') && !message.includes('SQLITE_BUSY')) throw error;
      const waitMs = 250 * (attempt + 1);
      const start = Date.now();
      while (Date.now() - start < waitMs) {
        // busy-wait; execFileSync flows are synchronous
      }
    }
  }
  throw lastError;
}

test('floating canvas toolbar responds in image edit mode', async ({ page, browserName }) => {
  const sessionTitle = `Toolbar Smoke ${browserName} ${Date.now()}`;
  const fixture = seedImageSessionFixture(sessionTitle);

  await page.addInitScript(() => {
    window.localStorage.clear();
  });

  await page.goto('/');

  await page.getByRole('button', { name: 'Image', exact: true }).click();
  await expect(page.getByText(fixture.title).first()).toBeVisible();

  await page.getByText(fixture.title).first().click();
  await expect(page.getByText('Image Edit Studio')).toBeVisible();

  // The sidebar also has an exact "Edit" button (Video Edit Studio); the studio's
  // Edit mode tab renders later in the DOM.
  await page.getByRole('button', { name: 'Edit', exact: true }).last().click();

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

test('AI enhance rewrites and can undo an image studio prompt', async ({ page, browserName }) => {
  const sessionTitle = `Prompt Enhance ${browserName} ${Date.now()}`;
  const fixture = seedImageSessionFixture(sessionTitle);
  const originalPrompt = 'cozy cabin in the woods';
  const enhancedPrompt = 'A cozy cedar cabin tucked into a misty pine forest, warm window light, textured snow, cinematic dusk composition, soft volumetric lighting, natural color palette, highly detailed.';
  let requestBody: Record<string, unknown> | undefined;

  await page.addInitScript(() => {
    window.localStorage.clear();
  });

  await page.route('**/v1/conversations/*/images/sessions/*/enhance-prompt', async (route) => {
    requestBody = route.request().postDataJSON() as Record<string, unknown>;
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        prompt: enhancedPrompt,
        original_prompt: originalPrompt,
        provider: 'test',
        model: 'test-model',
      }),
    });
  });

  await page.goto('/');

  await page.getByRole('button', { name: 'Image', exact: true }).click();
  await expect(page.getByText(fixture.title).first()).toBeVisible();

  await page.getByText(fixture.title).first().click();
  await expect(page.getByText('Image Edit Studio')).toBeVisible();

  const promptInput = page.getByPlaceholder('Describe the image you want to generate...');
  await promptInput.fill(originalPrompt);
  await page.getByRole('button', { name: /AI enhance prompt/i }).click();

  await expect(promptInput).toHaveValue(enhancedPrompt);
  expect(requestBody).toMatchObject({
    prompt: originalPrompt,
    mode: 'generate',
  });

  await page.getByRole('button', { name: 'Undo AI enhance' }).click();
  await expect(promptInput).toHaveValue(originalPrompt);
});
