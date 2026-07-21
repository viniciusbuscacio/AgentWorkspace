import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { Composer } from './Composer';

function renderComposer(overrides: Partial<Parameters<typeof Composer>[0]> = {}) {
  const props = {
    sending: false,
    onSubmit: vi.fn(),
    onStop: vi.fn(async () => {}),
    onTranscribeAudio: vi.fn(async () => ''),
    onVoiceError: vi.fn(),
    height: 100,
    onResizeStart: vi.fn(),
    ...overrides,
  };
  const result = render(<Composer {...props} />);
  return { ...props, ...result };
}

beforeEach(() => {
  window.localStorage.clear();
});

describe('Composer drafts', () => {
  it('restores unsent text for the same chat draft key after unmount', async () => {
    const first = renderComposer({ draftKey: 'chat-1' });
    await userEvent.type(screen.getByRole('textbox'), 'mensagem em andamento');

    first.unmount();
    renderComposer({ draftKey: 'chat-1' });

    expect((screen.getByRole('textbox') as HTMLTextAreaElement).value).toBe('mensagem em andamento');
  });

  it('keeps drafts separated by chat id', async () => {
    const { rerender } = renderComposer({ draftKey: 'chat-1' });
    await userEvent.type(screen.getByRole('textbox'), 'draft chat 1');

    rerender(<Composer
      sending={false}
      onSubmit={vi.fn()}
      onStop={vi.fn(async () => {})}
      onTranscribeAudio={vi.fn(async () => '')}
      onVoiceError={vi.fn()}
      draftKey="chat-2"
      height={100}
      onResizeStart={vi.fn()}
    />);
    expect((screen.getByRole('textbox') as HTMLTextAreaElement).value).toBe('');

    await userEvent.type(screen.getByRole('textbox'), 'draft chat 2');
    rerender(<Composer
      sending={false}
      onSubmit={vi.fn()}
      onStop={vi.fn(async () => {})}
      onTranscribeAudio={vi.fn(async () => '')}
      onVoiceError={vi.fn()}
      draftKey="chat-1"
      height={100}
      onResizeStart={vi.fn()}
    />);
    expect((screen.getByRole('textbox') as HTMLTextAreaElement).value).toBe('draft chat 1');
  });

  it('clears the stored draft after submit', async () => {
    const onSubmit = vi.fn();
    renderComposer({ draftKey: 'chat-1', onSubmit });
    await userEvent.type(screen.getByRole('textbox'), 'send me');
    await userEvent.click(screen.getByRole('button', { name: 'Send' }));

    expect(onSubmit).toHaveBeenCalledWith('send me', []);
    expect(window.localStorage.getItem('aw.chatDraft:chat-1')).toBeNull();
  });
});

describe('Composer slash commands', () => {
  it('a fully typed command sends on the first Enter', async () => {
    const onSubmit = vi.fn();
    renderComposer({ draftKey: 'chat-1', onSubmit });

    await userEvent.type(screen.getByRole('textbox'), '/new{Enter}');

    expect(onSubmit).toHaveBeenCalledTimes(1);
    expect(onSubmit).toHaveBeenCalledWith('/new', []);
  });

  it('a partial draft completes on the first Enter and sends on the second', async () => {
    const onSubmit = vi.fn();
    renderComposer({ draftKey: 'chat-1', onSubmit });
    const textbox = screen.getByRole('textbox') as HTMLTextAreaElement;

    await userEvent.type(textbox, '/comp{Enter}');
    expect(onSubmit).not.toHaveBeenCalled();
    expect(textbox.value).toBe('/compact');

    await userEvent.type(textbox, '{Enter}');
    expect(onSubmit).toHaveBeenCalledWith('/compact', []);
  });

  it('Tab completes a fully typed command without sending', async () => {
    const onSubmit = vi.fn();
    renderComposer({ draftKey: 'chat-1', onSubmit });

    await userEvent.type(screen.getByRole('textbox'), '/new{Tab}');

    expect(onSubmit).not.toHaveBeenCalled();
    expect((screen.getByRole('textbox') as HTMLTextAreaElement).value).toBe('/new');
  });
});

describe('Composer merged send/stop button', () => {
  it('idle shows a disabled Send (nothing to send)', () => {
    renderComposer({ sending: false });
    const send = screen.getByRole('button', { name: 'Send' }) as HTMLButtonElement;
    expect(send.disabled).toBe(true);
    expect(screen.queryByRole('button', { name: 'Stop' })).toBeNull();
  });

  it('running with an empty composer shows Stop', () => {
    renderComposer({ sending: true });
    expect(screen.getByRole('button', { name: 'Stop' })).toBeTruthy();
    expect(screen.queryByRole('button', { name: /Send|Queue/ })).toBeNull();
  });

  it('typing while running flips Stop back to Send (queue)', async () => {
    renderComposer({ sending: true });
    expect(screen.getByRole('button', { name: 'Stop' })).toBeTruthy();
    await userEvent.type(screen.getByRole('textbox'), 'próxima');
    // Send is shown (labelled "Queue message" while a run is in flight).
    expect(screen.getByRole('button', { name: 'Queue message' })).toBeTruthy();
    expect(screen.queryByRole('button', { name: 'Stop' })).toBeNull();
  });

  it('clearing the draft while running flips Send back to Stop', async () => {
    renderComposer({ sending: true });
    await userEvent.type(screen.getByRole('textbox'), 'x');
    expect(screen.getByRole('button', { name: 'Queue message' })).toBeTruthy();
    await userEvent.clear(screen.getByRole('textbox'));
    expect(screen.getByRole('button', { name: 'Stop' })).toBeTruthy();
  });

  it('Esc stops the run even when a draft is typed (Send is showing)', async () => {
    const onStop = vi.fn(async () => {});
    renderComposer({ sending: true, onStop });
    const textbox = screen.getByRole('textbox');
    await userEvent.type(textbox, 'rascunho');
    expect(screen.getByRole('button', { name: 'Queue message' })).toBeTruthy();
    await userEvent.type(textbox, '{Escape}');
    expect(onStop).toHaveBeenCalledTimes(1);
  });
});

