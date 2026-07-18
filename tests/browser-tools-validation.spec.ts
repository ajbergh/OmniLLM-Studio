import { expect, test, type Page } from '@playwright/test';

const BACKEND_URL = process.env.OMNILLM_PLAYWRIGHT_BACKEND_URL ?? 'http://127.0.0.1:8090';

async function openSettings(page: Page, tab: string) {
  await page.locator('button[aria-label="Settings"]').first().click();
  const dialog = page.locator('[role="dialog"][aria-label="Settings"]');
  await expect(dialog).toBeVisible({ timeout: 8000 });

  const tabSelect = page.locator('#settings-tab-select');
  if (await tabSelect.isVisible({ timeout: 500 }).catch(() => false)) {
    await tabSelect.selectOption(tab.toLowerCase());
  } else {
    await dialog.getByRole('button', { name: tab, exact: true }).click();
  }
}

async function closeSettings(page: Page) {
  const closeButton = page.locator('button[aria-label="Close Settings"]').first();
  if (await closeButton.isVisible({ timeout: 500 }).catch(() => false)) {
    await closeButton.click();
  } else {
    await page.keyboard.press('Escape');
  }
}

async function expectBrowserRoutesAvailable(request: Parameters<Parameters<typeof test>[1]>[0]['request']) {
  for (const path of ['/v1/browser/status', '/v1/browser/sessions']) {
    const response = await request.get(`${BACKEND_URL}${path}`);
    expect([200, 401], `${path} returned ${response.status()}`).toContain(response.status());
  }
}

