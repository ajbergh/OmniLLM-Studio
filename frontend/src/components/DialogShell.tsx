import { useEffect, useId, useRef, type KeyboardEvent, type ReactNode, type RefObject } from 'react';
import { AnimatePresence, motion } from 'framer-motion';
import { X } from 'lucide-react';
import { clsx } from 'clsx';

const focusableSelector = [
  'a[href]',
  'button:not([disabled])',
  'input:not([disabled])',
  'select:not([disabled])',
  'textarea:not([disabled])',
  '[tabindex]:not([tabindex="-1"])',
].join(',');

type DialogPlacement = 'center' | 'top' | 'bottom';

interface DialogShellProps {
  open: boolean;
  onClose: () => void;
  title: string;
  icon?: ReactNode;
  actions?: ReactNode;
  footer?: ReactNode;
  children: ReactNode;
  maxWidth?: string;
  maxHeight?: string;
  placement?: DialogPlacement;
  className?: string;
  bodyClassName?: string;
  initialFocusRef?: RefObject<HTMLElement | null>;
}

const placementClasses: Record<DialogPlacement, string> = {
  center: 'items-center justify-center p-3 sm:p-4',
  top: 'items-start justify-center p-3 pt-[10vh]',
  bottom: 'items-end justify-center p-3 sm:p-4',
};

export function DialogShell({
  open,
  onClose,
  title,
  icon,
  actions,
  footer,
  children,
  maxWidth = 'max-w-2xl',
  maxHeight = 'max-h-[80vh]',
  placement = 'center',
  className,
  bodyClassName,
  initialFocusRef,
}: DialogShellProps) {
  const titleId = useId();
  const dialogRef = useRef<HTMLDivElement>(null);
  const previousFocusRef = useRef<HTMLElement | null>(null);

  useEffect(() => {
    if (!open) return;

    previousFocusRef.current = document.activeElement instanceof HTMLElement
      ? document.activeElement
      : null;
    const previousOverflow = document.body.style.overflow;
    document.body.style.overflow = 'hidden';

    const focusTimer = window.setTimeout(() => {
      const dialog = dialogRef.current;
      if (!dialog) return;
      const firstFocusable = dialog.querySelector<HTMLElement>(focusableSelector);
      const target = initialFocusRef?.current ?? firstFocusable ?? dialog;
      target.focus();
    }, 0);

    return () => {
      window.clearTimeout(focusTimer);
      document.body.style.overflow = previousOverflow;
      previousFocusRef.current?.focus?.();
    };
  }, [initialFocusRef, open]);

  const handleKeyDown = (event: KeyboardEvent<HTMLDivElement>) => {
    if (event.key === 'Escape') {
      event.preventDefault();
      onClose();
      return;
    }

    if (event.key !== 'Tab') return;

    const dialog = dialogRef.current;
    if (!dialog) return;

    const focusable = Array.from(dialog.querySelectorAll<HTMLElement>(focusableSelector))
      .filter((element) => !element.hasAttribute('disabled') && element.offsetParent !== null);
    if (focusable.length === 0) {
      event.preventDefault();
      dialog.focus();
      return;
    }

    const first = focusable[0];
    const last = focusable[focusable.length - 1];

    if (event.shiftKey && document.activeElement === first) {
      event.preventDefault();
      last.focus();
    } else if (!event.shiftKey && document.activeElement === last) {
      event.preventDefault();
      first.focus();
    }
  };

  return (
    <AnimatePresence>
      {open && (
        <motion.div
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          exit={{ opacity: 0 }}
          className={clsx('fixed inset-0 bg-black/50 backdrop-blur-sm z-50 flex', placementClasses[placement])}
          onClick={onClose}
        >
          <motion.div
            ref={dialogRef}
            role="dialog"
            aria-modal="true"
            aria-labelledby={titleId}
            tabIndex={-1}
            initial={{
              opacity: 0,
              scale: placement === 'bottom' ? 1 : 0.95,
              y: placement === 'bottom' ? 24 : placement === 'top' ? -20 : 0,
            }}
            animate={{ opacity: 1, scale: 1, y: 0 }}
            exit={{
              opacity: 0,
              scale: placement === 'bottom' ? 1 : 0.95,
              y: placement === 'bottom' ? 24 : placement === 'top' ? -20 : 0,
            }}
            transition={{ duration: 0.2, ease: 'easeOut' }}
            onClick={(event) => event.stopPropagation()}
            onKeyDown={handleKeyDown}
            className={clsx(
              'glass-strong rounded-2xl w-full overflow-hidden flex flex-col shadow-2xl border border-border',
              maxWidth,
              maxHeight,
              className
            )}
          >
            <div className="flex items-center justify-between gap-3 px-4 py-3 sm:px-6 sm:py-4 border-b border-border shrink-0">
              <div className="flex min-w-0 items-center gap-2">
                {icon && <span className="shrink-0 text-primary">{icon}</span>}
                <h2 id={titleId} className="text-base sm:text-lg font-semibold text-text truncate">
                  {title}
                </h2>
              </div>
              <div className="flex shrink-0 items-center gap-2">
                {actions}
                <motion.button
                  whileHover={{ scale: 1.05 }}
                  whileTap={{ scale: 0.95 }}
                  onClick={onClose}
                  className="min-h-10 min-w-10 inline-flex items-center justify-center rounded-xl text-text-muted hover:text-text hover:bg-surface-hover transition-colors"
                  aria-label={`Close ${title}`}
                  title={`Close ${title}`}
                >
                  <X size={18} />
                </motion.button>
              </div>
            </div>

            <div className={clsx('flex-1 overflow-y-auto', bodyClassName ?? 'px-4 py-4 sm:px-6')}>
              {children}
            </div>

            {footer && (
              <div className="border-t border-border px-4 py-3 sm:px-6 shrink-0">
                {footer}
              </div>
            )}
          </motion.div>
        </motion.div>
      )}
    </AnimatePresence>
  );
}