// jsdom has no navigator.mediaDevices, mirroring the packaged WKWebView where
// wails:// is not a secure context — the mic must use the native Go capture.
describe('Composer native voice fallback', () => {
  it('falls back to the native capture when getUserMedia exists but denies the request', async () => {
    // WKWebView shape: getUserMedia is present but rejects (Wails has no
    // media-permission handler), even with the macOS mic permission granted.
    const getUserMedia = vi.fn(async () => {
      throw new DOMException('The request is not allowed by the user agent or the platform in the current context, possibly because the user denied permission.', 'NotAllowedError');
    });
    Object.defineProperty(navigator, 'mediaDevices', {
      configurable: true,
      value: { getUserMedia },
    });
    try {
      const onStartNativeVoice = vi.fn(async () => {});
      const onStopNativeVoice = vi.fn(async () => 'ola mundo');
      renderComposer({ onStartNativeVoice, onStopNativeVoice });

      await userEvent.click(screen.getByRole('button', { name: 'Record audio' }));
      expect(getUserMedia).toHaveBeenCalledTimes(1);
      await waitFor(() => expect(onStartNativeVoice).toHaveBeenCalledTimes(1));
      expect(screen.getByTestId('voice-recording-panel')).toBeTruthy();
    } finally {
      Reflect.deleteProperty(navigator as object, 'mediaDevices');
    }
  });

  it('records and transcribes through the native capture when getUserMedia is missing', async () => {
    const onStartNativeVoice = vi.fn(async () => {});
    const onStopNativeVoice = vi.fn(async () => 'ola mundo');
    renderComposer({ onStartNativeVoice, onStopNativeVoice });

    await userEvent.click(screen.getByRole('button', { name: 'Record audio' }));
    expect(onStartNativeVoice).toHaveBeenCalledTimes(1);

    // Recording feedback replaces the textarea: REC badge, waveform, timer.
    expect(screen.getByTestId('voice-rec-badge')).toBeTruthy();
    expect(screen.getByTestId('voice-recording-panel')).toBeTruthy();
    expect(screen.getByLabelText('Voice recording waveform')).toBeTruthy();
    expect(screen.queryByRole('textbox')).toBeNull();

    await userEvent.click(screen.getByRole('button', { name: 'Stop voice input' }));
    expect(onStopNativeVoice).toHaveBeenCalledTimes(1);
    await waitFor(() => expect((screen.getByRole('textbox') as HTMLTextAreaElement).value).toBe('ola mundo'));
    expect(screen.queryByTestId('voice-recording-panel')).toBeNull();
  });

  it('shows the transcribing panel while waiting for the transcript', async () => {
    let finish: (value: string) => void = () => {};
    const onStopNativeVoice = vi.fn(() => new Promise<string>((resolve) => { finish = resolve; }));
    renderComposer({ onStartNativeVoice: vi.fn(async () => {}), onStopNativeVoice });

    await userEvent.click(screen.getByRole('button', { name: 'Record audio' }));
    await userEvent.click(screen.getByRole('button', { name: 'Stop voice input' }));

    expect(screen.getByTestId('voice-processing-panel')).toBeTruthy();
    expect(screen.getByText('Transcribing…')).toBeTruthy();

    finish('pronto');
    await waitFor(() => expect((screen.getByRole('textbox') as HTMLTextAreaElement).value).toBe('pronto'));
  });

  it('send during recording stops, transcribes and submits in one click', async () => {
    const onSubmit = vi.fn();
    const onStopNativeVoice = vi.fn(async () => 'mensagem por voz');
    renderComposer({
      onSubmit,
      onStartNativeVoice: vi.fn(async () => {}),
      onStopNativeVoice,
    });

    await userEvent.click(screen.getByRole('button', { name: 'Record audio' }));
    const send = screen.getByRole('button', { name: 'Send' }) as HTMLButtonElement;
    expect(send.disabled).toBe(false);

    await userEvent.click(send);
    expect(onStopNativeVoice).toHaveBeenCalledTimes(1);
    await waitFor(() => expect(onSubmit).toHaveBeenCalledWith('mensagem por voz', []));
    // Auto-send leaves a clean composer instead of a drafted transcript.
    await waitFor(() => expect((screen.getByRole('textbox') as HTMLTextAreaElement).value).toBe(''));
  });

  it('surfaces native capture start failures', async () => {
    const onVoiceError = vi.fn();
    renderComposer({
      onStartNativeVoice: vi.fn(async () => { throw new Error('mic denied'); }),
      onStopNativeVoice: vi.fn(async () => ''),
      onVoiceError,
    });

    await userEvent.click(screen.getByRole('button', { name: 'Record audio' }));
    await waitFor(() => expect(onVoiceError).toHaveBeenCalledWith('mic denied'));
  });

  it('reports unavailability when no native fallback is wired', async () => {
    const onVoiceError = vi.fn();
    renderComposer({ onVoiceError });

    await userEvent.click(screen.getByRole('button', { name: 'Record audio' }));
    expect(onVoiceError).toHaveBeenCalledWith('Microphone recording is not available in this WebView');
  });
});
