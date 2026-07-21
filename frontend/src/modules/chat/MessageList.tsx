import { Fragment } from 'react';
import { domain } from '@services/models';
import { Message } from './Message';
import { SpawnIndicator } from './SpawnIndicator';
import { spawnMarker, splitAtSpawnMarkers } from './subagent-blocks';
import type { SpawnBlock } from './subagent-tasks';

interface MessageListProps {
  messages: domain.Message[];
  spawnBlocks?: SpawnBlock[];
}

type StreamingFlags = { pending?: boolean; streaming?: boolean };

// Renders the vanilla `.chat-timeline` inner content (messages + bottom spacer),
// or the empty state (big eyeglasses_2 glyph). The scrolling `.chat-scroll`
// wrapper lives in ChatModule so the auto-scroll hook can hold its ref.
//
// An assistant message with ::spawn markers renders as separate bubbles with
// the spawn card BETWEEN them at timeline level (message -> card -> message).
// Blocks without a marker in any message fall back to the old placement:
// anchored right after their assistant message (chat:done set its messageId),
// or trailing the list while live/unanchored.
export function MessageList({ messages, spawnBlocks = [] }: MessageListProps) {
  if (messages.length === 0 && spawnBlocks.length === 0) {
    return (
      <div className="chat-timeline is-empty">
        <div className="empty-chat-state">
          <span className="material-symbols-outlined empty-chat-logo" role="img" aria-label="Agent Workspace">
            eyeglasses_2
          </span>
        </div>
      </div>
    );
  }

  const messageIds = new Set(messages.map((message) => message.id));
  const blocksByRun = new Map(spawnBlocks.map((block) => [block.runId, block]));
  // A block whose ::spawn marker is in some assistant text (streaming or
  // persisted) renders at that marker — never in the timeline fallbacks too.
  const markedRuns = new Set<string>();
  for (const block of spawnBlocks) {
    if (messages.some((message) => message.role === 'assistant' && message.content?.includes(spawnMarker(block.runId)))) {
      markedRuns.add(block.runId);
    }
  }
  const anchoredByMessage = new Map<string, SpawnBlock[]>();
  for (const block of spawnBlocks) {
    if (markedRuns.has(block.runId)) continue;
    if (block.messageId && messageIds.has(block.messageId)) {
      const list = anchoredByMessage.get(block.messageId) ?? [];
      list.push(block);
      anchoredByMessage.set(block.messageId, list);
    }
  }
  // Live blocks, or ones whose assistant message hasn't loaded yet, trail the list.
  const trailing = spawnBlocks.filter((block) => !markedRuns.has(block.runId) && !(block.messageId && messageIds.has(block.messageId)));

  // A live reply bubble ("..."/streaming) at the tail is chronologically AFTER
  // a spawn that already ran: unanchored cards pin BEFORE it, at the point in
  // time the subagent ran, instead of floating below the forming reply.
  let tailStart = messages.length;
  while (tailStart > 0) {
    const tail = messages[tailStart - 1] as domain.Message & StreamingFlags;
    if (tail.role === 'assistant' && (tail.pending || tail.streaming)) tailStart -= 1;
    else break;
  }

  const renderMessage = (message: domain.Message) => (
    <Fragment key={message.id}>
      {message.role === 'assistant' && message.content?.includes('::spawn{')
        ? renderSplitAssistantMessage(message, blocksByRun)
        : <Message message={message} />}
      {(anchoredByMessage.get(message.id) ?? []).map((block) => (
        <SpawnIndicator key={block.runId} tasks={block.tasks} />
      ))}
    </Fragment>
  );

  return (
    <div className="chat-timeline">
      {messages.slice(0, tailStart).map(renderMessage)}
      {trailing.map((block) => <SpawnIndicator key={block.runId} tasks={block.tasks} />)}
      {messages.slice(tailStart).map(renderMessage)}
      <div className="chat-bottom-spacer" aria-hidden="true" />
    </div>
  );
}

// Bubble -> card -> bubble for one assistant message: each text segment is its
// own bubble; the run's live card sits between them. Time, streaming flags and
// attachments stay on the last segment only, so they don't duplicate.
function renderSplitAssistantMessage(message: domain.Message, blocksByRun: ReadonlyMap<string, SpawnBlock>) {
  const segments = splitAtSpawnMarkers(message.content || '');
  const lastTextIndex = segments.reduce((last, segment, index) => (segment.type === 'text' ? index : last), -1);
  const renderedRuns = new Set<string>();
  return segments.map((segment, index) => {
    if (segment.type === 'spawn') {
      // First marker of a run wins. The live in-memory block has the freshest
      // state; a persisted results block (segment.tasks) keeps the card alive
      // after chat reload or app restart. Neither -> render nothing.
      const tasks = blocksByRun.get(segment.runId)?.tasks ?? segment.tasks;
      if (!tasks?.length || renderedRuns.has(segment.runId)) return null;
      renderedRuns.add(segment.runId);
      return <SpawnIndicator key={`${message.id}-spawn-${segment.runId}`} tasks={tasks} />;
    }
    const isLast = index === lastTextIndex;
    const flags = message as domain.Message & StreamingFlags;
    const segmentMessage = Object.assign(
      new domain.Message({
        ...message,
        content: segment.text,
        createdAt: isLast ? message.createdAt : '',
        attachments: isLast ? message.attachments : [],
      }),
      isLast ? { pending: flags.pending, streaming: flags.streaming } : {},
    );
    return <Message key={`${message.id}-seg-${index}`} message={segmentMessage} />;
  });
}
