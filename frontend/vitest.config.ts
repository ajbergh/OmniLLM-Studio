import { defineConfig } from 'vitest/config';

// Store-level unit tests run in plain node — the video studio store guards
// all window/localStorage access, and saves no-op without an active project.
export default defineConfig({
  test: {
    environment: 'node',
    include: ['src/**/*.test.{ts,tsx}'],
  },
});
