import { useState } from 'react';
import { Button } from '@ui/button';
import type { PromptDebugSnapshot } from './prompt-debug-messages';

function snapshotMarkdown(snapshot: PromptDebugSnapshot): string {
  const images = snapshot.images?.length
    ? snapshot.images.map((img) => `- ${img.mimeType}, data length: ${img.dataLength}`).join('\n')
    : '- none';
  const tools = snapshot.activeTools?.length
    ? snapshot.activeTools.map((tool) => `- ${tool}`).join('\n')
    : '- none';
  return `# Prompt Debug Snapshot\n\n` +
    `- id: ${snapshot.id}\n` +
    `- timestamp: ${snapshot.timestamp}\n` +
    `- provider: ${snapshot.provider || ''}\n` +
    `- model: ${snapshot.model || ''}\n` +
    `- moduleId: ${snapshot.moduleId || ''}\n` +
    `- turn: ${snapshot.turn ?? ''}\n` +
    `- planMode: ${snapshot.planMode}\n\n` +
    `## System Prompt\n\n${snapshot.systemPrompt || ''}\n\n` +
    `## User Prompt\n\n${snapshot.userPrompt || ''}\n\n` +
    `## Images\n\n${images}\n\n` +
    `## Active Tools\n\n${tools}\n\n` +
    `## Raw Snapshot JSON\n\n\`\`\`json\n${JSON.stringify(snapshot, null, 2)}\n\`\`\`\n`;
}

export function PromptDebugCard({ snapshot }: { snapshot: PromptDebugSnapshot }) {
  const [copied, setCopied] = useState(false);
  const [expanded, setExpanded] = useState(true);
  const title = `Prompt Debug${snapshot.turn ? ` - Turn ${snapshot.turn}` : ''}`;
  const meta = [snapshot.provider, snapshot.model].filter(Boolean).join(' / ') || 'provider/model unavailable';
  const copy = async () => {
    await navigator.clipboard.writeText(snapshotMarkdown(snapshot));
    setCopied(true);
    window.setTimeout(() => setCopied(false), 1500);
  };
  return (
    <div className="w-full shrink-0 overflow-hidden rounded-lg border border-primary/40 bg-primary/10 text-foreground" data-testid="prompt-debug-card">
      <div className="flex select-none items-center justify-between gap-3 px-3 py-2 text-sm font-semibold">
        <Button
          type="button"
          variant="ghost"
          size="sm"
          className="min-w-0 flex-1 justify-start gap-2 px-0 hover:bg-transparent"
          aria-expanded={expanded}
          onClick={() => setExpanded((value) => !value)}
        >
          <span className="material-symbols-outlined text-[18px]" aria-hidden="true">bug_report</span>
          <span className="truncate">{title}</span>
          <span className="truncate text-xs font-normal text-muted-foreground">{meta}</span>
          <span className="material-symbols-outlined ml-auto text-[18px]" aria-hidden="true">{expanded ? 'expand_less' : 'expand_more'}</span>
        </Button>
        <Button type="button" variant="outline" size="sm" onClick={() => void copy()}>{copied ? 'Copied' : 'Copy'}</Button>
      </div>
      {expanded && (
        <div className="grid gap-3 border-t border-primary/25 p-3">
          <div className="grid grid-cols-[repeat(auto-fit,minmax(150px,1fr))] gap-2 text-xs">
            <Info label="Provider" value={snapshot.provider || 'unknown'} />
            <Info label="Model" value={snapshot.model || 'unknown'} />
            <Info label="Module" value={snapshot.moduleId || 'unknown'} />
            <Info label="Plan Mode" value={snapshot.planMode ? 'ON' : 'OFF'} />
            <Info label="Images" value={String(snapshot.images?.length || 0)} />
            <Info label="Tools" value={String(snapshot.activeTools?.length || 0)} />
          </div>
          <Section title="System Prompt" text={snapshot.systemPrompt || ''} />
          <Section title="User Prompt" text={snapshot.userPrompt || ''} />
          <Section title="Active Tools" text={snapshot.activeTools?.length ? snapshot.activeTools.join('\n') : 'none'} />
          <Section title="Raw Snapshot JSON" text={JSON.stringify(snapshot, null, 2)} />
        </div>
      )}
    </div>
  );
}

function Info({ label, value }: { label: string; value: string }) {
  return <div className="rounded-md border border-border bg-background p-2"><span className="block text-muted-foreground">{label}</span>{value}</div>;
}

function Section({ title, text }: { title: string; text: string }) {
  return (
    <section>
      <h4 className="mb-1 text-xs font-semibold uppercase text-muted-foreground">{title}</h4>
      <pre className="max-h-[340px] overflow-auto whitespace-pre-wrap break-words rounded-md border border-border bg-background p-3 font-mono text-[11px] leading-relaxed text-foreground">{text}</pre>
    </section>
  );
}
