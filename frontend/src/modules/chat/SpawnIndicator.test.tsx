import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it } from 'vitest';
import { SpawnIndicator } from './SpawnIndicator';
import type { SubagentTask } from './subagent-tasks';

const tasks: SubagentTask[] = [
  { id: 't1', task: 'subagent 1', status: 'success', output: 'subagent 1 ok' },
  { id: 't2', task: 'subagent 2', status: 'running' },
];

describe('SpawnIndicator', () => {
  it('starts collapsed: shows the summary row but not the tasks until clicked', async () => {
    render(<SpawnIndicator tasks={tasks} />);
    expect(screen.getByText('Running 2 subagents... (1/2)')).toBeTruthy();
    expect(screen.queryByText('subagent 1')).toBeNull();
    expect(screen.queryByText('subagent 1 ok')).toBeNull();

    await userEvent.click(screen.getByRole('button', { name: /Running 2 subagents/ }));
    expect(screen.getByText('subagent 1')).toBeTruthy();
    expect(screen.getByText('subagent 1 ok')).toBeTruthy();

    // Collapses again.
    await userEvent.click(screen.getByRole('button', { name: /Running 2 subagents/ }));
    expect(screen.queryByText('subagent 1')).toBeNull();
  });

  it('switches the summary to completed once every task finished', () => {
    render(<SpawnIndicator tasks={tasks.map((task) => ({ ...task, status: 'success' as const }))} />);
    expect(screen.getByText('2 subagents completed')).toBeTruthy();
  });

  it('renders nothing without tasks', () => {
    const { container } = render(<SpawnIndicator tasks={[]} />);
    expect(container.firstChild).toBeNull();
  });

  it('expanded shows the FULL task text, never a truncated single line', async () => {
    const longTask = 'On this Windows machine, install the Google Cloud SDK. '.repeat(8).trim();
    render(<SpawnIndicator tasks={[{ id: 't1', task: longTask, status: 'running' }]} />);
    await userEvent.click(screen.getByRole('button', { name: /Running 1 subagent/ }));

    const taskText = screen.getByText(longTask);
    expect(taskText.className).not.toContain('truncate');
    expect(taskText.className).toContain('whitespace-pre-wrap');
  });
});
