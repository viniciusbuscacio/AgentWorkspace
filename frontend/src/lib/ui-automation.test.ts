import { afterEach, describe, expect, it, vi } from 'vitest';
import { executeUiCommand } from './ui-automation';

afterEach(() => {
  document.body.innerHTML = '';
});

async function snapshot(max?: number) {
  return (await executeUiCommand('snapshot', max ? { max } : {})) as { tree: string; nodes: number };
}

describe('ui.snapshot accessibility tree', () => {
  it('emits a pruned tree with roles, names and refs, skipping layout wrappers', async () => {
    document.body.innerHTML = `
      <main>
        <h1>Settings</h1>
        <div class="layout-wrapper">
          <div data-slot="card">
            <div data-slot="card-title">MCP server</div>
            <button data-awid="mcp-server-toggle">Start</button>
          </div>
        </div>
      </main>
    `;
    const { tree } = await snapshot();
    const lines = tree.split('\n');
    // main and heading are kept; the layout-wrapper div is collapsed.
    expect(lines.some((l) => l.startsWith('main '))).toBe(true);
    expect(tree).toContain('heading "Settings" [e2] level=1');
    expect(tree).toContain('group "MCP server"');
    expect(tree).toContain('button "Start"');
    expect(tree).not.toContain('layout-wrapper');
    // The card (group) is indented under main, the button under the group.
    const groupLine = lines.find((l) => l.includes('group "MCP server"'))!;
    const buttonLine = lines.find((l) => l.includes('button "Start"'))!;
    expect(buttonLine.indexOf('button')).toBeGreaterThan(groupLine.indexOf('group'));
  });

  it('reports control state and resolves field labels and values', async () => {
    document.body.innerHTML = `
      <div data-slot="field" role="group">
        <label data-slot="field-label">Port</label>
        <input type="number" value="9300" />
      </div>
      <button role="switch" aria-checked="true" aria-label="Auto-start">on</button>
      <button disabled aria-label="Save">Save</button>
      <input type="password" aria-label="Token" value="supersecret" />
    `;
    const { tree } = await snapshot();
    expect(tree).toContain('spinbutton "Port" [e2] = "9300"');
    expect(tree).toContain('switch "Auto-start" [e3] checked');
    expect(tree).toContain('button "Save" [e4] disabled');
    // Secrets are masked, never echoed.
    expect(tree).toContain('"Token"');
    expect(tree).not.toContain('supersecret');
  });

  it('masks controls marked data-sensitive even when they are visible text fields', async () => {
    document.body.innerHTML = `<input type="text" aria-label="Revealed password" value="visible-secret" data-sensitive />`;

    const { tree } = await snapshot();

    expect(tree).toContain('textbox "Revealed password" [e1] = "••••••"');
    expect(tree).not.toContain('visible-secret');
  });

  it('excludes aria-hidden icon glyphs from the accessible name', async () => {
    document.body.innerHTML = `
      <button><span class="material-symbols-outlined" aria-hidden="true">content_copy</span>Copy</button>
      <button><span aria-hidden="true">save</span>Save</button>
    `;
    const { tree } = await snapshot();
    expect(tree).toContain('button "Copy"');
    expect(tree).toContain('button "Save"');
    expect(tree).not.toContain('content_copy');
    expect(tree).not.toContain('saveSave');
  });

  it('caps the number of nodes at max', async () => {
    document.body.innerHTML = Array.from({ length: 20 }, (_, i) => `<button>B${i}</button>`).join('');
    const { nodes } = await snapshot(5);
    expect(nodes).toBeLessThanOrEqual(5);
  });

  it('filters transparent, inert and explicitly zero-size elements', async () => {
    document.body.innerHTML = `
      <button style="opacity:0">Transparent</button>
      <div inert><button>Inert</button></div>
      <button id="zero" style="width:0;height:0">Zero</button>
      <button>Visible</button>
    `;
    vi.spyOn(document.querySelector('#zero') as HTMLElement, 'getClientRects').mockReturnValue({ length: 0, item: () => null } as unknown as DOMRectList);

    const { tree } = await snapshot();

    expect(tree).toContain('button "Visible"');
    expect(tree).not.toContain('Transparent');
    expect(tree).not.toContain('Inert');
    expect(tree).not.toContain('Zero');
  });
});

describe('ui.click / ui.fill by ref', () => {
  it('clicks the element behind a ref from the latest snapshot', async () => {
    document.body.innerHTML = `<button data-awid="go">Go</button>`;
    let clicked = false;
    document.querySelector('[data-awid="go"]')!.addEventListener('click', () => {
      clicked = true;
    });
    const { tree } = await snapshot();
    const ref = tree.match(/\[(e\d+)\]/)![1];
    const result = (await executeUiCommand('click', { ref })) as { clicked: boolean };
    expect(result.clicked).toBe(true);
    expect(clicked).toBe(true);
  });

  it('fills an input by ref and dispatches input/change', async () => {
    document.body.innerHTML = `<input aria-label="Port" value="9300" />`;
    const el = document.querySelector('input') as HTMLInputElement;
    const events: string[] = [];
    el.addEventListener('input', () => events.push('input'));
    el.addEventListener('change', () => events.push('change'));
    const { tree } = await snapshot();
    const ref = tree.match(/\[(e\d+)\]/)![1];
    const result = (await executeUiCommand('fill', { ref, value: '9305' })) as { value: string };
    expect(result.value).toBe('9305');
    expect(el.value).toBe('9305');
    expect(events).toEqual(['input', 'change']);
  });

  it('still supports a CSS selector as a fallback', async () => {
    document.body.innerHTML = `<button data-awid="go">Go</button>`;
    const result = (await executeUiCommand('click', { selector: '[data-awid="go"]' })) as { clicked: boolean };
    expect(result.clicked).toBe(true);
  });

  it('errors on an unknown ref, missing target, unfillable element and unknown command', async () => {
    document.body.innerHTML = `<div id="box">x</div>`;
    await expect(executeUiCommand('click', { ref: 'e999' })).rejects.toThrow(/unknown ref/);
    await expect(executeUiCommand('click', { selector: '#nope' })).rejects.toThrow(/no element/);
    await expect(executeUiCommand('fill', { selector: '#box', value: 'x' })).rejects.toThrow(/not fillable/);
    await expect(executeUiCommand('click', {})).rejects.toThrow(/ref or selector is required/);
    await expect(executeUiCommand('teleport', {})).rejects.toThrow(/unknown ui command/);
  });
});
