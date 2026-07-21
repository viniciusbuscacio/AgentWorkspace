import { useEffect, useRef, useState } from 'react';
import { toast } from 'sonner';
import type { domain } from '@services/models';
import { isSlashDraft, slashSuggestions, type ModelProviderHint, type SlashCommand } from './slash-commands';

interface ComposerProps {
  sending: boolean;
  /** Decides queue-vs-send; the composer clears immediately either way. */
  onSubmit: (text: string, attachments: domain.Attachment[]) => void;
  onStop: () => Promise<void>;
  /** Transcribe a recorded/uploaded audio clip and return editable text. */
  onTranscribeAudio: (fileName: string, mimeType: string, dataUri: string) => Promise<string>;
  /** Native Go-side recording fallback for WebViews without getUserMedia. */
  onStartNativeVoice?: () => Promise<void>;
  /** Stops the native recording and returns the transcript. */
  onStopNativeVoice?: () => Promise<string>;
  onCancelNativeVoice?: () => void;
  onVoiceError: (message: string) => void;
  /** Pop the chat out into its own OS window; hidden when absent or in pip. */
  onOpenPip?: () => void;
  isPip?: boolean;
  /** Stable per-chat draft key. Draft text survives module navigation/unmounts. */
  draftKey?: string;
  /** Resizable textarea height (owned by ChatModule so chat-module can set the CSS var). */
  height: number;
  onResizeStart: (event: React.PointerEvent) => void;
  /** Connected providers used to auto-complete the /model command. */
  modelProviders?: ModelProviderHint[];
}

// Any file is accepted: the backend reader extracts what it can (images via
// OCR, PDF, DOCX, and best-effort text sniffing for everything else) and
// degrades to metadata-only otherwise. Drag-drop never filtered anyway.
const ATTACH_ACCEPT = '*/*';
// Mirrors attachmentsafe.MaxAttachmentBytes so oversized picks fail fast here
// instead of erroring after the base64 round-trip to the backend.
const MAX_ATTACH_BYTES = 16 * 1024 * 1024;
const DRAFT_PREFIX = 'aw.chatDraft:';

// Waveform feedback ported from AW2's VoiceRecordingPanel.
const WAVEFORM_BAR_COUNT = 96;
const EMPTY_WAVEFORM_LEVELS = Array.from({ length: WAVEFORM_BAR_COUNT }, () => 0.08);

function formatRecordingTime(ms: number): string {
  const totalSeconds = Math.max(0, Math.floor(ms / 1000));
  const minutes = Math.floor(totalSeconds / 60);
  const seconds = totalSeconds % 60;
  return `${minutes}:${String(seconds).padStart(2, '0')}`;
}

