import { expect, test } from '@playwright/test';

test('Gemini Omni exposes guided video workflows and only supported controls', async ({ page }) => {
  await page.addInitScript(() => window.localStorage.clear());
  await page.goto('/');

  await page.getByRole('button', { name: 'Video Studio' }).click();
  await expect(page.getByText('Create Video')).toBeVisible();

  await page.getByLabel('Provider').selectOption('gemini');
  await page.locator('select').filter({ has: page.locator('option[value="gemini-omni-flash-preview"]') }).selectOption('gemini-omni-flash-preview');

  const modePicker = page.getByTestId('omni-mode-picker');
  await expect(modePicker).toBeVisible();
  await expect(modePicker.getByRole('radio')).toHaveCount(4);
  await expect(modePicker.getByRole('radio', { name: /Create/ })).toHaveAttribute('aria-checked', 'true');
  await expect(page.getByText(/synchronized audio track by default/i)).toBeVisible();
  await expect(page.getByLabel('Aspect')).toBeVisible();
  await expect(page.getByLabel('Resolution')).toHaveCount(0);
  await expect(page.getByLabel('FPS')).toHaveCount(0);

  await modePicker.getByRole('radio', { name: /Animate/ }).click();
  await expect(page.getByText('Image to animate', { exact: true })).toBeVisible();
  await expect(page.getByText(/motion, camera movement, and environmental effects/i)).toBeVisible();

  await modePicker.getByRole('radio', { name: /References/ }).click();
  await expect(page.getByText('Visual references (0/6)')).toBeVisible();
  await expect(page.getByText(/tagged in upload order/i)).toBeVisible();

  await modePicker.getByRole('radio', { name: /Edit/ }).click();
  await expect(page.getByText('Video to edit')).toBeVisible();
  await expect(page.getByText(/uploaded video editing is region-limited/i)).toBeVisible();
  await expect(page.getByRole('button', { name: 'Keep edit focused' })).toBeDisabled();
  await expect(page.getByRole('button', { name: 'Apply edit' })).toBeDisabled();
});
