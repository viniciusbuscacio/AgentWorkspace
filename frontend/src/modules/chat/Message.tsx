import type { domain } from '@services/models';
import { useEffect, useMemo, useRef, useState } from 'react';
import { renderMarkdownFragment } from '@/lib/markdown';
import { PromptDebugCard } from './PromptDebugCard';
import type { PromptDebugChatMessage } from './prompt-debug-messages';
import { parseSubagentBlocks, type SubagentBlock } from './subagent-blocks';

type ChatMessage = domain.Message & {
  pending?: boolean;
  streaming?: boolean;
} & PromptDebugChatMessage;

interface MessageProps {
  message: ChatMessage;
}

// Ported 1:1 from the vanilla frontend (formatTime in main.js).
function formatTime(value?: string): string {
  if (!value) return '';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return '';
  return date.toLocaleTimeString('pt-BR', { hour: '2-digit', minute: '2-digit' });
}

function imageList(attachments?: domain.Attachment[]): domain.Attachment[] {
  return (attachments || []).filter((a) => String(a.type || '').startsWith('image/') && Boolean(a.dataUri));
}

// Non-image attachments (PDF/DOCX/text/any): rendered as file chips so the user
// sees what they actually sent, even though only the extracted text reaches the
// model.
function fileList(attachments?: domain.Attachment[]): domain.Attachment[] {
  return (attachments || []).filter((a) => !String(a.type || '').startsWith('image/'));
}

function fileIcon(type: string, name: string): string {
  const t = type.toLowerCase();
  const n = name.toLowerCase();
  if (t.includes('pdf') || n.endsWith('.pdf')) return 'picture_as_pdf';
  if (t.includes('word') || n.endsWith('.docx') || n.endsWith('.doc')) return 'description';
  if (t.startsWith('audio/')) return 'graphic_eq';
  if (t.startsWith('text/') || t.includes('json') || t.includes('xml')) return 'article';
  return 'attach_file';
}

// Mirrors renderMessage() in the vanilla main.js: message-row > bubble + message-time,
// both user and assistant left-aligned (CSS in theme/aw-chat.css drives the look).
export function Message({ message }: MessageProps) {
  const [previewImage, setPreviewImage] = useState<string | null>(null);
  const images = useMemo(() => imageList(message.attachments), [message.attachments]);
  const files = useMemo(() => fileList(message.attachments), [message.attachments]);
  const time = formatTime(message.createdAt);
  const isUser = message.role === 'user';
  const assistantParts = useMemo(() => (!isUser && message.content ? parseSubagentBlocks(message.content) : []), [isUser, message.content]);

  if (message.promptDebug) {
    return <PromptDebugCard snapshot={message.promptDebug} />;
  }

  if (message.role === 'system') {
    if (images.length) {
      return (
        <div className="message-row system-image">
          <div className="bubble image-bubble">
            {message.content && <div className="image-bubble-title">{message.content}</div>}
            <ImageAttachments images={images} onPreview={setPreviewImage} />
          </div>
          {time && <span className="message-time">{time}</span>}
          <ImagePreview src={previewImage} onClose={() => setPreviewImage(null)} />
        </div>
      );
    }
    // Vanilla: kind defaults to 'centered' for "Stopped", otherwise 'command' (markdown).
    if (message.content === 'Stopped') {
      return <div className="system-message centered stopped">{message.content}</div>;
    }
    return (
      <div className="system-message command">
        <SafeMarkdown markdown={message.content} className="markdown-content system-markdown" />
      </div>
    );
  }

  // "active" = the run is still going (the indicator must show). The hook
  // flips pending→streaming on the first delta, so checking only pending hid
  // the indicator mid-stream.
  const active = Boolean(message.pending || message.streaming);
  const cleanUser = String(message.content || '').replace(/\[Image:\s*[^\]]+\]/g, '').trim();

  return (
    <div className={`message-row ${isUser ? 'user' : 'assistant'}${active ? ' pending' : ''}`} data-message-id={message.id}>
      <div className="bubble">
        {isUser ? (
          <>
            {cleanUser && <div className="user-content">{cleanUser}</div>}
            <ImageAttachments images={images} onPreview={setPreviewImage} />
            <FileAttachments files={files} />
          </>
        ) : message.content ? (
          // Streaming with text: render it and trail a single pulsing dot at
          // the end of the last line until the run completes.
          <AssistantContent parts={assistantParts} active={active} />
        ) : active ? (
          // No text yet: the classic 3-dot "thinking" bubble.
          <TypingIndicator />
        ) : null}
      </div>
      {time && <span className="message-time">{time}</span>}
      <ImagePreview src={previewImage} onClose={() => setPreviewImage(null)} />
    </div>
  );
}

