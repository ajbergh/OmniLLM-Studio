import { expect, test } from '@playwright/test';

test('music studio renders Lyria-only controls without console errors', async ({ page }) => {
  const consoleErrors: string[] = [];
  page.on('console', (message) => {
    if (message.type() === 'error') {
      consoleErrors.push(message.text());
    }
  });

  await page.addInitScript(() => {
    window.localStorage.clear();
  });

  await page.route('**/v1/music/providers', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ openrouter: true, gemini: true }),
    });
  });

  await page.route('**/v1/music/sessions', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify([]),
    });
  });

  await page.route('**/v1/music/models**', async (route) => {
    const url = new URL(route.request().url());
    const provider = url.searchParams.get('provider');
    const models = provider === 'gemini'
      ? [
          { id: 'lyria-3-clip-preview', provider: 'gemini', name: 'Lyria 3 Clip (Preview)', capabilities: ['text_to_music'], output_modalities: ['audio', 'text'], supports_streaming: false },
          { id: 'lyria-3-pro-preview', provider: 'gemini', name: 'Lyria 3 Pro (Preview)', capabilities: ['text_to_music'], output_modalities: ['audio', 'text'], supports_streaming: false },
        ]
      : [
          { id: 'google/lyria-3-clip-preview', provider: 'openrouter', name: 'Lyria 3 Clip (Preview)', capabilities: ['text_to_music'], output_modalities: ['audio', 'text'], supports_streaming: true },
          { id: 'google/lyria-3-pro-preview', provider: 'openrouter', name: 'Lyria 3 Pro (Preview)', capabilities: ['text_to_music'], output_modalities: ['audio', 'text'], supports_streaming: true },
        ];
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(models),
    });
  });

  await page.goto('/');
  await page.getByRole('button', { name: 'Music Studio' }).click();

  await expect(page.getByText('Music Studio').first()).toBeVisible();
  await expect(page.getByText('Describe a song and generate to start.')).toBeVisible();
  await expect(page.getByRole('combobox', { name: 'Music provider' })).toHaveValue('openrouter');
  await expect(page.getByRole('combobox', { name: 'Lyria model' })).toHaveValue('google/lyria-3-clip-preview');

  await page.getByRole('combobox', { name: 'Music provider' }).selectOption('gemini');
  await expect(page.getByRole('combobox', { name: 'Lyria model' })).toHaveValue('lyria-3-clip-preview');

  const generateButton = page.getByRole('button', { name: /Generate Track/i });
  await expect(generateButton).toBeDisabled();
  await page.getByPlaceholder('Describe the track...').fill('Upbeat synth pop track with bright drums');
  await expect(generateButton).toBeEnabled();

  expect(consoleErrors).toEqual([]);
});
