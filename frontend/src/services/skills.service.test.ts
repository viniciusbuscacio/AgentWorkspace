import { describe, expect, it, vi } from 'vitest';
import { skillsService } from './skills.service';

const ListSkills = vi.fn();
const GetSkillDetail = vi.fn();
const SetSkillEnabled = vi.fn();
const SaveSkill = vi.fn();
const DeleteSkill = vi.fn();
const ResetSkillToSeed = vi.fn();
const ImportSkillFromFile = vi.fn();
const ImportSkillsFromParent = vi.fn();
const SelectSkillFile = vi.fn();
const SelectImportFolder = vi.fn();
const CreateSkill = vi.fn();

vi.mock('@wails/go/main/App', () => ({
  ListSkills: (...args: unknown[]) => ListSkills(...args),
  GetSkillDetail: (...args: unknown[]) => GetSkillDetail(...args),
  SetSkillEnabled: (...args: unknown[]) => SetSkillEnabled(...args),
  SaveSkill: (...args: unknown[]) => SaveSkill(...args),
  DeleteSkill: (...args: unknown[]) => DeleteSkill(...args),
  ResetSkillToSeed: (...args: unknown[]) => ResetSkillToSeed(...args),
  ImportSkillFromFile: (...args: unknown[]) => ImportSkillFromFile(...args),
  ImportSkillsFromParent: (...args: unknown[]) => ImportSkillsFromParent(...args),
  SelectSkillFile: (...args: unknown[]) => SelectSkillFile(...args),
  SelectImportFolder: (...args: unknown[]) => SelectImportFolder(...args),
  CreateSkill: (...args: unknown[]) => CreateSkill(...args),
}));

describe('skillsService', () => {
  it('list delegates to ListSkills', async () => {
    ListSkills.mockResolvedValue({ success: true, skills: [{ id: 'gmail-web', enabled: true }] });
    const result = await skillsService.list();
    expect(result.success).toBe(true);
    expect(result.skills?.[0].id).toBe('gmail-web');
  });

  it('setEnabled passes id and flag', async () => {
    SetSkillEnabled.mockResolvedValue({ success: true });
    await skillsService.setEnabled('gmail-web', false);
    expect(SetSkillEnabled).toHaveBeenCalledWith('gmail-web', false);
  });

  it('save builds full file payloads', async () => {
    SaveSkill.mockResolvedValue({ success: true });
    await skillsService.save('my-skill', [{ path: 'SKILL.md', content: 'body' }]);
    expect(SaveSkill).toHaveBeenCalledWith('my-skill', [
      { path: 'SKILL.md', content: 'body', contentHash: '', seedHash: '' },
    ]);
  });

  it('create passes id/name/description/enabled and full files', async () => {
    CreateSkill.mockResolvedValue({ success: true });
    await skillsService.create({ id: 'my-skill', enabled: true, files: [{ path: 'SKILL.md', content: 'body' }] });
    expect(CreateSkill).toHaveBeenCalledWith('my-skill', '', '', true, [
      { path: 'SKILL.md', content: 'body', contentHash: '', seedHash: '' },
    ]);
  });

  it('importSkill returns null when the picker is canceled', async () => {
    SelectSkillFile.mockResolvedValue({ canceled: true });
    const result = await skillsService.importSkill();
    expect(result).toBeNull();
    expect(ImportSkillFromFile).not.toHaveBeenCalled();
  });

  it('importSkill imports the chosen SKILL.md', async () => {
    SelectSkillFile.mockResolvedValue({ canceled: false, path: '/tmp/my-skill/SKILL.md' });
    ImportSkillFromFile.mockResolvedValue({ success: true });
    const result = await skillsService.importSkill();
    expect(ImportSkillFromFile).toHaveBeenCalledWith('/tmp/my-skill/SKILL.md');
    expect(result?.success).toBe(true);
  });

  it('importFolder returns null when the picker is canceled', async () => {
    SelectImportFolder.mockResolvedValue({ canceled: true });
    const result = await skillsService.importFolder();
    expect(result).toBeNull();
    expect(ImportSkillsFromParent).not.toHaveBeenCalled();
  });

  it('importFolder imports the chosen parent and returns the summary', async () => {
    SelectImportFolder.mockResolvedValue({ canceled: false, path: '/tmp/skills' });
    ImportSkillsFromParent.mockResolvedValue({
      success: true,
      summary: { imported: ['alpha'], skipped: [{ id: 'beta', reason: 'already exists' }] },
    });
    const result = await skillsService.importFolder();
    expect(ImportSkillsFromParent).toHaveBeenCalledWith('/tmp/skills');
    expect(result?.summary.imported).toEqual(['alpha']);
    expect(result?.summary.skipped[0].id).toBe('beta');
  });
});
