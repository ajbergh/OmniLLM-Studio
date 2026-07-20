import { execFileSync } from 'node:child_process';
import path from 'node:path';
import { expect, test, type Page } from '@playwright/test';

interface ChatFixture {
  title: string;
  conversation_id: string;
  user_message_id: string;
  assistant_message_id: string;
}

interface ImageFixture {
  title: string;
  conversation_id: string;
  session_id: string;
  node_id: string;
  asset_id: string;
  attachment_id: string;
}

function seedChatFixture(title: string): ChatFixture {
  const repoRoot = process.cwd();
  const backendDir = path.join(repoRoot, 'backend');
  const raw = execFileSync('go', ['run', './cmd/playwrightseedchat', '--title', title], {
    cwd: backendDir,
    encoding: 'utf-8',
    env: {
      ...process.env,
      OMNILLM_DB_PATH: process.env.OMNILLM_PLAYWRIGHT_DB_PATH,
    },
  });

  return JSON.parse(raw.trim()) as ChatFixture;
}

function seedImageSessionFixture(title: string): ImageFixture {
  const repoRoot = process.cwd();
  const backendDir = path.join(repoRoot, 'backend');
  const fixtureImage = path.join(repoRoot, 'docs', 'assets', 'screenshots', 'chat-studio.png');
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

  return JSON.parse(raw.trim()) as ImageFixture;
}

async function resetClientState(page: Page) {
  await page.addInitScript(() => {
    window.localStorage.clear();
  });
}

async function openSidebarIfNeeded(page: Page) {
  const openSidebar = page.getByRole('button', { name: 'Open sidebar' });
  if (await openSidebar.isVisible().catch(() => false)) {
    await openSidebar.click();
  }
}

async function closeSidebarIfOpen(page: Page) {
  const closeSidebar = page.getByRole('button', { name: 'Close sidebar' });
  if (await closeSidebar.isVisible().catch(() => false)) {
    await closeSidebar.click();
  }
}

async function expectNoHorizontalOverflow(page: Page) {
  const overflow = await page.evaluate(() => {
    const viewportWidth = document.documentElement.clientWidth;
    const scrollWidth = Math.max(document.documentElement.scrollWidth, document.body.scrollWidth);
    const offenders = Array.from(document.querySelectorAll<HTMLElement>('body *'))
      .map((element) => {
        const rect = element.getBoundingClientRect();
        return {
          tag: element.tagName.toLowerCase(),
          className: element.className?.toString().slice(0, 120) || '',
          left: Math.floor(rect.left),
          right: Math.ceil(rect.right),
          width: Math.ceil(rect.width),
        };
      })
      .filter((item) => item.width > 0 && (item.left < -1 || item.right > viewportWidth + 1))
      .slice(0, 8);

    return { viewportWidth, scrollWidth, offenders };
  });

  expect(
    overflow.scrollWidth,
    `Horizontal overflow at ${overflow.viewportWidth}px: ${JSON.stringify(overflow.offenders, null, 2)}`
  ).toBeLessThanOrEqual(overflow.viewportWidth + 1);
}

test.describe.configure({ mode: 'serial' });

test('mobile global tools open as a labelled dialog and do not overflow', async ({ page }) => {
  await resetClientState(page);
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto('/');
  await closeSidebarIfOpen(page);

  await expect(page.getByRole('button', { name: 'Settings' })).toBeVisible();
  await expect(page.getByRole('button', { name: 'Open tools menu' })).toBeVisible();
  await page.getByRole('button', { name: 'Settings' }).click();
  await expect(page.getByRole('dialog', { name: 'Settings' })).toBeVisible();
  await page.getByRole('button', { name: 'Close Settings' }).click();

  await page.getByRole('button', { name: 'Open tools menu' }).click();

  const toolsDialog = page.getByRole('dialog', { name: 'Tools' });
  await expect(toolsDialog).toBeVisible();
  await expect(toolsDialog.getByRole('button', { name: 'Search conversations' })).toBeVisible();
  await expect(toolsDialog.getByRole('button', { name: 'Settings' })).toBeVisible();
  await expectNoHorizontalOverflow(page);

  await page.keyboard.press('Escape');
  await expect(toolsDialog).toBeHidden();
});

