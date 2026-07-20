import { expect, test } from '@playwright/test';

test('video edit studio exposes source monitor and media relink workflows', async ({ page }) => {
  await page.addInitScript(() => window.localStorage.clear());
  await page.goto('/');

  await page.getByRole('button', { name: 'Video Edit Studio', exact: true }).click();
  await page.getByRole('button', { name: 'Project', exact: true }).click();
  await expect(page.getByRole('button', { name: 'Save timeline' })).toBeVisible();

  await page.getByRole('button', { name: 'Open source monitor' }).click();
  await expect(page.getByRole('dialog', { name: 'Source monitor' })).toBeVisible();
  await expect(page.getByText('Mark In')).toBeVisible();
  await expect(page.getByText('Mark Out')).toBeVisible();
  await expect(page.getByRole('button', { name: 'Insert at playhead' })).toBeVisible();
  await expect(page.getByRole('button', { name: 'Overwrite' })).toBeVisible();
  await page.getByRole('button', { name: 'Close source monitor' }).click();

  await page.getByRole('button', { name: 'Open media relink lab' }).click();
  await expect(page.getByRole('dialog', { name: 'Media relink lab' })).toBeVisible();
  await expect(page.getByText('Replace media while preserving edits')).toBeVisible();
  await expect(page.getByText('Reference health')).toBeVisible();
  await expect(page.getByRole('button', { name: 'Download manifest' })).toBeVisible();
  await page.getByRole('button', { name: 'Close media relink lab' }).click();
});
