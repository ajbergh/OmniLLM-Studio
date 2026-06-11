import { expect, test } from '@playwright/test';

// Core Video Edit Studio flow: open studio → create project → media bin +
// timeline render → add text clip → inspector controls → save → captions and
// export panels present. Runs against the isolated smoke backend (fresh DB).
test('video edit studio core editing flow', async ({ page }) => {
  await page.addInitScript(() => {
    window.localStorage.clear();
  });

  await page.goto('/');

  // Sidebar mode button (the image studio's "Edit" tab is not mounted here).
  await page.getByRole('button', { name: 'Edit', exact: true }).click();
  await expect(page.getByText('Video Edit Studio').first()).toBeVisible();

  // Create a project from the studio header.
  await page.getByRole('button', { name: 'Project', exact: true }).click();

  // Media bin and timeline shell appear once the project/timeline load.
  await expect(page.getByRole('heading', { name: 'Media Bin' })).toBeVisible();
  await expect(page.getByRole('button', { name: 'Add track' })).toBeVisible();
  await expect(page.getByRole('button', { name: 'Save timeline' })).toBeVisible();

  // Add a text clip (button enables once the timeline is loaded) and check
  // the inspector shows clip controls for the new selection.
  await page.getByRole('button', { name: 'Text', exact: true }).click();
  await expect(page.getByText(/Scale:/).first()).toBeVisible();
  await expect(page.getByText(/Layer order/).first()).toBeVisible();

  // Caption editor and export/render panel are present in the right sidebar.
  await expect(page.getByRole('heading', { name: 'Captions' })).toBeVisible();
  await expect(page.getByRole('heading', { name: 'Export' })).toBeVisible();

  // Save the timeline explicitly.
  await page.getByRole('button', { name: 'Save timeline' }).click();

  // Add a caption segment at the playhead and confirm it lists in the panel.
  await page.getByRole('button', { name: 'Add at playhead' }).click();
  await expect(page.getByLabel('Caption 1 text')).toHaveValue('New caption');
});
