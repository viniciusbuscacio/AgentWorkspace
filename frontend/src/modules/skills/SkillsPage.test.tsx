// @vitest-environment happy-dom
import { render, screen, waitFor, fireEvent, act } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { SkillsPage } from './SkillsPage';

// Notices now go through the app-wide toast (lib/notify -> sonner); module
// tests assert the notify text instead of inline DOM messages.
const notifyMock = vi.fn();
vi.mock('@/lib/notify', () => ({ notify: (...a: unknown[]) => notifyMock(...a) }));


const list = vi.fn();
const detail = vi.fn();
const create = vi.fn();
const save = vi.fn();
const setEnabled = vi.fn();
const del = vi.fn();
const reset = vi.fn();
const importSkill = vi.fn();
const importFolder = vi.fn();

vi.mock('@services/skills.service', async (importOriginal) => {
  const original = await importOriginal<typeof import('@services/skills.service')>();
  return {
    ...original,
    skillsService: {
      list: (...a: unknown[]) => list(...a),
      detail: (...a: unknown[]) => detail(...a),
      create: (...a: unknown[]) => create(...a),
      save: (...a: unknown[]) => save(...a),
      setEnabled: (...a: unknown[]) => setEnabled(...a),
      delete: (...a: unknown[]) => del(...a),
      reset: (...a: unknown[]) => reset(...a),
      importSkill: (...a: unknown[]) => importSkill(...a),
      importFolder: (...a: unknown[]) => importFolder(...a),
    },
  };
});

const skillRow = {
  id: 'avell-health',
  name: 'avell-health',
  description: 'Check Avell laptop health.',
  origin: 'builtin',
  enabled: true,
  customized: false,
  updateAvailable: false,
  deleted: false,
  fileCount: 1,
};

beforeEach(() => {
  vi.clearAllMocks();
  list.mockResolvedValue({ success: true, skills: [skillRow] });
  detail.mockResolvedValue({
    success: true,
    skill: { ...skillRow, files: [{ path: 'SKILL.md', content: '---\nname: avell-health\ndescription: x\n---\n\nbody' }] },
  });
});

async function openEditor(user: ReturnType<typeof userEvent.setup>) {
  await screen.findByRole('button', { name: 'Add Skill' });
  await user.click(screen.getByRole('button', { name: 'Edit' }));
  await screen.findByRole('textbox');
}

