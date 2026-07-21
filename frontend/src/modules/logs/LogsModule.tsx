import { useCallback, useEffect, useMemo, useState } from 'react';
import { notify as setMessage } from '@/lib/notify';
import { Button } from '@ui/button';
import { Input } from '@ui/input';
import { ZoomSafeSelect, type ZoomSafeSelectOption } from '@ui/zoom-safe-select';
import { logsService, type LogDateCount, type LogEntry } from '@services/logs.service';

const PAGE_SIZE = 50;
const LEVELS = [
  [0, 'Emergency'], [1, 'Alert'], [2, 'Critical'], [3, 'Error'],
  [4, 'Warning'], [5, 'Notice'], [6, 'Info'], [7, 'Debug'],
] as const;

export function LogsModule() {
  const [logs, setLogs] = useState<LogEntry[]>([]);
  const [dates, setDates] = useState<LogDateCount[]>([]);
  const [date, setDate] = useState('');
  const [level, setLevel] = useState('all');
  const [source, setSource] = useState('all');
  const [search, setSearch] = useState('');
  const [eventPrefix, setEventPrefix] = useState('');
  const [traceId, setTraceId] = useState('');
  const [status, setStatus] = useState('all');
  const [risk, setRisk] = useState('all');
  const [retentionDays, setRetentionDays] = useState(7);
  const [confirmClean, setConfirmClean] = useState(false);
  const [confirmClearAll, setConfirmClearAll] = useState(false);
  const [offset, setOffset] = useState(0);

  const sources = useMemo(() => Array.from(new Set(logs.map((log) => log.source).filter(Boolean))).sort(), [logs]);

  const dateOptions = useMemo<ZoomSafeSelectOption[]>(
    () => [{ value: '', label: 'All dates' }, ...dates.map((item) => ({ value: item.date, label: `${item.date} (${item.entries})` }))],
    [dates],
  );
  const levelOptions = useMemo<ZoomSafeSelectOption[]>(
    () => [{ value: 'all', label: 'All levels' }, ...LEVELS.map(([value, label]) => ({ value: String(value), label }))],
    [],
  );
  const sourceOptions = useMemo<ZoomSafeSelectOption[]>(
    () => [{ value: 'all', label: 'All sources' }, ...sources.map((item) => ({ value: item, label: item }))],
    [sources],
  );
  const statusOptions = useMemo<ZoomSafeSelectOption[]>(
    () => [{ value: 'all', label: 'All status' }, ...['ok', 'error', 'blocked', 'canceled', 'denied', 'timeout'].map((item) => ({ value: item, label: item }))],
    [],
  );
  const riskOptions = useMemo<ZoomSafeSelectOption[]>(
    () => [{ value: 'all', label: 'All risk' }, ...['low', 'medium', 'high'].map((item) => ({ value: item, label: item }))],
    [],
  );

  const refresh = useCallback(async (nextOffset = 0, append = false) => {
    const parsedLevel = level === 'all' ? -1 : Number(level);
    const result = await logsService.list(date, parsedLevel, source, search, eventPrefix, traceId, status, '', '', risk, PAGE_SIZE, nextOffset);
    if (!result.success) {
      setMessage(result.error || 'Could not load logs.');
      return;
    }
    setLogs((current) => append ? [...current, ...(result.logs ?? [])] : result.logs ?? []);
    setOffset(nextOffset + (result.logs?.length ?? 0));
    const datesResult = await logsService.dates();
    if (datesResult.success) setDates(datesResult.dates ?? []);
  }, [date, eventPrefix, level, risk, search, source, status, traceId]);

  const traceRows = useMemo(() => {
    const selected = traceId.trim();
    if (!selected) return [];
    return logs.filter((log) => log.traceId === selected).slice().reverse();
  }, [logs, traceId]);

  useEffect(() => {
    void refresh(0, false);
  }, [refresh]);

  async function cleanOld() {
    const days = clampRetentionDays(retentionDays);
    if (days !== retentionDays) setRetentionDays(days);
    if (!confirmClean) {
      setConfirmClean(true);
      setConfirmClearAll(false);
      setMessage(`Click Confirm retention clean to delete logs before the last ${days} day${days === 1 ? '' : 's'}.`);
      return;
    }
    const result = await logsService.cleanOld(days);
    setConfirmClean(false);
    setMessage(result.success ? `Deleted ${result.deleted} old logs.` : result.error || 'Could not clean logs.');
    await refresh(0, false);
  }

  async function clearAll() {
    if (!confirmClearAll) {
      setConfirmClearAll(true);
      setConfirmClean(false);
      setMessage('Click Confirm clear all to delete every log entry, including today.');
      return;
    }
    const result = await logsService.clearAll();
    setConfirmClearAll(false);
    setMessage(result.success ? `Deleted ${result.deleted} logs.` : result.error || 'Could not clear logs.');
    await refresh(0, false);
  }

  return (
    <section className="home-screen" data-awid="logs-screen">
      <div className="home-header">
        <h1 className="home-title">Logs</h1>
        <p className="home-subtitle">Action dispatches, lifecycle events, blocks and errors.</p>
      </div>
      <div className="flex flex-wrap items-center gap-2 rounded-lg border border-border bg-card p-3">
        <div className="w-40"><ZoomSafeSelect value={date} onValueChange={setDate} options={dateOptions} aria-label="Log date" /></div>
        <div className="w-32"><ZoomSafeSelect value={level} onValueChange={setLevel} options={levelOptions} aria-label="Log level" /></div>
        <div className="w-36"><ZoomSafeSelect value={source} onValueChange={setSource} options={sourceOptions} aria-label="Log source" /></div>
        <div className="w-32"><ZoomSafeSelect value={status} onValueChange={setStatus} options={statusOptions} aria-label="Log status" /></div>
        <div className="w-28"><ZoomSafeSelect value={risk} onValueChange={setRisk} options={riskOptions} aria-label="Safety risk" /></div>
        <Input className="w-44" value={eventPrefix} onChange={(event) => setEventPrefix(event.target.value)} onKeyDown={(event) => { if (event.key === 'Enter') void refresh(0, false); }} placeholder="Event prefix..." aria-label="Event prefix" />
        <Input className="w-44" value={traceId} onChange={(event) => setTraceId(event.target.value)} onKeyDown={(event) => { if (event.key === 'Enter') void refresh(0, false); }} placeholder="Trace/run id..." aria-label="Trace id" />
        <Input className="max-w-xs" value={search} onChange={(event) => setSearch(event.target.value)} onKeyDown={(event) => { if (event.key === 'Enter') void refresh(0, false); }} placeholder="Search logs..." aria-label="Search logs" />
        <Button variant="outline" icon="search" onClick={() => void refresh(0, false)}>Search</Button>
        <Button variant="outline" icon="refresh" onClick={() => void refresh(0, false)}>Refresh</Button>
        <div className="ml-auto flex items-center gap-2">
          <span className="text-xs text-muted-foreground">Keep days</span>
          <Input className="w-20" type="number" min={1} max={90} value={retentionDays} onChange={(event) => { setRetentionDays(clampRetentionDays(Number(event.target.value))); setConfirmClean(false); }} aria-label="Retention days" />
          <Button variant={confirmClean ? 'destructive' : 'outline'} icon="delete_sweep" onClick={() => void cleanOld()}>
            {confirmClean ? 'Confirm retention clean' : 'Clean by retention'}
          </Button>
          <Button variant={confirmClearAll ? 'destructive' : 'outline'} icon="delete" onClick={() => void clearAll()}>
            {confirmClearAll ? 'Confirm clear all' : 'Clear all logs'}
          </Button>
        </div>
      </div>
      <div className="mt-4 flex flex-col gap-2">
        {logs.length === 0 && <div className="rounded-md border border-border bg-card p-4 text-sm text-muted-foreground">No logs found.</div>}
        {logs.map((log) => <LogRow key={log.id} log={log} />)}
      </div>
      {traceRows.length > 0 && (
        <section className="mt-4 border-t border-border pt-4" aria-label="Trace view">
          <h2 className="mb-2 text-sm font-medium">Trace {shortId(traceId)}</h2>
          <div className="flex flex-col gap-1">
            {traceRows.map((log) => <TraceLine key={`trace-${log.id}`} log={log} />)}
          </div>
        </section>
      )}
      <div className="mt-4 flex items-center gap-3">
        <Button variant="outline" icon="expand_more" disabled={logs.length < offset} onClick={() => void refresh(offset, true)}>Load more</Button>
      </div>
    </section>
  );
}