function AssistantContent({ parts, active }: { parts: ReturnType<typeof parseSubagentBlocks>; active: boolean }) {
  if (parts.length === 0) return null;
  if (parts.length === 1 && parts[0].type === 'markdown') {
    return <SafeMarkdown markdown={parts[0].markdown} className="markdown-content" trailingDot={active} />;
  }
  return (
    <div className="assistant-content">
      {parts.map((part, index) => {
        if (part.type === 'subagent') return <SubagentCard key={`subagent-${index}`} block={part.block} />;
        // A stray ::spawn marker (MessageList splits them out before they get
        // here) is stripped, never shown as text.
        if (part.type === 'spawn') return null;
        return <SafeMarkdown key={`markdown-${index}`} markdown={part.markdown} className="markdown-content" trailingDot={active && index === parts.length - 1} />;
      })}
    </div>
  );
}

function SubagentCard({ block }: { block: SubagentBlock }) {
  const status = block.status.trim().toLowerCase() || 'running';
  return (
    <details className={`subagent-block status-${status}`}>
      <summary>
        <span className="material-symbols-outlined subagent-icon" aria-hidden="true">account_tree</span>
        <span className="subagent-task">{block.task}</span>
        <span className="subagent-status">{status}</span>
        {block.result && <span className="subagent-result">{block.result}</span>}
      </summary>
      {block.body && <SafeMarkdown markdown={block.body} className="markdown-content subagent-body" />}
    </details>
  );
}

function TypingIndicator() {
  return (
    <div className="typing-indicator" aria-label="Agent is typing">
      <div className="dot" />
      <div className="dot" />
      <div className="dot" />
    </div>
  );
}

interface ImageAttachmentsProps {
  images: domain.Attachment[];
  onPreview: (dataUri: string) => void;
}

function ImageAttachments({ images, onPreview }: ImageAttachmentsProps) {
  if (!images.length) return null;
  return (
    <div className="image-attachments">
      {images.map((attachment, index) => (
        <img
          key={`${index}-${attachment.name || attachment.dataUri.slice(0, 32)}`}
          className="chat-image"
          src={attachment.dataUri}
          alt={attachment.name || 'Image'}
          onClick={() => onPreview(attachment.dataUri)}
        />
      ))}
    </div>
  );
}

function FileAttachments({ files }: { files: domain.Attachment[] }) {
  if (!files.length) return null;
  return (
    <div className="file-attachments">
      {files.map((attachment, index) => (
        <span key={`${index}-${attachment.name}`} className="file-attachment-chip" title={attachment.name}>
          <span className="material-symbols-outlined" aria-hidden="true">{fileIcon(attachment.type || '', attachment.name || '')}</span>
          <span className="file-attachment-name">{attachment.name || 'file'}</span>
        </span>
      ))}
    </div>
  );
}

function ImagePreview({ src, onClose }: { src: string | null; onClose: () => void }) {
  if (!src) return null;
  return (
    <div className="image-preview-root" style={{ pointerEvents: 'auto' }}>
      <div className="image-preview" onClick={onClose}>
        <img src={src} alt="Preview" />
      </div>
    </div>
  );
}

// DOMPurify-sanitized markdown injected via fragment (never dangerouslySetInnerHTML).
// trailingDot appends a single pulsing dot inline after the last line while the
// agent is still writing, so the user sees it is alive and where the text is.
function SafeMarkdown({ markdown, className, trailingDot = false }: { markdown: string; className: string; trailingDot?: boolean }) {
  const ref = useRef<HTMLDivElement>(null);
  useEffect(() => {
    const root = ref.current;
    if (!root) return;
    root.replaceChildren(renderMarkdownFragment(markdown));
    if (!trailingDot) return;
    const dot = document.createElement('span');
    dot.className = 'typing-dot-single';
    dot.setAttribute('aria-hidden', 'true');
    // Sit inside the last block (p/li/heading) so the dot trails the text on
    // the same line; fall back to the root for content without a trailing block.
    const last = root.lastElementChild;
    if (last && /^(P|LI|H[1-6]|BLOCKQUOTE)$/.test(last.tagName)) {
      last.appendChild(document.createTextNode(' '));
      last.appendChild(dot);
    } else {
      root.appendChild(dot);
    }
  }, [markdown, trailingDot]);
  return <div ref={ref} className={className} />;
}
