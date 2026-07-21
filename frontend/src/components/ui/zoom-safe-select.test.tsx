import { describe, it, expect, vi } from 'vitest';
import { fireEvent, render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';

import { ZoomSafeSelect } from './zoom-safe-select';

const options = [
  { value: '1', label: 'One' },
  { value: '2', label: 'Two' },
  { value: '3', label: 'Three' },
];

describe('ZoomSafeSelect', () => {
  it('shows the selected option in the trigger', () => {
    render(<ZoomSafeSelect aria-label="Pick" value="2" onValueChange={() => {}} options={options} />);
    expect(screen.getByRole('combobox', { name: 'Pick' }).textContent).toContain('Two');
  });

  it('opens on click and selects an option', async () => {
    const onChange = vi.fn();
    render(<ZoomSafeSelect aria-label="Pick" value="1" onValueChange={onChange} options={options} />);

    await userEvent.click(screen.getByRole('combobox', { name: 'Pick' }));
    expect(screen.getByRole('listbox')).toBeTruthy();

    await userEvent.click(screen.getByRole('option', { name: 'Three' }));
    expect(onChange).toHaveBeenCalledWith('3');
    // Panel closes after selection.
    expect(screen.queryByRole('listbox')).toBeNull();
  });

  it('marks the current value as selected', async () => {
    render(<ZoomSafeSelect aria-label="Pick" value="2" onValueChange={() => {}} options={options} />);
    await userEvent.click(screen.getByRole('combobox', { name: 'Pick' }));
    expect(screen.getByRole('option', { name: 'Two' }).getAttribute('aria-selected')).toBe('true');
    expect(screen.getByRole('option', { name: 'One' }).getAttribute('aria-selected')).toBe('false');
  });

  it('supports keyboard: arrow down + Enter selects the next option', async () => {
    const onChange = vi.fn();
    render(<ZoomSafeSelect aria-label="Pick" value="1" onValueChange={onChange} options={options} />);

    const trigger = screen.getByRole('combobox', { name: 'Pick' });
    trigger.focus();
    await userEvent.keyboard('{ArrowDown}'); // opens, active = selected (index 0)
    await userEvent.keyboard('{ArrowDown}'); // active = index 1
    await userEvent.keyboard('{Enter}');
    expect(onChange).toHaveBeenCalledWith('2');
  });

  it('stays open when its own list scrolls, closes when the page scrolls', async () => {
    render(<ZoomSafeSelect aria-label="Pick" value="1" onValueChange={() => {}} options={options} />);
    await userEvent.click(screen.getByRole('combobox', { name: 'Pick' }));
    const listbox = screen.getByRole('listbox');

    // Wheel-scrolling inside the dropdown fires a scroll event on the panel
    // itself; that must not close it.
    fireEvent.scroll(listbox);
    expect(screen.queryByRole('listbox')).toBeTruthy();

    // A scroll anywhere else (the page behind) detaches the panel from the
    // trigger, so it closes.
    fireEvent.scroll(document.body);
    expect(screen.queryByRole('listbox')).toBeNull();
  });

  it('closes on Escape without selecting', async () => {
    const onChange = vi.fn();
    render(<ZoomSafeSelect aria-label="Pick" value="1" onValueChange={onChange} options={options} />);
    await userEvent.click(screen.getByRole('combobox', { name: 'Pick' }));
    await userEvent.keyboard('{Escape}');
    expect(screen.queryByRole('listbox')).toBeNull();
    expect(onChange).not.toHaveBeenCalled();
  });
});
