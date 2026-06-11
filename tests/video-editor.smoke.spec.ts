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

// Right-click context menus: clip menu duplicates and deletes through the
// shared ContextMenu component, and clipboard copy/paste round-trips a clip.
test('video edit studio clip context menu and clipboard', async ({ page }) => {
  await page.addInitScript(() => {
    window.localStorage.clear();
  });

  await page.goto('/');
  await page.getByRole('button', { name: 'Edit', exact: true }).click();
  await page.getByRole('button', { name: 'Project', exact: true }).click();
  await expect(page.getByRole('button', { name: 'Save timeline' })).toBeVisible();

  // Seed one text clip; it lands on the topmost layer. Timeline clips are the
  // draggable elements — the preview canvas renders the same text separately.
  await page.getByRole('button', { name: 'Text', exact: true }).click();
  const clips = page.locator('[draggable="true"][title="Title card"]');
  await expect(clips).toHaveCount(1);

  // Duplicate via the clip context menu.
  await clips.first().click({ button: 'right' });
  await page.getByRole('menuitem', { name: 'Duplicate', exact: true }).click();
  await expect(clips).toHaveCount(2);

  // Copy, then paste at the playhead — a third clip appears. The pasted clip
  // lands at the playhead (0ms) covering the original, so later steps target
  // the non-overlapped duplicate at the end of the lane.
  await clips.last().click({ button: 'right' });
  await page.getByRole('menuitem', { name: 'Copy', exact: true }).click();
  await clips.last().click({ button: 'right' });
  await page.getByRole('menuitem', { name: 'Paste at playhead' }).click();
  await expect(clips).toHaveCount(3);

  // Delete one through the menu (danger item).
  await clips.last().click({ button: 'right' });
  await page.getByRole('menuitem', { name: 'Delete', exact: true }).click();
  await expect(clips).toHaveCount(2);

  // Layer header context menu offers layer management commands.
  await page.getByText('Layer 4', { exact: true }).click({ button: 'right' });
  await expect(page.getByRole('menuitem', { name: 'Duplicate layer' })).toBeVisible();
  await page.keyboard.press('Escape');
});