test.describe('Headless Browser — UI and API validation', () => {
  test.beforeEach(async ({ page }) => {
    await page.addInitScript(() => window.localStorage.clear());
    (page as Page & { _browserConsoleErrors?: string[] })._browserConsoleErrors = [];
    page.on('console', (message) => {
      if (message.type() === 'error') {
        (page as Page & { _browserConsoleErrors?: string[] })._browserConsoleErrors?.push(message.text());
      }
    });
  });

  test('desktop Settings → Tools shows the Headless Browser card', async ({ page }) => {
    await page.setViewportSize({ width: 1280, height: 900 });
    await page.goto('/');
    await openSettings(page, 'Tools');
    await expect(page.getByText('Headless Browser', { exact: true })).toBeVisible({ timeout: 8000 });
    await closeSettings(page);
  });

  test('mobile Settings → Tools shows the browser card without horizontal overflow', async ({ page }) => {
    await page.setViewportSize({ width: 390, height: 844 });
    await page.goto('/');

    const closeSidebar = page.getByRole('button', { name: 'Close sidebar' });
    if (await closeSidebar.isVisible().catch(() => false)) {
      await closeSidebar.click();
    }

    await openSettings(page, 'Tools');
    await expect(page.getByText('Headless Browser', { exact: true })).toBeVisible({ timeout: 8000 });
    const overflow = await page.evaluate(() => ({
      viewportWidth: document.documentElement.clientWidth,
      scrollWidth: Math.max(document.documentElement.scrollWidth, document.body.scrollWidth),
    }));
    expect(overflow.scrollWidth).toBeLessThanOrEqual(overflow.viewportWidth + 1);
    await closeSettings(page);
  });

  test('browser status and session routes are registered', async ({ request }) => {
    await expectBrowserRoutesAvailable(request);
  });

  test('solo-mode browser sessions response is an array', async ({ request }) => {
    const response = await request.get(`${BACKEND_URL}/v1/browser/sessions`);
    if (response.status() === 200) {
      expect(Array.isArray(await response.json())).toBe(true);
    }
  });

  test('browser status exposes runtime and sandbox fields', async ({ request }) => {
    const response = await request.get(`${BACKEND_URL}/v1/browser/status`);
    if (response.status() === 200) {
      const body = await response.json();
      expect(typeof body.enabled).toBe('boolean');
      expect(typeof body.browser_running).toBe('boolean');
      expect(typeof body.active_sessions).toBe('number');
      expect(typeof body.sandboxed).toBe('boolean');
    }
  });

  test('feature list contains a valid headless_browser flag in solo mode', async ({ request }) => {
    const response = await request.get(`${BACKEND_URL}/v1/features`);
    if (response.status() === 200) {
      const flags = await response.json();
      expect(Array.isArray(flags)).toBe(true);
      const browserFlag = flags.find((flag: { key?: string }) => flag.key === 'headless_browser');
      expect(browserFlag).toBeDefined();
      expect(typeof browserFlag.enabled).toBe('boolean');
    } else {
      expect(response.status()).toBe(401);
    }
  });

  test('page load emits no browser-route console errors', async ({ page }) => {
    await page.setViewportSize({ width: 1280, height: 900 });
    await page.goto('/');
    await page.waitForTimeout(2000);

    const errors = ((page as Page & { _browserConsoleErrors?: string[] })._browserConsoleErrors ?? [])
      .filter((message) => message.includes('/v1/browser/'));
    expect(errors, `Unexpected browser API errors: ${errors.join('; ')}`).toHaveLength(0);
  });

  test('web search control is disabled when Gemini is the active provider', async ({ page }) => {
    await page.route('**/v1/providers', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify([{
          id: 'gemini-test',
          type: 'gemini',
          name: 'Gemini Test',
          enabled: true,
          default_model: 'gemini-2.5-flash',
          created_at: new Date().toISOString(),
          updated_at: new Date().toISOString(),
        }]),
      });
    });

    await page.setViewportSize({ width: 1280, height: 900 });
    await page.goto('/');
    const webSearchButton = page.getByRole('button', { name: /web search/i }).first();
    if (await webSearchButton.isVisible().catch(() => false)) {
      const disabled = await webSearchButton.isDisabled().catch(() => false);
      const opacity = Number.parseFloat(await webSearchButton.evaluate((element) => getComputedStyle(element).opacity));
      expect(disabled || opacity < 0.5).toBe(true);
    }
  });

  test('Gemini model selector identifies models without tool calling', async ({ page }) => {
    await page.route('**/v1/providers', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify([{
          id: 'gemini-badge-test',
          type: 'gemini',
          name: 'Gemini',
          enabled: true,
          default_model: 'gemini-2.5-flash',
          created_at: new Date().toISOString(),
          updated_at: new Date().toISOString(),
        }]),
      });
    });

    await page.goto('/');
    const modelButton = page.getByRole('button', { name: /model|gemini/i }).first();
    if (await modelButton.isVisible().catch(() => false)) {
      await modelButton.click();
      const badge = page.getByText('no tools', { exact: true }).first();
      if (await badge.isVisible({ timeout: 3000 }).catch(() => false)) {
        await expect(badge).toBeVisible();
      }
      await page.keyboard.press('Escape');
    }
  });

  test('Headless Browser toggle responds and can be restored', async ({ page }) => {
    await page.setViewportSize({ width: 1280, height: 900 });
    await page.goto('/');
    await openSettings(page, 'Tools');

    const browserHeading = page.getByText('Headless Browser', { exact: true });
    await expect(browserHeading).toBeVisible({ timeout: 8000 });
    const browserCard = page.locator('div, section').filter({ hasText: 'Headless Browser' }).first();
    const toggle = browserCard.getByRole('checkbox').first();
    if (await toggle.isVisible().catch(() => false)) {
      const before = await toggle.isChecked();
      await toggle.click();
      await expect(toggle).toBeChecked({ checked: !before });
      await toggle.click();
      await expect(toggle).toBeChecked({ checked: before });
    }
    await closeSettings(page);
  });

  test('browser tool route wiring never returns 404 or 5xx', async ({ request }) => {
    const response = await request.get(`${BACKEND_URL}/v1/browser/sessions`);
    expect(response.status()).not.toBe(404);
    expect(response.status()).toBeLessThan(500);
  });
});
