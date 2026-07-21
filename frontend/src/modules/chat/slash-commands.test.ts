import { describe, expect, it } from 'vitest';
import {
  filterSlashCommands,
  isSlashDraft,
  providerToken,
  resolveModelArg,
  slashSuggestions,
  SLASH_COMMANDS,
  type ModelProviderHint,
} from './slash-commands';

const PROVIDERS: ModelProviderHint[] = [
  { id: 'openai', name: 'OpenAI', model: 'gpt-5.5', connected: true, models: ['gpt-5.5', 'gpt-4.1'] },
  { id: 'anthropic', name: 'Anthropic', model: 'claude-sonnet-5', connected: true, models: ['claude-sonnet-5', 'claude-opus-4-8'] },
  // A custom provider: opaque id, multi-word name, and (typically) no model list.
  { id: 'custom-3sd32491', name: 'Maritaca IA', model: 'sabia-3', connected: true, models: [] },
  { id: 'offline', name: 'Offline', model: '', connected: false, models: [] },
];

describe('slash commands', () => {
  it('isSlashDraft detects single-line slash drafts', () => {
    expect(isSlashDraft('/comp')).toBe(true);
    expect(isSlashDraft('hello')).toBe(false);
    expect(isSlashDraft('/new\nmore')).toBe(false);
  });

  it('filters by prefix, returns all for "/"', () => {
    expect(filterSlashCommands('/')).toHaveLength(SLASH_COMMANDS.length);
    expect(filterSlashCommands('/comp').map((c) => c.name)).toEqual(['/compact']);
    expect(filterSlashCommands('/zzz')).toHaveLength(0);
  });

  it('includes the per-chat /model command', () => {
    expect(SLASH_COMMANDS.map((c) => c.name)).toContain('/model');
    expect(filterSlashCommands('/mod').map((c) => c.name)).toEqual(['/model']);
  });
});

describe('slashSuggestions for /model', () => {
  it('falls back to the static command list for non-/model drafts', () => {
    expect(slashSuggestions('/comp', PROVIDERS).map((c) => c.name)).toEqual(['/compact']);
  });

  it('suggests connected providers as hyphen slugs plus keywords once "/model " is typed', () => {
    const names = slashSuggestions('/model ', PROVIDERS).map((c) => c.name);
    expect(names).toContain('/model list');
    expect(names).toContain('/model default');
    expect(names).toContain('/model OpenAI');
    expect(names).toContain('/model Anthropic');
    // A custom provider is offered by its slugged name, never its raw id.
    expect(names).toContain('/model Maritaca-IA');
    expect(names).not.toContain('/model custom-3sd32491');
    // Disconnected providers are not offered.
    expect(names).not.toContain('/model Offline');
  });

  it('filters provider suggestions by the partial token', () => {
    const names = slashSuggestions('/model anth', PROVIDERS).map((c) => c.name);
    expect(names).toEqual(['/model Anthropic']);
  });

  it('matches a custom provider slug while still typing it', () => {
    expect(slashSuggestions('/model Maritaca', PROVIDERS).map((c) => c.name)).toEqual(['/model Maritaca-IA']);
    expect(slashSuggestions('/model Maritaca-I', PROVIDERS).map((c) => c.name)).toEqual(['/model Maritaca-IA']);
    // The raw id fragment still resolves too.
    expect(slashSuggestions('/model 3sd', PROVIDERS).map((c) => c.name)).toEqual(['/model Maritaca-IA']);
  });

  it('suggests the provider models after a provider slug and a space', () => {
    const names = slashSuggestions('/model OpenAI ', PROVIDERS).map((c) => c.name);
    expect(names).toEqual(['/model OpenAI gpt-5.5', '/model OpenAI gpt-4.1']);
  });

  it('offers the configured model for a custom provider with no model list', () => {
    const names = slashSuggestions('/model Maritaca-IA ', PROVIDERS).map((c) => c.name);
    expect(names).toEqual(['/model Maritaca-IA sabia-3']);
  });

  it('filters model suggestions by the partial model token', () => {
    const names = slashSuggestions('/model OpenAI gpt-4', PROVIDERS).map((c) => c.name);
    expect(names).toEqual(['/model OpenAI gpt-4.1']);
  });
});

describe('providerToken / resolveModelArg', () => {
  it('slugs whitespace in a display name', () => {
    expect(providerToken('Maritaca IA')).toBe('Maritaca-IA');
    expect(providerToken('OpenAI')).toBe('OpenAI');
  });

  it('resolves a slug token to the provider id and leaves the rest as the model', () => {
    expect(resolveModelArg('Maritaca-IA sabia-3', PROVIDERS)).toEqual({ providerId: 'custom-3sd32491', model: 'sabia-3' });
    expect(resolveModelArg('OpenAI', PROVIDERS)).toEqual({ providerId: 'openai', model: '' });
  });

  it('still resolves a hand-typed spaced name', () => {
    expect(resolveModelArg('Maritaca IA sabia-3', PROVIDERS)).toEqual({ providerId: 'custom-3sd32491', model: 'sabia-3' });
  });

  it('returns null for an unknown provider', () => {
    expect(resolveModelArg('nope', PROVIDERS)).toBeNull();
  });
});
