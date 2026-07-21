import { CreateNote, DeleteNote, ListNotes, UpdateNote, UpdateNoteFlags, UpdateNoteInPrompt } from '@wails/go/main/App';
import type { domain, dto } from '@wails/go/models';

export type Note = domain.Note;
export type NotesResult = dto.NotesResult;
export type NoteResult = dto.NoteResult;

export const notesService = {
  list(): Promise<NotesResult> {
    return ListNotes();
  },
  create(title: string, content: string, inPrompt: boolean): Promise<NoteResult> {
    return CreateNote(title, content, inPrompt);
  },
  update(id: string, title: string, content: string): Promise<NoteResult> {
    return UpdateNote(id, title, content);
  },
  updateFlags(id: string, pinned: boolean, archived: boolean): Promise<NoteResult> {
    return UpdateNoteFlags(id, pinned, archived);
  },
  updateInPrompt(id: string, inPrompt: boolean): Promise<NoteResult> {
    return UpdateNoteInPrompt(id, inPrompt);
  },
  delete(id: string): Promise<NoteResult> {
    return DeleteNote(id);
  },
};