test('table-heavy chat stays within the mobile viewport and header controls are reachable', async ({ page }) => {
  const fixture = seedChatFixture(`UIUX Table Fixture ${Date.now()}`);
  await resetClientState(page);
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto('/');

  await openSidebarIfNeeded(page);
  await expect(page.getByText(fixture.title, { exact: true }).first()).toBeVisible();
  await page.getByText(fixture.title, { exact: true }).first().click();

  await expect(page.getByRole('button', { name: 'Manage attachments' })).toBeVisible();
  await expect(page.locator('.message-table-wrap').first()).toBeVisible();
  await expectNoHorizontalOverflow(page);

  const tableSizing = await page.locator('.message-table-wrap').first().evaluate((element) => ({
    clientWidth: element.clientWidth,
    scrollWidth: element.scrollWidth,
  }));
  expect(tableSizing.clientWidth).toBeLessThanOrEqual(390);
  expect(tableSizing.scrollWidth).toBeGreaterThanOrEqual(tableSizing.clientWidth);
});

test('desktop chat header controls remain clickable below the global tools header', async ({ page }) => {
  const fixture = seedChatFixture(`UIUX Header Fixture ${Date.now()}`);
  await resetClientState(page);
  await page.setViewportSize({ width: 1440, height: 900 });
  await page.goto('/');

  await page.getByText(fixture.title, { exact: true }).first().click();
  await page.getByRole('button', { name: 'Manage attachments' }).click();

  await expect(page.getByRole('dialog', { name: /Attachments/ })).toBeVisible();
  await page.keyboard.press('Escape');
  await expect(page.getByRole('dialog', { name: /Attachments/ })).toBeHidden();
});

test('mobile settings uses a section selector and pricing layout fits', async ({ page }) => {
  await resetClientState(page);
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto('/');
  await closeSidebarIfOpen(page);

  await page.getByRole('button', { name: 'Open tools menu' }).click();
  await page.getByRole('dialog', { name: 'Tools' }).getByRole('button', { name: 'Settings' }).click();

  await expect(page.getByRole('dialog', { name: 'Settings' })).toBeVisible();
  const sectionSelect = page.locator('#settings-tab-select');
  await expect(sectionSelect).toBeVisible();
  await sectionSelect.selectOption('pricing');
  await expect(page.getByRole('button', { name: 'Add Rule' })).toBeVisible();
  await expectNoHorizontalOverflow(page);
});

test('mobile image studio exposes prompt canvas and history without fixed-column overflow', async ({ page }) => {
  const fixture = seedImageSessionFixture(`UIUX Image Fixture ${Date.now()}`);
  await resetClientState(page);
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto('/');

  await openSidebarIfNeeded(page);
  await page.getByRole('button', { name: 'Image Studio' }).click();
  await expect(page.getByRole('button', { name: 'Close sidebar' })).toBeHidden();
  await expect(page.getByRole('button', { name: 'Open sidebar' })).toBeVisible();
  await page.getByRole('button', { name: 'Open sidebar' }).click();
  await expect(page.getByRole('button', { name: 'Close sidebar' })).toBeVisible();
  await expect(page.getByText(fixture.title, { exact: true }).first()).toBeVisible();
  await page.getByText(fixture.title, { exact: true }).first().click();

  await expect(page.getByText('Image Edit Studio')).toBeVisible();
  await expect(page.getByRole('button', { name: 'Prompt', exact: true })).toBeVisible();
  await page.getByRole('button', { name: 'Canvas', exact: true }).click();
  await expect(page.getByTestId('image-canvas-toolbar')).toBeVisible();
  await page.getByRole('button', { name: 'History', exact: true }).click();
  await expect(page.getByText(fixture.title).first()).toBeVisible();
  await expectNoHorizontalOverflow(page);
});

