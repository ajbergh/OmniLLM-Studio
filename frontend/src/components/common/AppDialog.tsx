import { useEffect, useRef, useState } from 'react';
import type { ReactNode } from 'react';
import { createPortal } from 'react-dom';

/**
 * App-native replacements for window.confirm / window.prompt.
 * Portal-rendered modal with Escape-to-cancel, Enter-to-confirm, focus on
 * open, and the studio's dark surface styling.
 */
function DialogShell({ children, onCancel }: { children: ReactNode; onCancel: () => void }) {
  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        event.stopPropagation();
        onCancel();
      }
    };
    window.addEventListener('keydown', onKeyDown, true);
    return () => window.removeEventListener('keydown', onKeyDown, true);
  }, [onCancel]);

  return createPortal(
    <div className="fixed inset-0 z-[110] flex items-center justify-center bg-black/50" onClick={onCancel}>
      <div
        role="dialog"
        aria-modal="true"
        className="w-80 rounded-lg border border-border bg-surface p-4 shadow-xl"
        onClick={(event) => event.stopPropagation()}
      >
        {children}
      </div>
    </div>,
    document.body,
  );
}

export function ConfirmDialog({
  title,
  message,
  confirmLabel = 'Confirm',
  cancelLabel = 'Cancel',
  danger = false,
  onConfirm,
  onCancel,
}: {
  title: string;
  message?: string;
  confirmLabel?: string;
  cancelLabel?: string;
  danger?: boolean;
  onConfirm: () => void;
  onCancel: () => void;
}) {
  const confirmRef = useRef<HTMLButtonElement | null>(null);
  useEffect(() => {
    confirmRef.current?.focus();
  }, []);
  return (
    <DialogShell onCancel={onCancel}>
      <h3 className="text-sm font-semibold text-text">{title}</h3>
      {message && <p className="mt-2 text-[12px] leading-relaxed text-text-muted">{message}</p>}
      <div className="mt-4 flex justify-end gap-2">
        <button
          type="button"
          className="rounded-md border border-border bg-surface-alt px-3 py-1.5 text-[12px] text-text-secondary hover:text-text"
          onClick={onCancel}
        >
          {cancelLabel}
        </button>
        <button
          ref={confirmRef}
          type="button"
          className={`rounded-md px-3 py-1.5 text-[12px] font-medium ${
            danger ? 'bg-red-500/90 text-white hover:bg-red-500' : 'bg-primary text-white hover:bg-primary/90'
          }`}
          onClick={onConfirm}
        >
          {confirmLabel}
        </button>
      </div>
    </DialogShell>
  );
}

export function InputDialog({
  title,
  label,
  initialValue = '',
  placeholder,
  inputType = 'text',
  submitLabel = 'Save',
  validate,
  onSubmit,
  onCancel,
}: {
  title: string;
  label?: string;
  initialValue?: string;
  placeholder?: string;
  inputType?: 'text' | 'number';
  submitLabel?: string;
  /** Returns an error message to block submission, or null/undefined to allow it. */
  validate?: (value: string) => string | null | undefined;
  onSubmit: (value: string) => void;
  onCancel: () => void;
}) {
  const [value, setValue] = useState(initialValue);
  const [error, setError] = useState<string | null>(null);
  const inputRef = useRef<HTMLInputElement | null>(null);
  useEffect(() => {
    inputRef.current?.focus();
    inputRef.current?.select();
  }, []);

  const submit = () => {
    const problem = validate?.(value);
    if (problem) {
      setError(problem);
      return;
    }
    onSubmit(value);
  };

  return (
    <DialogShell onCancel={onCancel}>
      <h3 className="text-sm font-semibold text-text">{title}</h3>
      {label && <p className="mt-2 text-[12px] text-text-muted">{label}</p>}
      <input
        ref={inputRef}
        type={inputType}
        className="mt-2 w-full rounded-md border border-border bg-surface-alt px-2 py-1.5 text-[12px] text-text outline-none focus:border-primary/60"
        value={value}
        placeholder={placeholder}
        onChange={(event) => {
          setValue(event.target.value);
          setError(null);
        }}
        onKeyDown={(event) => {
          if (event.key === 'Enter') {
            event.preventDefault();
            submit();
          }
          event.stopPropagation();
        }}
      />
      {error && <p className="mt-1.5 text-[11px] text-red-400">{error}</p>}
      <div className="mt-4 flex justify-end gap-2">
        <button
          type="button"
          className="rounded-md border border-border bg-surface-alt px-3 py-1.5 text-[12px] text-text-secondary hover:text-text"
          onClick={onCancel}
        >
          Cancel
        </button>
        <button
          type="button"
          className="rounded-md bg-primary px-3 py-1.5 text-[12px] font-medium text-white hover:bg-primary/90"
          onClick={submit}
        >
          {submitLabel}
        </button>
      </div>
    </DialogShell>
  );
}
