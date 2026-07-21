import {
  GetEffectiveInstructions,
  GetInstructionDocument,
  GetInstructionSources,
  ListInstructionDocuments,
  ResetInstructionDocument,
  SaveInstructionDocument,
} from '@wails/go/main/App';
import type { domain, dto } from '@wails/go/models';

export type InstructionDocument = domain.InstructionDocument;
export type InstructionDocumentContent = domain.InstructionDocumentContent;
export type InstructionSource = domain.InstructionSource;
export type EffectiveInstructions = domain.EffectiveInstructions;

export type InstructionsListResult = dto.InstructionsListResult;
export type InstructionReadResult = dto.InstructionReadResult;
export type InstructionSaveResult = dto.InstructionSaveResult;
export type EffectiveInstructionsResult = dto.EffectiveInstructionsResult;
export type InstructionSourcesResult = dto.InstructionSourcesResult;

// instructionsService is the only frontend import point for the generated
// Agent Instructions Wails bindings, mirroring skills.service.ts.
export const instructionsService = {
  list(): Promise<InstructionsListResult> {
    return ListInstructionDocuments();
  },
  read(id: string): Promise<InstructionReadResult> {
    return GetInstructionDocument(id);
  },
  save(id: string, content: string): Promise<InstructionSaveResult> {
    return SaveInstructionDocument(id, content);
  },
  reset(id: string): Promise<InstructionSaveResult> {
    return ResetInstructionDocument(id);
  },
  effective(includeContent = true): Promise<EffectiveInstructionsResult> {
    return GetEffectiveInstructions(includeContent);
  },
  sources(): Promise<InstructionSourcesResult> {
    return GetInstructionSources();
  },
};