describe('SkillsPage', () => {
  it('shows the Add Skill / Import buttons in the list toolbar', async () => {
    render(<SkillsPage />);
    await screen.findByRole('button', { name: 'Add Skill' });
    expect(screen.getByRole('button', { name: 'Add Skill' })).toBeTruthy();
    expect(screen.getByRole('button', { name: 'Import Skill' })).toBeTruthy();
    expect(screen.getByRole('button', { name: 'Import Folder' })).toBeTruthy();
  });

  it('initialSkillId reopens that skill editor on mount (return-from-chat restore)', async () => {
    const onTitle = vi.fn();
    render(<SkillsPage initialSkillId="avell-health" onSkillDetailTitleChange={onTitle} />);
    // The editor opens without any user click: Cancel only exists in editor mode.
    await screen.findByRole('button', { name: 'Cancel' });
    const textarea = screen.getByRole('textbox') as HTMLTextAreaElement;
    expect(textarea.value).toContain('name: avell-health');
    expect(detail).toHaveBeenCalledWith('avell-health');
    await waitFor(() => expect(onTitle).toHaveBeenCalledWith('avell-health'));
  });

  it('ignores an initialSkillId that no longer exists and shows the list', async () => {
    render(<SkillsPage initialSkillId="ghost-skill" />);
    await screen.findByRole('button', { name: 'Add Skill' });
    // No editor opened: detail was never fetched and there is no Cancel button.
    expect(detail).not.toHaveBeenCalled();
    expect(screen.queryByRole('button', { name: 'Cancel' })).toBeNull();
  });

  it('opening edit reports the breadcrumb title and renders no Back button', async () => {
    const onTitle = vi.fn();
    const user = userEvent.setup();
    render(<SkillsPage onSkillDetailTitleChange={onTitle} />);
    await openEditor(user);

    expect(onTitle).toHaveBeenCalledWith('avell-health');
    expect(screen.queryByRole('button', { name: 'Back' })).toBeNull();
    expect(screen.queryByText('Editing avell-health')).toBeNull();
  });

  it('cancel in edit clears the breadcrumb title and returns to the list', async () => {
    const onTitle = vi.fn();
    const user = userEvent.setup();
    render(<SkillsPage onSkillDetailTitleChange={onTitle} />);
    await openEditor(user);

    await user.click(screen.getByRole('button', { name: 'Cancel' }));
    await waitFor(() => expect(onTitle).toHaveBeenLastCalledWith(null));
    expect(screen.getByRole('button', { name: 'Add Skill' })).toBeTruthy();
  });

  it('Add Skill opens the SKILL.md template editor with the Add Skill title', async () => {
    const onTitle = vi.fn();
    const user = userEvent.setup();
    render(<SkillsPage onSkillDetailTitleChange={onTitle} />);
    await screen.findByRole('button', { name: 'Add Skill' });

    await user.click(screen.getByRole('button', { name: 'Add Skill' }));
    expect(onTitle).toHaveBeenCalledWith('Add Skill');
    const textarea = (await screen.findByRole('textbox')) as HTMLTextAreaElement;
    expect(textarea.value).toContain('name: my-skill');
    expect(textarea.value).toContain('description:');
  });

  it('cancel in Add Skill returns to the list without creating', async () => {
    const user = userEvent.setup();
    render(<SkillsPage />);
    await screen.findByRole('button', { name: 'Add Skill' });
    await user.click(screen.getByRole('button', { name: 'Add Skill' }));
    await screen.findByRole('textbox');

    await user.click(screen.getByRole('button', { name: 'Cancel' }));
    await screen.findByRole('button', { name: 'Add Skill' });
    expect(create).not.toHaveBeenCalled();
  });

  it('saving Add Skill with the valid template id calls create and refreshes', async () => {
    create.mockResolvedValue({ success: true });
    const user = userEvent.setup();
    render(<SkillsPage />);
    await screen.findByRole('button', { name: 'Add Skill' });
    await user.click(screen.getByRole('button', { name: 'Add Skill' }));
    await screen.findByRole('textbox');

    await user.click(screen.getByRole('button', { name: 'Save' }));
    await waitFor(() => expect(create).toHaveBeenCalledTimes(1));
    expect(create).toHaveBeenCalledWith(expect.objectContaining({ id: 'my-skill', enabled: true }));
    expect(list).toHaveBeenCalledTimes(2); // initial + post-create refresh
  });

  it('invalid frontmatter name shows an error and does not call create', async () => {
    const user = userEvent.setup();
    render(<SkillsPage />);
    await screen.findByRole('button', { name: 'Add Skill' });
    await user.click(screen.getByRole('button', { name: 'Add Skill' }));
    const textarea = await screen.findByRole('textbox');

    fireEvent.change(textarea, { target: { value: '---\nname: Invalid Name\ndescription: d\n---\n\nbody' } });
    await user.click(screen.getByRole('button', { name: 'Save' }));

    expect(create).not.toHaveBeenCalled();
    await vi.waitFor(() => expect(notifyMock).toHaveBeenCalledWith(expect.stringContaining('valid skill id')));
    expect(screen.getByRole('textbox')).toBeTruthy(); // editor stays open
  });

  it('duplicate id keeps the editor open and shows the backend error', async () => {
    create.mockResolvedValue({ success: false, error: 'skill "my-skill" already exists; choose a new id' });
    const user = userEvent.setup();
    render(<SkillsPage />);
    await screen.findByRole('button', { name: 'Add Skill' });
    await user.click(screen.getByRole('button', { name: 'Add Skill' }));
    await screen.findByRole('textbox');

    await user.click(screen.getByRole('button', { name: 'Save' }));
    await waitFor(() => expect(create).toHaveBeenCalledTimes(1));
    await vi.waitFor(() => expect(notifyMock).toHaveBeenCalledWith(expect.stringContaining('already exists')));
    expect(screen.getByRole('textbox')).toBeTruthy();
  });

  it('clears the breadcrumb title on unmount', async () => {
    const onTitle = vi.fn();
    const user = userEvent.setup();
    const { unmount } = render(<SkillsPage onSkillDetailTitleChange={onTitle} />);
    await openEditor(user);
    onTitle.mockClear();
    unmount();
    expect(onTitle).toHaveBeenCalledWith(null);
  });

  it('returns to the list when the Skills breadcrumb fires (listRequest change)', async () => {
    const onTitle = vi.fn();
    const user = userEvent.setup();
    const { rerender } = render(<SkillsPage onSkillDetailTitleChange={onTitle} listRequest={0} />);
    await openEditor(user);

    rerender(<SkillsPage onSkillDetailTitleChange={onTitle} listRequest={1} />);
    await screen.findByRole('button', { name: 'Add Skill' });
    await waitFor(() => expect(onTitle).toHaveBeenLastCalledWith(null));
  });
});

