import { beforeEach, describe, expect, it } from 'vitest';
import type { Message } from '../types';
import { useMessageStore } from './index';

const message = (id: string, conversationId: string, content: string): Message => ({
  id,
  conversation_id: conversationId,
  role: 'assistant',
  content,
  created_at: new Date(0).toISOString(),
});

describe('message conversation isolation', () => {
  beforeEach(() => {
    useMessageStore.getState().clearMessages();
  });

  it('replaces the rendered transcript with selected branch messages', () => {
    useMessageStore.getState().replaceMessages('conversation-a', [
      message('branch-message', 'conversation-a', 'Branch-specific answer'),
    ]);

    const state = useMessageStore.getState();
    expect(state.loadedConversationId).toBe('conversation-a');
    expect(state.messages.map((item) => item.content)).toEqual(['Branch-specific answer']);
    expect(state.loading).toBe(false);
  });

  it('can mark a newly-created empty conversation as ready without retaining old messages', () => {
    useMessageStore.getState().replaceMessages('old-conversation', [
      message('old-message', 'old-conversation', 'Do not leak this transcript'),
    ]);
    useMessageStore.getState().clearMessages('new-conversation');

    const state = useMessageStore.getState();
    expect(state.loadedConversationId).toBe('new-conversation');
    expect(state.messages).toEqual([]);
  });
});
