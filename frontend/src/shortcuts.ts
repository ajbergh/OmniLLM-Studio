export type ShortcutId =
  | 'newConversation'
  | 'openSettings'
  | 'openShortcuts'
  | 'openSearch'
  | 'sendMessage'
  | 'newLine'
  | 'stopGenerating'
  | 'toggleSidebar'
  | 'exportConversation';

export interface ShortcutBinding {
  id: ShortcutId;
  description: string;
  key: string;
  requiresMod?: boolean;
  shift?: boolean;
}

export const DEFAULT_BINDINGS: ShortcutBinding[] = [
  { id: 'newConversation', description: 'New conversation', key: 'n', requiresMod: true },
  { id: 'openSettings', description: 'Open settings', key: ',', requiresMod: true },
  { id: 'openShortcuts', description: 'Keyboard shortcuts', key: 'k', requiresMod: true },
  { id: 'openSearch', description: 'Open search', key: '/', requiresMod: true },
  { id: 'sendMessage', description: 'Send message', key: 'Enter' },
  { id: 'newLine', description: 'New line in message', key: 'Enter', shift: true },
  { id: 'stopGenerating', description: 'Stop generating', key: 'Escape' },
  { id: 'toggleSidebar', description: 'Toggle sidebar', key: 's', requiresMod: true, shift: true },
  { id: 'exportConversation', description: 'Export conversation', key: 'e', requiresMod: true },
];

const STORAGE_KEY = 'omnillm_custom_shortcuts';

type CustomBindingMap = Partial<Record<ShortcutId, { key: string; requiresMod?: boolean; shift?: boolean }>>;

function loadCustomBindings(): CustomBindingMap {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (!raw) return {};
    return JSON.parse(raw) as CustomBindingMap;
  } catch {
    return {};
  }
}

function saveCustomBindings(map: CustomBindingMap): void {
  if (Object.keys(map).length === 0) {
    localStorage.removeItem(STORAGE_KEY);
  } else {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(map));
  }
}

let customBindings: CustomBindingMap = loadCustomBindings();

function getEffectiveBindings(): ShortcutBinding[] {
  return DEFAULT_BINDINGS.map((def) => {
    const custom = customBindings[def.id];
    if (custom) {
      return { ...def, key: custom.key, requiresMod: custom.requiresMod, shift: custom.shift };
    }
    return def;
  });
}

// Re-exported for backwards compat — always returns effective (custom-merged) bindings
export function getShortcutBindings(): ShortcutBinding[] {
  return getEffectiveBindings();
}
// Legacy alias
export const SHORTCUT_BINDINGS = DEFAULT_BINDINGS;

export function updateBinding(id: ShortcutId, key: string, requiresMod?: boolean, shift?: boolean): void {
  const def = DEFAULT_BINDINGS.find((b) => b.id === id);
  if (def && def.key === key && !!def.requiresMod === !!requiresMod && !!def.shift === !!shift) {
    // Same as default — remove custom override
    delete customBindings[id];
  } else {
    customBindings[id] = { key, requiresMod, shift };
  }
  saveCustomBindings(customBindings);
}

export function resetBinding(id: ShortcutId): void {
  delete customBindings[id];
  saveCustomBindings(customBindings);
}

export function resetAllBindings(): void {
  customBindings = {};
  saveCustomBindings(customBindings);
}

export function isCustomized(id: ShortcutId): boolean {
  return id in customBindings;
}

export function isMacPlatform(): boolean {
  if (typeof navigator === 'undefined') return false;
  return /Mac|iPhone|iPad|iPod/.test(navigator.platform);
}

export function getShortcutDisplayKeys(id: ShortcutId): string[] {
  const bindings = getEffectiveBindings();
  const binding = bindings.find((b) => b.id === id);
  if (!binding) return [];

  const keys: string[] = [];
  if (binding.requiresMod) {
    keys.push(isMacPlatform() ? 'Cmd' : 'Ctrl');
  }
  if (binding.shift) {
    keys.push('Shift');
  }

  if (binding.key === 'Enter') keys.push('Enter');
  else if (binding.key === 'Escape') keys.push('Esc');
  else keys.push(binding.key.toUpperCase());

  return keys;
}

export function formatBindingKeys(binding: Pick<ShortcutBinding, 'key' | 'requiresMod' | 'shift'>): string[] {
  const keys: string[] = [];
  if (binding.requiresMod) keys.push(isMacPlatform() ? 'Cmd' : 'Ctrl');
  if (binding.shift) keys.push('Shift');
  if (binding.key === 'Enter') keys.push('Enter');
  else if (binding.key === 'Escape') keys.push('Esc');
  else keys.push(binding.key.toUpperCase());
  return keys;
}

export function matchesShortcut(
  event: Pick<KeyboardEvent, 'key' | 'metaKey' | 'ctrlKey' | 'shiftKey'>,
  id: ShortcutId
): boolean {
  const bindings = getEffectiveBindings();
  const binding = bindings.find((b) => b.id === id);
  if (!binding) return false;

  const modPressed = event.metaKey || event.ctrlKey;
  const requiresMod = !!binding.requiresMod;
  const requiresShift = !!binding.shift;

  if (requiresMod !== modPressed) return false;
  if (requiresShift !== event.shiftKey) return false;

  return event.key.toLowerCase() === binding.key.toLowerCase();
}
