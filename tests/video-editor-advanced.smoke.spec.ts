import { expect, test } from '@playwright/test';

test('video edit studio advanced tools support range and timeline versions', async ({ page }) => {
  await page.addInitScript(() => window.localStorage.clear());
  await page.goto('/');

  await page.getByRole('button', { name: 'Video Edit Studio', exact: true }).click();
  await page.getByRole('button', { name: 'Project', exact: true }).click();
  await expect(page.getByRole('button', { name: 'Save timeline' })).toBeVisible();

  await page.getByRole('button', { name: 'Open advanced timeline tools' }).click();
  await expect(page.getByLabel('Advanced video editing tools')).toBeVisible();

  await page.getByRole('button', { name: 'Set In' }).click();
  await page.getByRole('button', { name: 'Set Out' }).click();
  await expect(page.getByText('Out point must be after the in point')).toBeVisible();

  await page.getByRole('button', { name: 'Versions' }).click();
  await page.getByPlaceholder('Version 1').fill('Before social cut');
  await page.getByRole('button', { name: 'Create version' }).click();
  await expect(page.getByText('Before social cut', { exact: true })).toBeVisible();
});

test('video edit studio converts pasted transcript into captions', async ({ page }) => {
  await page.addInitScript(() => window.localStorage.clear());
  await page.goto('/');

  await page.getByRole('button', { name: 'Video Edit Studio', exact: true }).click();
  await page.getByRole('button', { name: 'Project', exact: true }).click();
  await expect(page.getByRole('button', { name: 'Save timeline' })).toBeVisible();

  await page.getByRole('button', { name: 'Open advanced timeline tools' }).click();
  await page.getByRole('button', { name: 'Transcript' }).click();
  await page.getByPlaceholder('Paste transcript text…').fill('First caption line.\nSecond caption line.');
  await page.getByRole('button', { name: 'Create captions' }).click();

  await page.getByRole('button', { name: 'Close advanced tools' }).click();
  await page.getByRole('tab', { name: 'Captions' }).click();
  await expect(page.getByLabel('Caption 1 text')).toHaveValue('First caption line.');
  await expect(page.getByLabel('Caption 2 text')).toHaveValue('Second caption line.');
});
