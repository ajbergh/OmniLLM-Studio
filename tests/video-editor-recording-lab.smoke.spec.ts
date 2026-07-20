import { expect, test } from '@playwright/test';

test('video edit studio exposes combined screen and camera recording controls', async ({ page }) => {
  await page.addInitScript(() => window.localStorage.clear());
  await page.goto('/');

  await page.getByRole('button', { name: 'Video Edit Studio', exact: true }).click();
  await page.getByRole('button', { name: 'Project', exact: true }).click();
  await expect(page.getByRole('button', { name: 'Save timeline' })).toBeVisible();

  await page.getByRole('button', { name: 'Open recording lab' }).click();
  const dialog = page.getByRole('dialog', { name: 'Recording lab' });
  await expect(dialog).toBeVisible();
  await expect(dialog.getByRole('button', { name: 'Screen + camera', exact: true })).toBeVisible();
  await expect(dialog.getByRole('button', { name: 'Screen', exact: true })).toBeVisible();
  await expect(dialog.getByRole('button', { name: 'Camera', exact: true })).toBeVisible();
  await expect(dialog.getByRole('button', { name: 'Voiceover', exact: true })).toBeVisible();
  await expect(dialog.getByRole('combobox', { name: 'Resolution' })).toHaveValue('1080p');
  await expect(dialog.getByRole('combobox', { name: 'Frame rate' })).toHaveValue('30');
  await expect(dialog.getByRole('combobox', { name: 'Camera position' })).toHaveValue('bottom-right');
  await expect(dialog.getByRole('checkbox', { name: 'Capture live browser transcript' })).toBeVisible();

  await dialog.getByRole('button', { name: 'Close recording lab' }).click();
  await expect(dialog).toHaveCount(0);
});
