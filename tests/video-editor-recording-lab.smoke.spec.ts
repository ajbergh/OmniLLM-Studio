import { expect, test } from '@playwright/test';

test('video edit studio exposes combined screen and camera recording controls', async ({ page }) => {
  await page.addInitScript(() => window.localStorage.clear());
  await page.goto('/');

  await page.getByRole('button', { name: 'Video Edit Studio', exact: true }).click();
  await page.getByRole('button', { name: 'Project', exact: true }).click();
  await expect(page.getByRole('button', { name: 'Save timeline' })).toBeVisible();

  await page.getByRole('button', { name: 'Open recording lab' }).click();
  await expect(page.getByRole('dialog', { name: 'Recording lab' })).toBeVisible();
  await expect(page.getByRole('button', { name: 'Screen + camera' })).toBeVisible();
  await expect(page.getByRole('button', { name: 'Screen' })).toBeVisible();
  await expect(page.getByRole('button', { name: 'Camera' })).toBeVisible();
  await expect(page.getByRole('button', { name: 'Voiceover' })).toBeVisible();
  await expect(page.getByText('1080p')).toBeVisible();
  await expect(page.getByText('30 FPS')).toBeVisible();
  await expect(page.getByText('Camera position')).toBeVisible();
  await expect(page.getByText('Capture live browser transcript')).toBeVisible();

  await page.getByRole('button', { name: 'Close recording lab' }).click();
  await expect(page.getByRole('dialog', { name: 'Recording lab' })).toHaveCount(0);
});
