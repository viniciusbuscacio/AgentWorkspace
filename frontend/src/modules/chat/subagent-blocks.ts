import type { SubagentTask } from './subagent-tasks';

export interface SubagentBlock {
  task: string;
  status: string;
  result: string;
  body: string;
}

export type MessagePart =
  | { type: 'markdown'; markdown: string }
  | { type: 'subagent'; block: SubagentBlock }
  | { type: 'spawn'; runId: string };

const startPattern = /^::subagent\{([^}]*)\}\s*$/;
const endMarker = '::end-subagent';
// Marker the backend injects at the exact point a system.spawn call happened;
// the chat UI swaps it for the subagent card of that run. Live replies carry
// the bare marker line; persisted replies carry the full block:
//   ::spawn{run="..."}
//   {"tasks":[{"id","task","status","output","elapsedMs"}]}
//   ::end-spawn
const spawnPattern = /^::spawn\{run="([^"]+)"\}\s*$/;
const spawnEndMarker = '::end-spawn';

/** The literal marker line for a run, for content checks (see MessageList). */
export function spawnMarker(runId: string): string {
  return `::spawn{run="${runId}"}`;
}

export type SpawnSegment = { type: 'text'; text: string } | { type: 'spawn'; runId: string; tasks?: SubagentTask[] };

function parseSpawnTasks(body: string): SubagentTask[] | undefined {
  try {
    const payload = JSON.parse(body) as { tasks?: SubagentTask[] };
    return Array.isArray(payload.tasks) && payload.tasks.length ? payload.tasks : undefined;
  } catch {
    return undefined;
  }
}

// If a ::end-spawn closes a results body right after the marker at lines[start],
// returns its parsed tasks and the index of the ::end-spawn line.
function readSpawnBody(lines: string[], start: number): { tasks?: SubagentTask[]; end: number } | null {
  const body: string[] = [];
  for (let i = start + 1; i < lines.length; i += 1) {
    if (lines[i].trim() === spawnEndMarker) {
      return { tasks: parseSpawnTasks(body.join('\n')), end: i };
    }
    // Another marker before ::end-spawn: the one at `start` was bare — do not
    // swallow the next run's block as this one's body.
    if (spawnPattern.test(lines[i])) return null;
    body.push(lines[i]);
  }
  return null;
}

/**
 * Split assistant content at ::spawn markers/blocks so the timeline can render
 * bubble -> spawn card -> bubble. Everything else (including ::subagent blocks)
 * stays inside the text segments for Message's own parser.
 */
export function splitAtSpawnMarkers(content: string): SpawnSegment[] {
  const source = String(content || '');
  if (!source.includes('::spawn{')) return [{ type: 'text', text: source }];
  const lines = source.split(/\r?\n/);
  const segments: SpawnSegment[] = [];
  let buffer: string[] = [];

  function flushText() {
    const text = buffer.join('\n').trim();
    if (text) segments.push({ type: 'text', text });
    buffer = [];
  }

  for (let i = 0; i < lines.length; i += 1) {
    const match = lines[i].match(spawnPattern);
    if (!match) {
      buffer.push(lines[i]);
      continue;
    }
    flushText();
    const block = readSpawnBody(lines, i);
    if (block) {
      segments.push({ type: 'spawn', runId: match[1], tasks: block.tasks });
      i = block.end;
    } else {
      // Bare marker (live/streaming reply): the card state comes from memory.
      segments.push({ type: 'spawn', runId: match[1] });
    }
  }
  flushText();
  return segments;
}

export function parseSubagentBlocks(markdown: string): MessagePart[] {
  const lines = String(markdown || '').split(/\r?\n/);
  const parts: MessagePart[] = [];
  let markdownBuffer: string[] = [];

  function flushMarkdown() {
    const value = markdownBuffer.join('\n').trim();
    if (value) parts.push({ type: 'markdown', markdown: value });
    markdownBuffer = [];
  }

  for (let i = 0; i < lines.length; i += 1) {
    const spawnMatch = lines[i].match(spawnPattern);
    if (spawnMatch) {
      flushMarkdown();
      parts.push({ type: 'spawn', runId: spawnMatch[1] });
      const block = readSpawnBody(lines, i);
      if (block) i = block.end;
      continue;
    }
    const match = lines[i].match(startPattern);
    if (!match) {
      markdownBuffer.push(lines[i]);
      continue;
    }

    const body: string[] = [];
    let closed = false;
    for (i += 1; i < lines.length; i += 1) {
      if (lines[i].trim() === endMarker) {
        closed = true;
        break;
      }
      body.push(lines[i]);
    }

    if (!closed) {
      markdownBuffer.push(lines[i - body.length - 1], ...body);
      break;
    }

    flushMarkdown();
    const attrs = parseAttrs(match[1]);
    parts.push({
      type: 'subagent',
      block: {
        task: attrs.task || 'Subagent',
        status: attrs.status || 'running',
        result: attrs.result || '',
        body: body.join('\n').trim(),
      },
    });
  }

  flushMarkdown();
  return parts;
}

function parseAttrs(input: string): Record<string, string> {
  const attrs: Record<string, string> = {};
  const attrPattern = /(\w+)="([^"]*)"/g;
  for (const match of input.matchAll(attrPattern)) {
    attrs[match[1]] = match[2];
  }
  return attrs;
}