// DOM + classes ported 1:1 from the vanilla renderComposer(); the look comes
// from theme/aw-chat.css. Left tools = mic (disabled) / attach / pip,
// right actions = send / stop. No plan button (plan is the /plan slash command).
export function Composer({ sending, onSubmit, onStop, onTranscribeAudio, onStartNativeVoice, onStopNativeVoice, onCancelNativeVoice, onVoiceError, onOpenPip, isPip, draftKey = 'default', height, onResizeStart, modelProviders = [] }: ComposerProps) {
  const storageKey = `${DRAFT_PREFIX}${draftKey}`;
  const [text, setText] = useState(() => readDraft(storageKey));
  const [attachments, setAttachments] = useState<domain.Attachment[]>([]);
  const [slashOpen, setSlashOpen] = useState(false);
  const [slashIndex, setSlashIndex] = useState(0);
  const [recording, setRecording] = useState(false);
  const [transcribing, setTranscribing] = useState(false);
  const fileRef = useRef<HTMLInputElement | null>(null);
  const recorderRef = useRef<MediaRecorder | null>(null);
  const audioChunksRef = useRef<Blob[]>([]);
  const audioStreamRef = useRef<MediaStream | null>(null);
  const nativeVoiceRef = useRef(false);
  const [elapsedMs, setElapsedMs] = useState(0);
  const [voiceLevels, setVoiceLevels] = useState<number[]>(EMPTY_WAVEFORM_LEVELS);
  const meterCleanupRef = useRef<(() => void) | null>(null);
  // Send pressed during recording/transcribing: auto-submit once the
  // transcript lands (stop -> transcribe -> send in one click, like AW2).
  const autoSendRef = useRef(false);
  // Live mirrors of the draft: the MediaRecorder.onstop handler is a closure
  // captured at record-start, so its deliverTranscript would otherwise read the
  // text/attachments as they were BEFORE recording — dropping anything (e.g. an
  // image) added while recording. Refs stay current across that stale closure.
  const textRef = useRef(text);
  const attachmentsRef = useRef(attachments);
  textRef.current = text;
  attachmentsRef.current = attachments;

  const slashCommands = isSlashDraft(text) ? slashSuggestions(text, modelProviders) : [];
  const showSlash = slashOpen && slashCommands.length > 0;
  const canSend = text.trim().length > 0 || attachments.length > 0 || recording || transcribing;
  // Merged send/stop: one action button. It is Stop only while a run is in
  // flight AND there is nothing to send; otherwise it is Send (which queues
  // during a run). Esc stops regardless, so a typed draft never hides Stop.
  const showStop = sending && !canSend;

  useEffect(() => {
    setText(readDraft(storageKey));
    setAttachments([]);
    setSlashOpen(false);
    setSlashIndex(0);
  }, [storageKey]);

  useEffect(() => {
    writeDraft(storageKey, text);
  }, [storageKey, text]);

  useEffect(() => () => {
    if (recorderRef.current && recorderRef.current.state !== 'inactive') {
      recorderRef.current.onstop = null;
      recorderRef.current.stop();
    }
    audioStreamRef.current?.getTracks().forEach((track) => track.stop());
    meterCleanupRef.current?.();
    if (nativeVoiceRef.current) {
      nativeVoiceRef.current = false;
      onCancelNativeVoice?.();
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  async function addFiles(files: FileList | null) {
    if (!files || files.length === 0) return;
    const next: domain.Attachment[] = [];
    for (const file of Array.from(files)) {
      if (file.size > MAX_ATTACH_BYTES) {
        toast.error(`${file.name} is too large to attach (limit 16 MB)`);
        continue;
      }
      const dataUri = await fileToDataUri(file);
      if (isAudioFile(file)) {
        await transcribeIntoDraft(file.name, file.type || 'application/octet-stream', dataUri);
        continue;
      }
      next.push({
        name: file.name,
        type: file.type || 'application/octet-stream',
        dataUri,
      } as domain.Attachment);
    }
    if (next.length > 0) setAttachments((current) => [...current, ...next]);
    if (fileRef.current) fileRef.current.value = '';
  }

  function updateText(value: string) {
    setText(value);
    if (isSlashDraft(value)) {
      setSlashOpen(true);
      setSlashIndex(0);
    } else {
      setSlashOpen(false);
    }
  }

  function pickSlash(command: SlashCommand) {
    // Completing a provider leaves the draft as "/model <provider>": press Enter
    // to use its default model, or type a space to open the model list.
    setText(command.name);
    setSlashOpen(false);
    setSlashIndex(0);
  }

  function deliverTranscript(transcript: string) {
    const combined = (current: string) => transcript ? (current.trim() ? `${current.trim()}\n${transcript}` : transcript) : current.trim();
    if (autoSendRef.current) {
      autoSendRef.current = false;
      // Read the draft from refs, not the (possibly record-start) closure, so
      // text/attachments added during recording are included in the auto-send.
      const finalText = combined(textRef.current);
      if (finalText || attachmentsRef.current.length > 0) {
        clearDraft(storageKey);
        onSubmit(finalText, attachmentsRef.current);
        setText('');
        setAttachments([]);
        setSlashOpen(false);
        setSlashIndex(0);
      }
      return;
    }
    if (transcript) {
      setText(combined);
      setSlashOpen(false);
    }
  }

  async function transcribeIntoDraft(fileName: string, mimeType: string, dataUri: string) {
    setTranscribing(true);
    try {
      const transcript = (await onTranscribeAudio(fileName, mimeType, dataUri)).trim();
      deliverTranscript(transcript);
    } catch (err) {
      autoSendRef.current = false;
      onVoiceError(err instanceof Error ? err.message : 'Could not transcribe audio');
    } finally {
      setTranscribing(false);
    }
  }

  async function toggleRecording() {
    if (recording) {
      if (nativeVoiceRef.current) {
        await stopNativeRecording();
        return;
      }
      recorderRef.current?.stop();
      return;
    }
    if (!navigator.mediaDevices?.getUserMedia) {
      if (onStartNativeVoice && onStopNativeVoice) {
        try {
          await onStartNativeVoice();
          nativeVoiceRef.current = true;
          setRecording(true);
          startVoiceMeter();
        } catch (err) {
          onVoiceError(err instanceof Error ? err.message : 'Could not start recording');
        }
        return;
      }
      onVoiceError('Microphone recording is not available in this WebView');
      return;
    }
    try {
      const stream = await navigator.mediaDevices.getUserMedia({ audio: true });
      const mimeType = preferredRecordingMimeType();
      const recorder = mimeType ? new MediaRecorder(stream, { mimeType }) : new MediaRecorder(stream);
      audioStreamRef.current = stream;
      audioChunksRef.current = [];
      recorderRef.current = recorder;
      recorder.ondataavailable = (event) => {
        if (event.data.size > 0) audioChunksRef.current.push(event.data);
      };
      recorder.onerror = () => {
        onVoiceError('Microphone recording failed');
        stopAudioTracks();
        stopVoiceMeter();
        setRecording(false);
      };
      recorder.onstop = () => {
        const chunks = audioChunksRef.current;
        const type = recorder.mimeType || mimeType || 'audio/webm';
        stopAudioTracks();
        stopVoiceMeter();
        setRecording(false);
        recorderRef.current = null;
        audioChunksRef.current = [];
        if (chunks.length === 0) return;
        const blob = new Blob(chunks, { type });
        void blobToDataUri(blob)
          .then((dataUri) => transcribeIntoDraft(`recording-${Date.now()}.webm`, type, dataUri))
          .catch((err: unknown) => onVoiceError(err instanceof Error ? err.message : 'Could not read recording'));
      };
      recorder.start();
      setRecording(true);
      startVoiceMeter(stream);
    } catch (err) {
      stopAudioTracks();
      stopVoiceMeter();
      setRecording(false);
      // WKWebView exposes getUserMedia but denies the capture request (Wails
      // has no media-permission handler), even when macOS granted the app
      // microphone access — the Go-side recorder is the path that grant
      // actually covers, so fall back to it instead of surfacing the denial.
      if (onStartNativeVoice && onStopNativeVoice) {
        try {
          await onStartNativeVoice();
          nativeVoiceRef.current = true;
          setRecording(true);
          startVoiceMeter();
          return;
        } catch (nativeErr) {
          onVoiceError(nativeErr instanceof Error ? nativeErr.message : 'Could not start recording');
          return;
        }
      }
      onVoiceError(err instanceof Error ? err.message : 'Could not access microphone');
    }
  }

  function stopAudioTracks() {
    audioStreamRef.current?.getTracks().forEach((track) => track.stop());
    audioStreamRef.current = null;
  }

  // Recording feedback: timer plus waveform. With a MediaStream the bars show
  // real levels (AW2's analyser); the native Go capture exposes no stream, so
  // the bars get a gentle pseudo-level walk instead of staying frozen.
  function startVoiceMeter(stream?: MediaStream) {
    stopVoiceMeter();
    const startedAt = Date.now();
    setElapsedMs(0);
    setVoiceLevels(EMPTY_WAVEFORM_LEVELS);
    const timer = window.setInterval(() => setElapsedMs(Date.now() - startedAt), 200);
    const cleanups: Array<() => void> = [() => window.clearInterval(timer)];

    let analyserStarted = false;
    if (stream && typeof AudioContext !== 'undefined') {
      try {
        const audioContext = new AudioContext();
        const analyser = audioContext.createAnalyser();
        analyser.fftSize = 512;
        audioContext.createMediaStreamSource(stream).connect(analyser);
        const data = new Uint8Array(analyser.fftSize);
        let lastUpdate = 0;
        let frame = 0;
        const draw = (timestamp: number) => {
          if (timestamp - lastUpdate > 45) {
            analyser.getByteTimeDomainData(data);
            let sum = 0;
            for (let i = 0; i < data.length; i += 1) {
              const centered = ((data[i] ?? 128) - 128) / 128;
              sum += centered * centered;
            }
            const level = Math.max(0.06, Math.min(1, Math.sqrt(sum / data.length) * 5));
            setVoiceLevels((current) => [...current.slice(1), level]);
            lastUpdate = timestamp;
          }
          frame = requestAnimationFrame(draw);
        };
        frame = requestAnimationFrame(draw);
        cleanups.push(() => {
          cancelAnimationFrame(frame);
          void audioContext.close().catch(() => {});
        });
        analyserStarted = true;
      } catch {
        // Recording still works; fall through to the pseudo waveform.
      }
    }
    if (!analyserStarted) {
      let level = 0.25;
      const pseudo = window.setInterval(() => {
        level = Math.max(0.08, Math.min(0.9, level + (Math.random() - 0.5) * 0.3));
        setVoiceLevels((current) => [...current.slice(1), level]);
      }, 90);
      cleanups.push(() => window.clearInterval(pseudo));
    }
    meterCleanupRef.current = () => cleanups.forEach((cleanup) => cleanup());
  }

  function stopVoiceMeter() {
    meterCleanupRef.current?.();
    meterCleanupRef.current = null;
    setVoiceLevels(EMPTY_WAVEFORM_LEVELS);
  }

  async function stopNativeRecording() {
    nativeVoiceRef.current = false;
    stopVoiceMeter();
    setRecording(false);
    setTranscribing(true);
    try {
      const transcript = (await onStopNativeVoice!()).trim();
      deliverTranscript(transcript);
    } catch (err) {
      autoSendRef.current = false;
      onVoiceError(err instanceof Error ? err.message : 'Could not transcribe recording');
    } finally {
      setTranscribing(false);
    }
  }

  function submit() {
    if (recording) {
      autoSendRef.current = true;
      void toggleRecording();
      return;
    }
    if (transcribing) {
      autoSendRef.current = true;
      return;
    }
    const trimmed = text.trim();
    if (!trimmed && attachments.length === 0) return;
    clearDraft(storageKey);
    onSubmit(trimmed, attachments);
    setText('');
    setAttachments([]);
    setSlashOpen(false);
    setSlashIndex(0);
  }

  function onKeyDown(event: React.KeyboardEvent<HTMLTextAreaElement>) {
    if (showSlash) {
      if (event.key === 'ArrowDown') {
        event.preventDefault();
        setSlashIndex((i) => (i + 1) % slashCommands.length);
        return;
      }
      if (event.key === 'ArrowUp') {
        event.preventDefault();
        setSlashIndex((i) => (i - 1 + slashCommands.length) % slashCommands.length);
        return;
      }
      if (event.key === 'Tab' || (event.key === 'Enter' && !event.shiftKey)) {
        event.preventDefault();
        const picked = slashCommands[slashIndex] ?? slashCommands[0];
        // Enter on an already-fully-typed command sends in one press; Tab (or
        // a partial draft) completes the input without sending.
        if (event.key === 'Enter' && text.trim() === picked.name) {
          submit();
        } else {
          pickSlash(picked);
        }
        return;
      }
      if (event.key === 'Escape') {
        event.preventDefault();
        setSlashOpen(false);
        return;
      }
    }
    if (event.key === 'Escape' && sending) {
      // Stop the run from the keyboard even when the merged button is showing
      // Send (a draft is typed) — otherwise a typed draft would hide Stop.
      event.preventDefault();
      void onStop();
      return;
    }
    if (event.key === 'Enter' && !event.shiftKey) {
      event.preventDefault();
      submit();
    }
  }

  return (
    <form
      className={`chat-composer ${isPip ? 'pip-composer' : ''}`}
      onSubmit={(event) => { event.preventDefault(); submit(); }}
      onDragOver={(event) => event.preventDefault()}
      onDrop={(event) => { event.preventDefault(); void addFiles(event.dataTransfer.files); }}
    >
      <input
        ref={fileRef}
        className="hidden-file-input"
        type="file"
        multiple
        accept={ATTACH_ACCEPT}
        onChange={(event) => void addFiles(event.target.files)}
      />
      <div className="composer-resize" title="Drag to resize message box" onPointerDown={onResizeStart} />

      {attachments.length > 0 && (
        <div className="attachment-tray">
          {attachments.map((attachment, index) => (
            <div className="attachment-chip" key={`${attachment.name}-${index}`}>
              {String(attachment.type || '').startsWith('image/')
                ? <img src={attachment.dataUri} alt="" />
                : <span className="attachment-file-icon"><span className="material-symbols-outlined">description</span></span>}
              <span>{attachment.name}</span>
              <button
                type="button"
                title={`Remove ${attachment.name}`}
                onClick={() => setAttachments((current) => current.filter((_, i) => i !== index))}
              >
                <span className="material-symbols-outlined">close</span>
              </button>
            </div>
          ))}
        </div>
      )}

      <div className="composer-row">
        <div className="composer-tools">
          <button className="icon-button" type="button" title="Attach files" aria-label="Attach files" onClick={() => fileRef.current?.click()}>
            <span className="material-symbols-outlined">attach_file</span>
          </button>
          {!isPip && onOpenPip && (
            <button className="icon-button" type="button" title="Open external chat window" aria-label="Open external chat window" onClick={onOpenPip}>
              <span className="material-symbols-outlined">picture_in_picture_alt</span>
            </button>
          )}
        </div>

        <div className="composer-input-wrap">
          {recording ? (
            <div className="voice-panel" data-testid="voice-recording-panel" style={{ height }}>
              {text.trim() && <div className="voice-panel-draft">{text}</div>}
              <div className="voice-wave" aria-label="Voice recording waveform">
                {voiceLevels.map((level, index) => (
                  <span
                    key={index}
                    style={{ height: `${Math.max(3, Math.round(4 + level * 34))}px`, opacity: 0.4 + Math.min(0.6, level) }}
                  />
                ))}
              </div>
              <div className="voice-panel-meta">
                <span className="voice-timer">{formatRecordingTime(elapsedMs)}</span>
                <button
                  className="voice-stop"
                  type="button"
                  title="Stop voice input"
                  aria-label="Stop voice input"
                  onClick={() => void toggleRecording()}
                >
                  <span className="material-symbols-outlined">stop</span>
                </button>
              </div>
            </div>
          ) : transcribing ? (
            <div className="voice-panel processing" data-testid="voice-processing-panel" style={{ height }}>
              {text.trim() && <div className="voice-panel-draft">{text}</div>}
              <div className="voice-wave quiet">
                {EMPTY_WAVEFORM_LEVELS.map((_, index) => (
                  <span key={index} style={{ height: `${6 + (index % 7) * 4}px`, animationDelay: `${index * 18}ms` }} />
                ))}
              </div>
              <div className="voice-panel-meta">
                <span className="voice-processing-label">Transcribing…</span>
              </div>
            </div>
          ) : (
            <textarea
              className="composer-textarea"
              rows={1}
              wrap="soft"
              placeholder="Send message..."
              value={text}
              style={{ height }}
              onChange={(event) => updateText(event.target.value)}
              onKeyDown={onKeyDown}
            />
          )}
          <div className="slash-menu-slot">
            {showSlash && (
              <div className="slash-menu">
                <div className="slash-menu-header">Commands - arrows to navigate, Enter/Tab to select</div>
                <div className="slash-menu-list">
                  {slashCommands.map((command, index) => (
                    <button
                      key={command.name}
                      className={`slash-item ${index === slashIndex ? 'active' : ''}`}
                      type="button"
                      onMouseEnter={() => setSlashIndex(index)}
                      onClick={() => pickSlash(command)}
                    >
                      <span>{command.name}</span>
                      <small>{command.description}</small>
                    </button>
                  ))}
                </div>
              </div>
            )}
          </div>
        </div>

        <div className="composer-actions">
          {showStop ? (
            <button
              className="icon-button stop-button"
              type="button"
              title="Stop (Esc)"
              aria-label="Stop"
              onClick={() => void onStop()}
            >
              <span className="material-symbols-outlined">stop</span>
            </button>
          ) : (
            <button
              className="icon-button send-button"
              type="submit"
              title={recording ? 'Stop, transcribe and send' : transcribing ? 'Send when transcribed' : sending ? 'Queue message' : 'Send (Enter)'}
              aria-label={sending ? 'Queue message' : 'Send'}
              disabled={!canSend}
            >
              <span className="material-symbols-outlined">send</span>
            </button>
          )}
          {recording ? (
            <div className="voice-rec-badge" title="Recording" data-testid="voice-rec-badge">
              <span className="voice-rec-dot" />
              <span>REC</span>
            </div>
          ) : (
            <button
              className="icon-button"
              type="button"
              title={transcribing ? 'Transcribing audio...' : 'Record audio'}
              aria-label={transcribing ? 'Transcribing audio' : 'Record audio'}
              disabled={transcribing}
              onClick={() => void toggleRecording()}
            >
              <span className="material-symbols-outlined">{transcribing ? 'hourglass_top' : 'mic'}</span>
            </button>
          )}
        </div>
      </div>
    </form>
  );
}

function fileToDataUri(file: File): Promise<string> {
  return blobToDataUri(file);
}

function blobToDataUri(blob: Blob): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onerror = () => reject(reader.error ?? new Error('could not read audio'));
    reader.onload = () => resolve(String(reader.result));
    reader.readAsDataURL(blob);
  });
}

function isAudioFile(file: File) {
  return file.type.startsWith('audio/') || /\.(m4a|mp3|mp4|mpeg|mpga|wav|webm)$/i.test(file.name);
}

function preferredRecordingMimeType() {
  const candidates = ['audio/webm;codecs=opus', 'audio/webm', 'audio/mp4'];
  return candidates.find((type) => typeof MediaRecorder !== 'undefined' && MediaRecorder.isTypeSupported(type)) || '';
}

function readDraft(key: string): string {
  try {
    return window.localStorage.getItem(key) ?? '';
  } catch {
    return '';
  }
}

function writeDraft(key: string, value: string) {
  try {
    const text = value.trim() ? value : '';
    if (text) window.localStorage.setItem(key, value);
    else window.localStorage.removeItem(key);
  } catch {
    // Draft persistence is best-effort only.
  }
}

function clearDraft(key: string) {
  try {
    window.localStorage.removeItem(key);
  } catch {
    // Draft persistence is best-effort only.
  }
}
