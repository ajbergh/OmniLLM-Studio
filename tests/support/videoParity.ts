import type { Locator, Page } from '@playwright/test';

export interface VideoParitySample {
  name: string;
  frame_index: number;
  time_ms: number;
  reason: string;
}

/** Seek the canonical program monitor to a named frame and wait for its
 * render commit before taking a locator-only screenshot. */
export async function captureVideoParityFrame(
  page: Page,
  sample: VideoParitySample,
  outputPath: string,
): Promise<Locator> {
  const requestId = `parity-${sample.frame_index}-${sample.name}`;
  await page.evaluate(({ frameIndex, id }) => new Promise<void>((resolve) => {
    const ready = (event: Event) => {
      const detail = (event as CustomEvent<{ requestId?: string }>).detail;
      if (detail?.requestId !== id) return;
      window.removeEventListener('omnillm:video-parity-ready', ready);
      resolve();
    };
    window.addEventListener('omnillm:video-parity-ready', ready);
    window.dispatchEvent(new CustomEvent('omnillm:video-parity-seek', {
      detail: { frameIndex, requestId: id },
    }));
  }), { frameIndex: sample.frame_index, id: requestId });

  const program = page.getByTestId('video-preview-program');
  await program.waitFor({ state: 'visible' });
  await page.waitForFunction((frameIndex) => {
    const node = document.querySelector('[data-testid="video-preview-program"]');
    return node?.getAttribute('data-parity-frame-index') === String(frameIndex);
  }, sample.frame_index);
  await program.screenshot({ path: outputPath, animations: 'disabled' });
  return program;
}