describe('SkillsPage unsaved-changes guard', () => {
  it('dirty edit + Cancel opens the discard dialog; Keep editing preserves text', async () => {
    const user = userEvent.setup();
    render(<SkillsPage />);
    await openEditor(user);
    const ta = screen.getByRole('textbox');
    fireEvent.change(ta, { target: { value: 'changed body' } });

    await user.click(screen.getByRole('button', { name: 'Cancel' }));
    expect(screen.getByText('Discard unsaved skill changes?')).toBeTruthy();

    await user.click(screen.getByRole('button', { name: 'Keep editing' }));
    expect(screen.queryByText('Discard unsaved skill changes?')).toBeNull();
    expect((screen.getByRole('textbox') as HTMLTextAreaElement).value).toBe('changed body');
  });

  it('dirty edit + Cancel + Discard changes returns to the list', async () => {
    const user = userEvent.setup();
    render(<SkillsPage />);
    await openEditor(user);
    fireEvent.change(screen.getByRole('textbox'), { target: { value: 'changed body' } });

    await user.click(screen.getByRole('button', { name: 'Cancel' }));
    await user.click(screen.getByRole('button', { name: 'Discard changes' }));
    await screen.findByRole('button', { name: 'Add Skill' });
  });

  it('clean edit + Cancel returns immediately without a dialog', async () => {
    const user = userEvent.setup();
    render(<SkillsPage />);
    await openEditor(user);

    await user.click(screen.getByRole('button', { name: 'Cancel' }));
    expect(screen.queryByText('Discard unsaved skill changes?')).toBeNull();
    await screen.findByRole('button', { name: 'Add Skill' });
  });

  it('dirty create + Cancel opens the discard dialog', async () => {
    const user = userEvent.setup();
    render(<SkillsPage />);
    await screen.findByRole('button', { name: 'Add Skill' });
    await user.click(screen.getByRole('button', { name: 'Add Skill' }));
    fireEvent.change(await screen.findByRole('textbox'), { target: { value: 'edited template' } });

    await user.click(screen.getByRole('button', { name: 'Cancel' }));
    expect(screen.getByText('Discard unsaved skill changes?')).toBeTruthy();
  });

  it('dirty edit + Skills breadcrumb (listRequest) opens the discard dialog', async () => {
    const user = userEvent.setup();
    const { rerender } = render(<SkillsPage listRequest={0} />);
    await openEditor(user);
    fireEvent.change(screen.getByRole('textbox'), { target: { value: 'changed body' } });

    rerender(<SkillsPage listRequest={1} />);
    expect(await screen.findByText('Discard unsaved skill changes?')).toBeTruthy();
  });

  it('registers a guard with the parent that defers external navigation until confirmed', async () => {
    const user = userEvent.setup();
    let guard: ((proceed: () => void) => void) | null = null;
    render(<SkillsPage onRegisterLeaveGuard={(g) => { guard = g; }} />);
    await openEditor(user);
    fireEvent.change(screen.getByRole('textbox'), { target: { value: 'changed body' } });
    expect(typeof guard).toBe('function');

    const proceed = vi.fn();
    act(() => guard!(proceed));
    expect(proceed).not.toHaveBeenCalled(); // dirty → parked behind the dialog
    expect(await screen.findByText('Discard unsaved skill changes?')).toBeTruthy();

    await user.click(screen.getByRole('button', { name: 'Discard changes' }));
    expect(proceed).toHaveBeenCalledTimes(1);
  });

  it('clears the registered guard when the editor closes', async () => {
    const user = userEvent.setup();
    const onRegister = vi.fn();
    render(<SkillsPage onRegisterLeaveGuard={onRegister} />);
    await openEditor(user);
    expect(onRegister).toHaveBeenLastCalledWith(expect.any(Function));
    await user.click(screen.getByRole('button', { name: 'Cancel' })); // clean → closes
    await waitFor(() => expect(onRegister).toHaveBeenLastCalledWith(null));
  });
});

