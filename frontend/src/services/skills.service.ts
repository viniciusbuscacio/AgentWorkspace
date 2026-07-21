import {
  CreateSkill,
  DeleteSkill,
  GetSkillDetail,
  ImportSkillFromFile,
  ImportSkillsFromParent,
  ListSkills,
  ResetSkillToSeed,
  SaveSkill,
  SelectImportFolder,
  SelectSkillFile,
  SetSkillEnabled,
} from '@wails/go/main/App';
import type { domain, dto } from '@wails/go/models';

export type SkillView = dto.SkillView;
export type SkillsResult = dto.SkillsResult;
export type SkillDetailResult = dto.SkillDetailResult;
export type SkillResult = dto.SkillResult;
export type ImportSkillsResult = dto.ImportSkillsResult;
export type Skill = domain.Skill;
export type SkillFile = domain.SkillFile;

export interface SkillFileDraft {
  path: string;
  content: string;
}

// toSkillFiles shapes editor drafts into the full SkillFile the binding expects;
// contentHash/seedHash are recomputed server-side.
function toSkillFiles(files: SkillFileDraft[]): SkillFile[] {
  return files.map((f) => ({ path: f.path, content: f.content, contentHash: '', seedHash: '' })) as SkillFile[];
}

export const skillsService = {
  list(): Promise<SkillsResult> {
    return ListSkills();
  },
  detail(id: string): Promise<SkillDetailResult> {
    return GetSkillDetail(id);
  },
  setEnabled(id: string, enabled: boolean): Promise<SkillResult> {
    return SetSkillEnabled(id, enabled);
  },
  save(id: string, files: SkillFileDraft[]): Promise<SkillResult> {
    return SaveSkill(id, toSkillFiles(files));
  },
  // create authors a new user skill from supplied files (Add Skill). id is the
  // explicit skill id (the frontmatter name in v1).
  create(input: {
    id: string;
    name?: string;
    description?: string;
    enabled?: boolean;
    files: SkillFileDraft[];
  }): Promise<SkillResult> {
    return CreateSkill(
      input.id,
      input.name ?? '',
      input.description ?? '',
      input.enabled ?? true,
      toSkillFiles(input.files),
    );
  },
  delete(id: string): Promise<SkillResult> {
    return DeleteSkill(id);
  },
  reset(id: string): Promise<SkillResult> {
    return ResetSkillToSeed(id);
  },
  // importSkill opens a file picker for a SKILL.md and imports that one skill.
  // Returns null when the user cancels the picker.
  async importSkill(): Promise<SkillResult | null> {
    const picked = await SelectSkillFile();
    if (picked.canceled || !picked.path) return null;
    return ImportSkillFromFile(picked.path);
  },
  // importFolder opens a directory picker for a parent folder and imports every
  // direct subfolder that has a SKILL.md, returning a batch summary. Returns
  // null when the user cancels the picker.
  async importFolder(): Promise<ImportSkillsResult | null> {
    const picked = await SelectImportFolder();
    if (picked.canceled || !picked.path) return null;
    return ImportSkillsFromParent(picked.path);
  },
};
