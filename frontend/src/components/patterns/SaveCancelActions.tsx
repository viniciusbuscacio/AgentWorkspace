import { Button } from '@ui/button';
import type { ButtonSize } from '@/components/ui/button';

export const SAVE_ICON = 'save';
export const CANCEL_ICON = 'cancel';
export const SAVE_CANCEL_BUTTON_CLASS = 'w-24';

export interface SaveCancelActionsProps {
  /** Called when the user confirms the save. Required — a Save without a
   *  Cancel must not be rendered; use the component's absence as the signal. */
  onSave: () => void;
  /** Called when the user discards the in-progress draft. Required (same
   *  rule as onSave: the component enforces the pair at the type level). */
  onCancel: () => void;
  /** Disable both buttons (e.g. nothing to save yet). */
  disabled?: boolean;
  /** Show the spinner/disabled state while an async save is in flight. */
  saving?: boolean;
  size?: ButtonSize;
  /** Optional status node rendered after the buttons (e.g. an error message). */
  status?: React.ReactNode;
  className?: string;
}

/**
 * SaveCancelActions — the ONE canonical way to render a Save button in aw.
 *
 * Ported from AW2 `src/renderer/components/patterns/SaveCancelActions.tsx`.
 * Both `onSave` and `onCancel` are required props; the TypeScript compiler
 * enforces the pair. Visual style matches the existing Notes Save button.
 *
 * The ESLint fence in `frontend/eslint.config.js` (`no-restricted-syntax`)
 * flags any `<Button>` with literal text "Save" or "Saving" outside this
 * component, providing a second layer of enforcement.
 */
export function SaveCancelActions({
  onSave,
  onCancel,
  disabled,
  saving,
  size = 'default',
  status,
  className = '',
}: SaveCancelActionsProps) {
  return (
    <div
      data-aw-save-cancel-actions
      className={['flex flex-wrap items-center gap-2', className].filter(Boolean).join(' ')}
    >
      {/* eslint-disable-next-line no-restricted-syntax -- Authorized canonical Save button. */}
      <Button type="button" size={size} icon={SAVE_ICON} className={SAVE_CANCEL_BUTTON_CLASS} onClick={onSave} disabled={disabled || saving} aria-busy={saving || undefined} aria-label="Save">
        Save
      </Button>
      <Button type="button" variant="outline" size={size} icon={CANCEL_ICON} className={SAVE_CANCEL_BUTTON_CLASS} onClick={onCancel} disabled={saving}>
        Cancel
      </Button>
      {status}
    </div>
  );
}

/** Toolbar-action adapter (for modules that use a toolbar instead of inline buttons). */
export interface ToolbarAction {
  id: string;
  icon?: string;
  label: string;
  variant?: 'default' | 'outline' | 'ghost' | 'destructive';
  onClick: () => void;
  disabled?: boolean;
}

export function saveCancelToolbarActions({
  onSave,
  onCancel,
  disabled,
  saving,
}: Omit<SaveCancelActionsProps, 'size' | 'status' | 'className'>): ToolbarAction[] {
  return [
    {
      id: 'save',
      icon: SAVE_ICON,
      label: 'Save',
      variant: 'default',
      onClick: onSave,
      disabled: disabled || saving,
    },
    { id: 'cancel', icon: CANCEL_ICON, label: 'Cancel', variant: 'outline', onClick: onCancel },
  ];
}
