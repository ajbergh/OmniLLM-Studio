import { expect, test } from '@playwright/test';

test('motion-design mode is reachable and does not mutate persisted timeline content', async ({ page }) => {
  const timelineWrites: unknown[] = [];
  page.on('request', (request) => {
    if (request.method() === 'PUT' && request.url().includes('/timeline')) timelineWrites.push(request.postDataJSON());
  });
  await page.addInitScript(() => window.localStorage.clear());
  await page.goto('/');
  await page.getByRole('button', { name: 'Video Edit Studio', exact: true }).click();
  await page.getByRole('button', { name: 'Project', exact: true }).click();
  await expect(page.getByRole('button', { name: 'Save timeline' })).toBeVisible();
  await page.getByRole('button', { name: 'Text', exact: true }).click();
  await expect.poll(() => timelineWrites.length).toBeGreaterThan(0);

  const before = structuredClone(timelineWrites.at(-1));
  const writesBeforeModeChange = timelineWrites.length;
  await page.getByLabel('Editor mode').selectOption('motion_design');
  for (const name of ['Design', 'Animate', 'Effects', 'AI', 'Export']) {
    await expect(page.getByRole('tab', { name, exact: true })).toBeVisible();
  }
  await page.getByRole('tab', { name: 'Animate', exact: true }).click();
  await expect(page.getByTestId('animation-block-picker')).toBeVisible();
  const inPicker = page.getByTestId('animation-block-picker');
  await expect(inPicker.getByText(/^in$/i)).toBeVisible();
  await expect(inPicker.getByText(/^during$/i)).toBeVisible();
  await expect(inPicker.getByText(/^out$/i)).toBeVisible();

  await page.getByLabel('Editor mode').selectOption('full');
  await expect(page.getByRole('tab', { name: 'Properties', exact: true })).toBeVisible();
  await page.waitForTimeout(250);
  expect(timelineWrites).toHaveLength(writesBeforeModeChange);
  expect(timelineWrites.at(-1)).toEqual(before);
});
