export interface ContractTimelineEffect {
  id: string;
  type: string;
  amount?: number;
  enabled?: boolean;
  params?: Record<string, any>;
}

export function buildCssFilterString(effects: ContractTimelineEffect[]): string {
  const filters: string[] = [];

  for (const effect of effects) {
    if (effect.enabled === false) continue;
    const amount = effect.amount ?? 1;

    switch (effect.type) {
      case 'blur':
        filters.push(`blur(${amount * 10}px)`);
        break;
      case 'brightness':
        filters.push(`brightness(${amount})`);
        break;
      case 'contrast':
        filters.push(`contrast(${amount})`);
        break;
      case 'grayscale':
        filters.push(`grayscale(${amount})`);
        break;
      case 'sepia':
        filters.push(`sepia(${amount})`);
        break;
      case 'hue_rotate':
        filters.push(`hue-rotate(${amount * 360}deg)`);
        break;
      case 'invert':
        filters.push(`invert(${amount})`);
        break;
      case 'saturate':
        filters.push(`saturate(${amount})`);
        break;
      case 'drop_shadow':
        filters.push(`drop-shadow(0 4px 12px rgba(0,0,0,${amount * 0.75}))`);
        break;
      default:
        break;
    }
  }

  return filters.join(' ');
}
