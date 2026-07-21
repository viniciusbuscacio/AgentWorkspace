import { CleanOldLogs, ClearAllLogs, ListLogDates, ListLogs } from '@wails/go/main/App';
import type { dto } from '@wails/go/models';

export interface LogEntry {
  id: string;
  timestamp: string;
  level: number;
  levelName: string;
  source: string;
  moduleId?: string;
  moduleType?: string;
  sessionId?: string;
  message: string;
  contextJson?: string;
  errorName?: string;
  errorMessage?: string;
  errorStack?: string;
  createdAt: string;
  event?: string;
  severity?: string;
  traceId?: string;
  spanId?: string;
  parentSpanId?: string;
  durationMs?: number;
  status?: string;
  attributesJson?: string;
}

export interface LogDateCount {
  date: string;
  entries: number;
}

export type LogsResult = dto.LogsResult & { logs?: LogEntry[] };
export type LogDatesResult = dto.LogDatesResult & { dates?: LogDateCount[] };

export const logsService = {
  list(
    date: string,
    level: number,
    source: string,
    search: string,
    eventPrefix: string,
    traceId: string,
    status: string,
    moduleId: string,
    sessionId: string,
    risk: string,
    limit: number,
    offset: number,
  ): Promise<LogsResult> {
    return ListLogs(date, level, source, search, eventPrefix, traceId, status, moduleId, sessionId, risk, limit, offset) as Promise<LogsResult>;
  },
  dates(): Promise<LogDatesResult> {
    return ListLogDates() as Promise<LogDatesResult>;
  },
  cleanOld(days: number): Promise<dto.LogRetentionResult> {
    return CleanOldLogs(days);
  },
  clearAll(): Promise<dto.LogRetentionResult> {
    return ClearAllLogs();
  },
};
