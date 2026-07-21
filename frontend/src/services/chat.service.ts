import {
  CancelVoiceCapture,
  ClearChat,
  CompactChat,
  CreateChat,
  CreateChatWithTitle,
  DeleteChat,
  GetChatSessionInfo,
  GetPlanMode,
  ListChats,
  ListMessages,
  NewChatSession,
  OpenPipWindow,
  RecentMessages,
  RenameChat,
  ResolveToolConfirmation,
  SetChatArchived,
  SetChatModel,
  SetPlanMode,
  StartVoiceCapture,
  StopChat,
  StopVoiceCapture,
  StreamChatMessage,
  TranscribeAudio,
} from '@wails/go/main/App';
import type { domain, dto } from '@wails/go/models';

export type Chat = domain.Chat;
export type Message = domain.Message;
export type Attachment = domain.Attachment;
export type ChatSendResult = dto.ChatSendResult;
export type ChatOperationResult = dto.ChatOperationResult;
export type ChatSessionInfo = dto.ChatSessionInfo;
export type PlanModeResult = dto.PlanModeResult;
export type ChatModelResult = dto.ChatModelResult;
export type OperationResult = dto.OperationResult;

export const chatService = {
  listChats(): Promise<Chat[]> {
    return ListChats();
  },
  createChat(): Promise<Chat> {
    return CreateChat();
  },
  createChatWithTitle(title: string): Promise<Chat> {
    return CreateChatWithTitle(title);
  },
  listMessages(chatId: string): Promise<Message[]> {
    return ListMessages(chatId);
  },
  recentMessages(chatId: string, limit: number): Promise<Message[]> {
    return RecentMessages(chatId, limit);
  },
  streamChatMessage(chatId: string, text: string, attachments: Attachment[]): Promise<ChatSendResult> {
    return StreamChatMessage(chatId, text, attachments);
  },
  transcribeAudio(fileName: string, mimeType: string, dataUri: string): Promise<dto.AudioTranscriptionResult> {
    return TranscribeAudio(fileName, mimeType, dataUri);
  },
  startVoiceCapture(): Promise<dto.VoiceCaptureResult> {
    return StartVoiceCapture();
  },
  stopVoiceCapture(): Promise<dto.AudioTranscriptionResult> {
    return StopVoiceCapture();
  },
  cancelVoiceCapture(): Promise<dto.VoiceCaptureResult> {
    return CancelVoiceCapture();
  },
  stopChat(chatId: string): Promise<ChatOperationResult> {
    return StopChat(chatId);
  },
  renameChat(chatId: string, title: string): Promise<ChatOperationResult> {
    return RenameChat(chatId, title);
  },
  clearChat(chatId: string): Promise<ChatOperationResult> {
    return ClearChat(chatId);
  },
  newChatSession(chatId: string): Promise<ChatOperationResult> {
    return NewChatSession(chatId);
  },
  setChatArchived(chatId: string, archived: boolean): Promise<ChatOperationResult> {
    return SetChatArchived(chatId, archived);
  },
  deleteChat(chatId: string): Promise<ChatOperationResult> {
    return DeleteChat(chatId);
  },
  compactChat(chatId: string): Promise<ChatOperationResult> {
    return CompactChat(chatId);
  },
  getChatSessionInfo(chatId: string): Promise<ChatSessionInfo> {
    return GetChatSessionInfo(chatId);
  },
  getPlanMode(chatId: string): Promise<PlanModeResult> {
    return GetPlanMode(chatId);
  },
  setPlanMode(chatId: string, enabled: boolean): Promise<PlanModeResult> {
    return SetPlanMode(chatId, enabled);
  },
  setChatModel(chatId: string, provider: string, model: string): Promise<ChatModelResult> {
    return SetChatModel(chatId, provider, model);
  },
  openPipWindow(chatId: string): Promise<OperationResult> {
    return OpenPipWindow(chatId);
  },
  resolveToolConfirmation(id: string, approved: boolean): Promise<OperationResult> {
    return ResolveToolConfirmation(id, approved);
  },
};