describe('SkillsPage search + origin filter', () => {
  const builtin = { ...skillRow, name: 'Avell Health' };
  const userSkill = {
    id: 'gmail-helper',
    name: 'Gmail Helper',
    description: 'Send gmail drafts.',
    origin: 'user',
    enabled: true,
    customized: false,
    updateAvailable: false,
    deleted: false,
    fileCount: 1,
  };

  beforeEach(() => {
    list.mockResolvedValue({ success: true, skills: [builtin, userSkill] });
  });

  it('renders the search input and the exact origin filter labels', async () => {
    const user = userEvent.setup();
    render(<SkillsPage />);
    expect(await screen.findByPlaceholderText('Search skills...')).toBeTruthy();
    await user.click(screen.getByRole('combobox', { name: 'Filter skills by origin' }));
    // Match by accessible name (textContent also carries the check glyph).
    expect(screen.getAllByRole('option')).toHaveLength(3);
    for (const label of ['All Skills', 'User Skills', 'Built in Skills']) {
      expect(screen.getByRole('option', { name: new RegExp(label) })).toBeTruthy();
    }
  });

  it('filters by search query (id/description) and clears', async () => {
    const user = userEvent.setup();
    render(<SkillsPage />);
    const input = await screen.findByPlaceholderText('Search skills...');

    await user.type(input, 'gmail');
    await waitFor(() => expect(screen.queryByText('Avell Health')).toBeNull());
    expect(screen.getByText('Gmail Helper')).toBeTruthy();

    await user.click(screen.getByRole('button', { name: 'Clear search' }));
    await screen.findByText('Avell Health');
  });

  it('filters by origin (User vs Built in)', async () => {
    const user = userEvent.setup();
    render(<SkillsPage />);
    await screen.findByPlaceholderText('Search skills...');
    const select = screen.getByRole('combobox', { name: 'Filter skills by origin' });

    await user.click(select);
    await user.click(screen.getByRole('option', { name: 'User Skills' }));
    expect(screen.queryByText('Avell Health')).toBeNull();
    expect(screen.getByText('Gmail Helper')).toBeTruthy();

    await user.click(select);
    await user.click(screen.getByRole('option', { name: 'Built in Skills' }));
    expect(screen.getByText('Avell Health')).toBeTruthy();
    expect(screen.queryByText('Gmail Helper')).toBeNull();
  });

  it('shows the filtered-empty state distinct from the truly-empty state', async () => {
    const user = userEvent.setup();
    render(<SkillsPage />);
    const input = await screen.findByPlaceholderText('Search skills...');
    await user.type(input, 'azure');
    expect(await screen.findByText('No skills match "azure".')).toBeTruthy();
  });
});