test('mobile media studios expose full-height workspace tabs', async ({ page }) => {
  await resetClientState(page);
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto('/');

  const switchMode = async (name: string) => {
    await openSidebarIfNeeded(page);
    await page.getByRole('button', { name }).click();
    await expect(page.getByRole('button', { name: 'Close sidebar' })).toBeHidden();
  };

  await switchMode('Music Studio');
  for (const name of ['Create', 'Result', 'History']) {
    await expect(page.getByRole('tab', { name, exact: true })).toBeVisible();
    await page.getByRole('tab', { name, exact: true }).click();
  }
  await expectNoHorizontalOverflow(page);

  await switchMode('Video Studio');
  for (const name of ['Create', 'Preview', 'History', 'Outputs']) {
    await expect(page.getByRole('tab', { name, exact: true })).toBeVisible();
    await page.getByRole('tab', { name, exact: true }).click();
  }
  await expectNoHorizontalOverflow(page);

  await switchMode('Video Edit Studio');
  for (const name of ['Preview', 'Timeline', 'Media', 'Inspector']) {
    await expect(page.getByRole('tab', { name, exact: true })).toBeVisible();
    await page.getByRole('tab', { name, exact: true }).click();
  }
  await expectNoHorizontalOverflow(page);
});

test('opening a global dialog dismisses the model selector', async ({ page }) => {
  const fixture = seedChatFixture(`UIUX Overlay Fixture ${Date.now()}`);
  await resetClientState(page);
  await page.setViewportSize({ width: 1440, height: 900 });
  await page.goto('/');
  await page.getByText(fixture.title, { exact: true }).first().click();

  await page.getByRole('button', { name: /^Model:/ }).click();
  await expect(page.getByRole('listbox', { name: 'Available models' })).toBeVisible();
  await page.getByRole('button', { name: 'Usage Dashboard' }).click();
  await expect(page.getByRole('dialog', { name: 'Usage & Cost Dashboard' })).toBeVisible();
  await expect(page.getByRole('listbox', { name: 'Available models' })).toBeHidden();
  await page.keyboard.press('Escape');
  await expect(page.getByRole('listbox', { name: 'Available models' })).toBeHidden();
});

test('slow conversation switching never renders the previous transcript under the new title', async ({ page }) => {
  const first = seedChatFixture(`UIUX Slow First ${Date.now()}`);
  const second = seedChatFixture(`UIUX Slow Second ${Date.now()}`);
  await resetClientState(page);
  await page.setViewportSize({ width: 1440, height: 900 });
  await page.goto('/');
  await page.getByText(first.title, { exact: true }).first().click();
  await expect(page.getByText('Show me a provider comparison table.', { exact: true })).toBeVisible();

  let releaseMessages!: () => void;
  const messagesGate = new Promise<void>((resolve) => {
    releaseMessages = resolve;
  });

  // Query parameters carry browser timezone/locale, so match the path plus any suffix.
  await page.route(`**/conversations/${second.conversation_id}/messages**`, async (route) => {
    await messagesGate;
    await route.continue();
  });

  const secondMessagesPath = `/conversations/${second.conversation_id}/messages`;
  const messagesRequest = page.waitForRequest((request) =>
    new URL(request.url()).pathname.endsWith(secondMessagesPath),
  );
  const messagesResponse = page.waitForResponse((response) =>
    new URL(response.url()).pathname.endsWith(secondMessagesPath),
  );

  await page.getByText(second.title, { exact: true }).first().click();
  await messagesRequest;

  try {
    await expect(page.getByRole('status').filter({ hasText: /loading conversation/i })).toBeVisible({ timeout: 5_000 });
    await expect(page.getByText('Show me a provider comparison table.', { exact: true })).toBeHidden();
  } finally {
    releaseMessages();
  }

  await messagesResponse;
  await expect(page.getByText('Show me a provider comparison table.', { exact: true })).toBeVisible();
});
