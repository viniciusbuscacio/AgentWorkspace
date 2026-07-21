import {
  AddTasksAttachment,
  AddTasksItem,
  DeleteTasksAttachment,
  DeleteTasksItem,
  GetTasksAttachmentData,
  ListTasksItems,
  UpdateTasksItem,
} from '@wails/go/main/App';
import type { domain, dto } from '@wails/go/models';

export type TasksItem = domain.TasksItem;
export type TasksAttachment = domain.TasksAttachment;
export type TasksResult = dto.TasksResult;
export type TasksItemResult = dto.TasksItemResult;
export type TasksAttachmentResult = dto.TasksAttachmentResult;
export type TasksAttachmentDataResult = dto.TasksAttachmentDataResult;

// position < 0 keeps the item's current position (mirrors the Go signature).
export const KEEP_POSITION = -1;

// Valid statuses (AW2 parity). The cycle order matches the Go catalog.
export const BACKLOG_STATUSES = ['open', 'in-progress', 'needs-validation', 'completed'] as const;
export type TasksStatus = (typeof BACKLOG_STATUSES)[number];

export const tasksService = {
  list(): Promise<TasksResult> {
    return ListTasksItems();
  },
  add(title: string, body = '', status: string = 'open'): Promise<TasksItemResult> {
    return AddTasksItem(title, body, status);
  },
  // body is authoritative (an empty string clears it); pass the item's current
  // body when you only mean to change the status or title.
  update(
    id: string,
    title: string,
    body: string,
    status: string,
    position: number = KEEP_POSITION,
  ): Promise<TasksItemResult> {
    return UpdateTasksItem(id, title, body, status, position);
  },
  delete(id: string): Promise<TasksItemResult> {
    return DeleteTasksItem(id);
  },
  addAttachment(itemId: string, name: string, mimeType: string, dataBase64: string): Promise<TasksAttachmentResult> {
    return AddTasksAttachment(itemId, name, mimeType, dataBase64);
  },
  getAttachmentData(attachmentId: string): Promise<TasksAttachmentDataResult> {
    return GetTasksAttachmentData(attachmentId);
  },
  deleteAttachment(attachmentId: string): Promise<TasksAttachmentResult> {
    return DeleteTasksAttachment(attachmentId);
  },
};
