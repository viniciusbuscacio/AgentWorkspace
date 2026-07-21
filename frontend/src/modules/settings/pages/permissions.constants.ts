// permissions.constants.ts — all UI copy for the Permissions page.
// No strings live inline in the component. Refine wording here freely.

export const PERM_TITLE = 'Access mode';

// ── Modes: a radio list of three, with the selected mode's panel below ──────
// Internal mode ids/names DO NOT change — presentation only. deny_list was
// removed from the product (a vault still storing it fails safe to
// permit_list on load).

export interface ModeInfo {
  value: string;
  /** Plain-language label shown in the radio list. */
  label: string;
  /** Technical mode name (the backend value, shown small in the panel). */
  subtitle: string;
  /** What the selected mode means, shown in the panel below the radios. */
  description: string;
  /** Whether the panel carries the editable folder list. */
  editable: boolean;
}

export const MODES: ModeInfo[] = [
  {
    value: 'block_all',
    label: 'Block all',
    subtitle: 'block_all',
    description:
      'The most restrictive mode. The agent cannot read or edit your files and cannot run shell commands — it only keeps temporary files inside its own workspace.',
    editable: false,
  },
  {
    value: 'permit_list',
    label: 'Balanced',
    subtitle: 'permit_list',
    description: 'The agent can access the folders below, plus its own workspace:',
    editable: true,
  },
  {
    value: 'permit_all',
    label: 'Permit all',
    subtitle: 'permit_all',
    description:
      'The dangerous mode. The agent can do anything your user account can do and reach all your files. Use with caution. Built-in protected paths (keys, credentials) still stay blocked.',
    editable: false,
  },
];

// ── Balanced folder list ─────────────────────────────────────────────────────

export const BUILTIN_BADGE_LABEL = 'built-in';
export const BUILTIN_HINT = 'Always part of this mode — cannot be removed.';
export const FOLDER_PLACEHOLDER = '~/Projects/my-app';

// ── Save / footer ─────────────────────────────────────────────────────────────

export const SAVE_CANCELED_MSG = 'Change was not approved in the confirmation dialog.';
export const SAVE_FOOTER =
  'Saving asks for confirmation in a native dialog. The agent can inspect permissions (sandbox.status) but never change them.';

// ── Self-dev banner ───────────────────────────────────────────────────────────

export function SELF_DEV_BANNER(mode: string): string {
  return `Self-dev mode is on: the agent currently runs as permit_all rooted at the repository. The mode below (${mode}) applies when self-dev is turned off.`;
}
