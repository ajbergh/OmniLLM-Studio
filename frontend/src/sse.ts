export interface SSEMessage {
  event: string;
  data: string;
  id?: string;
}

export interface SSEFinishResult {
  events: SSEMessage[];
  incomplete: boolean;
}

/** Incremental SSE decoder that preserves partial lines and event blocks. */
export class SSEDecoder {
  private buffer = '';
  private pendingCR = false;

  private readonly defaultEvent: string;

  constructor(defaultEvent = 'message') {
    this.defaultEvent = defaultEvent;
  }

  push(chunk: string): SSEMessage[] {
    let next = chunk;
    if (this.pendingCR) {
      next = `\r${next}`;
      this.pendingCR = false;
    }
    if (next.endsWith('\r')) {
      this.pendingCR = true;
      next = next.slice(0, -1);
    }
    if (next) this.buffer += next.replace(/\r\n/g, '\n').replace(/\r/g, '\n');
    const events: SSEMessage[] = [];
    let separator = this.buffer.indexOf('\n\n');
    while (separator >= 0) {
      const block = this.buffer.slice(0, separator);
      this.buffer = this.buffer.slice(separator + 2);
      const parsed = this.parseBlock(block);
      if (parsed) events.push(parsed);
      separator = this.buffer.indexOf('\n\n');
    }
    return events;
  }

  finish(): SSEFinishResult {
    const events: SSEMessage[] = [];
    if (this.pendingCR) {
      this.buffer += '\n';
      this.pendingCR = false;
    }
    const remaining = this.buffer;
    this.buffer = '';
    if (!remaining.trim()) return { events, incomplete: false };
    // A terminal blank line is required by the SSE framing contract. Do not
    // dispatch a partial event because its JSON may be split across network reads.
    return { events, incomplete: true };
  }

  private parseBlock(block: string): SSEMessage | null {
    let event = this.defaultEvent;
    let id: string | undefined;
    const data: string[] = [];
    for (const line of block.split('\n')) {
      if (!line || line.startsWith(':')) continue;
      const colon = line.indexOf(':');
      const field = colon >= 0 ? line.slice(0, colon) : line;
      let value = colon >= 0 ? line.slice(colon + 1) : '';
      if (value.startsWith(' ')) value = value.slice(1);
      if (field === 'event') event = value || this.defaultEvent;
      else if (field === 'data') data.push(value);
      else if (field === 'id') id = value;
    }
    if (data.length === 0) return null;
    return { event, data: data.join('\n'), ...(id !== undefined ? { id } : {}) };
  }
}