function clampRetentionDays(value: number) {
  if (!Number.isFinite(value)) return 7;
  return Math.min(90, Math.max(1, Math.trunc(value)));
}

function LogRow({ log }: { log: LogEntry }) {
  const [open, setOpen] = useState(false);
  const attrs = parseAttributes(log.attributesJson);
  return (
    <article className="rounded-md border border-border bg-card">
      <button type="button" className="grid w-full grid-cols-[88px_74px_minmax(150px,1fr)_minmax(120px,1.2fr)] items-start gap-3 p-3 text-left text-sm" onClick={() => setOpen((value) => !value)}>
        <span className="font-mono text-xs text-muted-foreground">{formatClock(log.timestamp)}</span>
        <span className={`rounded-sm border px-2 py-1 text-center text-xs ${levelClass(log.level)}`}>{severityLabel(log)}</span>
        <span className="min-w-0">
          <span className="font-mono text-xs">{log.event || log.message}</span>
          <span className="ml-2 text-xs text-muted-foreground">{log.source}</span>
          {log.status && <span className="ml-2 text-xs text-muted-foreground">{log.status}</span>}
        </span>
        <span className="min-w-0 truncate font-mono text-xs text-muted-foreground">{compactAttributes(log, attrs)}</span>
      </button>
      {open && (
        <pre className="max-h-72 overflow-auto border-t border-border p-3 text-xs text-muted-foreground">{JSON.stringify(structuredLog(log, attrs), null, 2)}</pre>
      )}
    </article>
  );
}

