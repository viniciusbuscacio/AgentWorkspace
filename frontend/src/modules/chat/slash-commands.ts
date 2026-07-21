export interface SlashCommand {
  name: string;
  description: string;
}

// Ported from the vanilla AW2 frontend (SLASH_COMMANDS).
export const SLASH_COMMANDS: SlashCommand[] = [
  { name: '/new', description: 'Start a new session' },
  { name: '/compact', description: 'Compact chat context' },
  { name: '/session-info', description: 'Show provider, model, and session details' },
  { name: '/model', description: 'Set the provider/model for this chat only' },
  { name: '/plan', description: 'Toggle plan mode preference' },
  { name: '/clear', description: 'Clear all messages in this chat' },
  { name: '/debug', description: 'Open the prompt/LLM debug panel' },
  { name: '/help', description: 'Show available commands' },
];

export function filterSlashCommands(value: string): SlashCommand[] {
  const trimmed = value.trim();
  if (!trimmed || trimmed === '/') return SLASH_COMMANDS;
  return SLASH_COMMANDS.filter((command) => command.name.startsWith(trimmed));
}

/** True when the draft is a slash command line (and not a multi-line message). */
export function isSlashDraft(value: string): boolean {
  return value.startsWith('/') && !value.includes('\n');
}

// ModelProviderHint feeds the /model auto-complete: one connected provider with
// the model it is configured to use and its selectable model options.
export interface ModelProviderHint {
  id: string;
  name: string;
  model: string;
  connected: boolean;
  models: string[];
  /** Provider is benched by the failover breaker (recent failures). */
  benched?: boolean;
}

const MODEL_KEYWORDS: SlashCommand[] = [
  { name: '/model list', description: 'Show every provider and its models' },
  { name: '/model default', description: 'Revert this chat to the global provider' },
];

// providerToken slugs a provider's display name into a single command token by
// replacing runs of whitespace with a hyphen ("Maritaca IA" -> "Maritaca-IA"),
// so /model always takes exactly two space-separated words (provider, model)
// and never a raw custom-provider id.
export function providerToken(name: string): string {
  return name.trim().replace(/\s+/g, '-');
}

// providerFromToken resolves a slug token back to a provider, matching the
// slugged name or the raw id (both case-insensitive).
export function providerFromToken(token: string, providers: ModelProviderHint[]): ModelProviderHint | null {
  const t = token.trim().toLowerCase();
  if (!t) return null;
  return providers.find(
    (provider) => providerToken(provider.name).toLowerCase() === t || provider.id.toLowerCase() === t,
  ) ?? null;
}

// providerModelOptions is the model list offered for a provider. Custom
// providers usually carry no predefined model list, so the currently-configured
// model is folded in — otherwise the model auto-complete would be empty.
function providerModelOptions(provider: ModelProviderHint): string[] {
  const options = provider.model ? [provider.model, ...provider.models] : [...provider.models];
  return Array.from(new Set(options.filter(Boolean)));
}

// resolveModelArg parses the "<provider> [model]" argument of /model. It first
// treats the first word as a slug token (the form the auto-complete emits), then
// falls back to a longest name match so a hand-typed spaced name still resolves.
export interface ResolvedModelArg {
  providerId: string;
  model: string;
}

export function resolveModelArg(args: string, providers: ModelProviderHint[]): ResolvedModelArg | null {
  const trimmed = args.trim();
  if (!trimmed) return null;
  const parts = trimmed.split(/\s+/);
  const bySlug = providerFromToken(parts[0], providers);
  if (bySlug) return { providerId: bySlug.id, model: parts.slice(1).join(' ') };

  const lower = trimmed.toLowerCase();
  let best: (ResolvedModelArg & { len: number }) | null = null;
  for (const provider of providers) {
    const name = provider.name.trim();
    const k = name.toLowerCase();
    if (!k) continue;
    if (lower === k || lower.startsWith(`${k} `)) {
      const candidate = { providerId: provider.id, model: trimmed.slice(name.length).trim(), len: k.length };
      if (!best || candidate.len > best.len) best = candidate;
    }
  }
  return best ? { providerId: best.providerId, model: best.model } : null;
}

// slashSuggestions returns the auto-complete menu for the current draft. For a
// plain slash draft it is the static command list; once the user has typed
// "/model " it becomes provider suggestions (shown as a hyphen-slugged name),
// and after a provider is chosen it becomes that provider's model list — so the
// user never has to guess what to type, and never sees a raw custom id.
export function slashSuggestions(value: string, providers: ModelProviderHint[]): SlashCommand[] {
  const draft = value.replace(/^\s+/, '');
  if (!draft.toLowerCase().startsWith('/model ')) {
    return filterSlashCommands(value);
  }
  const rest = draft.slice('/model '.length);
  const firstSpace = rest.indexOf(' ');

  // Provider slug is fully typed (a space follows it): suggest its models.
  if (firstSpace !== -1) {
    const provider = providerFromToken(rest.slice(0, firstSpace), providers);
    if (!provider) return [];
    const modelPartial = rest.slice(firstSpace + 1).toLowerCase();
    const token = providerToken(provider.name);
    return providerModelOptions(provider)
      .filter((model) => model.toLowerCase().includes(modelPartial))
      .map((model) => ({ name: `/model ${token} ${model}`, description: provider.name }));
  }

  // Still choosing the provider (or a keyword like list/default).
  const partial = rest.toLowerCase();
  const spaced = partial.replace(/-/g, ' ');
  const items: SlashCommand[] = [];
  for (const keyword of MODEL_KEYWORDS) {
    const token = keyword.name.slice('/model '.length);
    if (token.startsWith(partial)) items.push(keyword);
  }
  for (const provider of providers) {
    if (!provider.connected) continue;
    const token = providerToken(provider.name).toLowerCase();
    if (token.includes(partial) || provider.id.toLowerCase().includes(partial) || provider.name.toLowerCase().includes(spaced)) {
      items.push({
        name: `/model ${providerToken(provider.name)}`,
        description: provider.model || 'default model',
      });
    }
  }
  return items;
}
