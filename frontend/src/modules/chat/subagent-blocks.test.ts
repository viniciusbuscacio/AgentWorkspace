import { describe, expect, it } from 'vitest';
import { parseSubagentBlocks, splitAtSpawnMarkers } from './subagent-blocks';

describe('parseSubagentBlocks', () => {
  it('splits assistant markdown around subagent directives', () => {
    const parts = parseSubagentBlocks(`Before

::subagent{task="Read API" status="completed" result="Found endpoint"}
Prompt:
check api

Return:
Status: done
::end-subagent

After`);

    expect(parts).toHaveLength(3);
    expect(parts[0]).toEqual({ type: 'markdown', markdown: 'Before' });
    expect(parts[1]).toMatchObject({
      type: 'subagent',
      block: {
        task: 'Read API',
        status: 'completed',
        result: 'Found endpoint',
      },
    });
    expect(parts[2]).toEqual({ type: 'markdown', markdown: 'After' });
  });

  it('keeps incomplete directives as markdown', () => {
    const source = '::subagent{task="Open" status="running"}\nPrompt: still streaming';
    expect(parseSubagentBlocks(source)).toEqual([{ type: 'markdown', markdown: source }]);
  });

  it('splits at ::spawn markers into spawn parts', () => {
    const parts = parseSubagentBlocks('Vou disparar os agentes:\n\n::spawn{run="run-1"}\n\nPronto, o resultado foi X.');
    expect(parts).toEqual([
      { type: 'markdown', markdown: 'Vou disparar os agentes:' },
      { type: 'spawn', runId: 'run-1' },
      { type: 'markdown', markdown: 'Pronto, o resultado foi X.' },
    ]);
  });

  it('keeps a malformed spawn marker as markdown', () => {
    const source = 'texto\n::spawn{run=""}\nmais texto';
    expect(parseSubagentBlocks(source)).toEqual([{ type: 'markdown', markdown: source }]);
  });
});

describe('splitAtSpawnMarkers', () => {
  it('splits text around the marker into text/spawn segments', () => {
    expect(splitAtSpawnMarkers('Antes.\n\n::spawn{run="run-1"}\n\nDepois.')).toEqual([
      { type: 'text', text: 'Antes.' },
      { type: 'spawn', runId: 'run-1' },
      { type: 'text', text: 'Depois.' },
    ]);
  });

  it('handles a trailing marker while the reply is still streaming', () => {
    expect(splitAtSpawnMarkers('Disparando:\n\n::spawn{run="run-1"}\n\n')).toEqual([
      { type: 'text', text: 'Disparando:' },
      { type: 'spawn', runId: 'run-1' },
    ]);
  });

  it('returns plain text untouched', () => {
    expect(splitAtSpawnMarkers('sem marcador')).toEqual([{ type: 'text', text: 'sem marcador' }]);
  });

  it('parses a persisted results block into the spawn segment tasks', () => {
    const block = '::spawn{run="run-1"}\n{"tasks":[{"id":"t1","task":"subagent 1","status":"success","output":"ok","elapsedMs":10}]}\n::end-spawn';
    expect(splitAtSpawnMarkers(`Antes.\n\n${block}\n\nDepois.`)).toEqual([
      { type: 'text', text: 'Antes.' },
      { type: 'spawn', runId: 'run-1', tasks: [{ id: 't1', task: 'subagent 1', status: 'success', output: 'ok', elapsedMs: 10 }] },
      { type: 'text', text: 'Depois.' },
    ]);
  });

  it('does not let a bare marker swallow the next run body', () => {
    const source = '::spawn{run="run-1"}\nmeio\n::spawn{run="run-2"}\n{"tasks":[{"id":"t1","task":"x","status":"success"}]}\n::end-spawn';
    const segments = splitAtSpawnMarkers(source);
    expect(segments).toEqual([
      { type: 'spawn', runId: 'run-1' },
      { type: 'text', text: 'meio' },
      { type: 'spawn', runId: 'run-2', tasks: [{ id: 't1', task: 'x', status: 'success' }] },
    ]);
  });
});