function TraceLine({ log }: { log: LogEntry }) {
  const depth = log.parentSpanId ? 1 : 0;
  return (
    <div className="grid grid-cols-[88px_minmax(180px,1fr)_120px_120px] gap-3 text-xs" style={{ paddingLeft: `${depth * 18}px` }}>
      <span className="font-mono text-muted-foreground">{formatClock(log.timestamp)}</span>
      <span className="font-mono">{log.event || log.message}</span>
      <span className="text-muted-foreground">{log.status || 'ok'}</span>
      <span className="text-muted-foreground">{formatDuration(log.durationMs)}</span>
    </div>
  );
}

function formatClock(value: string) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleTimeString(undefined, { hour12: false, hour: '2-digit', minute: '2-digit', second: '2-digit' });
}

function severityLabel(log: LogEntry) {
  return (log.severity || log.levelName || 'info').toUpperCase();
}

function parseAttributes(value?: string): Record<string, unknown> {
  if (!value) return {};
  try {
    const parsed = JSON.parse(value);
    return parsed && typeof parsed === 'object' && !Array.isArray(parsed) ? parsed as Record<string, unknown> : {};
  } catch {
    return {};
  }
}

function compactAttributes(log: LogEntry, attrs: Record<string, unknown>) {
  const pieces = [
    log.traceId ? `trace=${shortId(log.traceId)}` : '',
    log.durationMs ? `duration=${formatDuration(log.durationMs)}` : '',
    typeof attrs['model'] === 'string' ? `model=${attrs['model']}` : '',
    typeof attrs['action'] === 'string' ? `action=${attrs['action']}` : '',
    typeof attrs['risk'] === 'string' ? `risk=${attrs['risk']}` : '',
    typeof attrs['tokens.total'] === 'number' ? `tokens=${attrs['tokens.total']}` : '',
  ].filter(Boolean);
  return pieces.join(' ');
}

function structuredLog(log: LogEntry, attrs: Record<string, unknown>) {
  return {
    ...log,
    attributes: attrs,
  };
}

function shortId(value: string) {
  return value.length > 12 ? `${value.slice(0, 12)}...` : value;
}

function formatDuration(value?: number) {
  if (!value) return '';
  if (value < 1000) return `${value}ms`;
  return `${(value / 1000).toFixed(1)}s`;
}

function levelClass(level: number) {
  if (level <= 3) return 'border-destructive/70 bg-destructive/15 text-destructive';
  if (level === 4) return 'border-primary/40 bg-primary/10 text-primary';
  return 'border-border bg-muted text-muted-foreground';
}
