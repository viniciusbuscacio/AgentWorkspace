import { cn } from '@/lib/utils';
// Monochrome brand marks from @lobehub/icons-static-svg (MIT), bundled at
// build time. Each SVG is 1em-sized with fill="currentColor", so the
// surrounding font-size and color drive it exactly like a material icon.
import openaiSvg from '@lobehub/icons-static-svg/icons/openai.svg?raw';
import githubCopilotSvg from '@lobehub/icons-static-svg/icons/githubcopilot.svg?raw';
import azureSvg from '@lobehub/icons-static-svg/icons/azure.svg?raw';
import openrouterSvg from '@lobehub/icons-static-svg/icons/openrouter.svg?raw';
import nvidiaSvg from '@lobehub/icons-static-svg/icons/nvidia.svg?raw';

// The embedded <title> would fight the cards' own tooltip (provider + model).
function stripSvgTitle(svg: string): string {
  return svg.replace(/<title>.*?<\/title>/, '');
}

const BRAND_SVGS: Record<string, string> = {
  'github-copilot': stripSvgTitle(githubCopilotSvg),
  openai: stripSvgTitle(openaiSvg),
  'openai-codex': stripSvgTitle(openaiSvg),
  'azure-openai': stripSvgTitle(azureSvg),
  openrouter: stripSvgTitle(openrouterSvg),
  nvidia: stripSvgTitle(nvidiaSvg),
};

// Generic material icons a custom provider can be assigned in its detail
// panel. The first entry is the default for customizable providers without a
// saved choice.
export const CUSTOM_PROVIDER_ICON_OPTIONS = [
  'memory',
  'smart_toy',
  'psychology',
  'bolt',
  'terminal',
  'api',
  'cloud',
  'dns',
  'rocket_launch',
  'router',
] as const;

// providerIsIconCustomizable marks providers whose icon the user picks (the
// dynamic custom slots plus the built-in "Custom OpenAI-compatible" one).
// Brand providers keep their official mark.
export function providerIsIconCustomizable(providerId: string): boolean {
  return providerId.startsWith('custom');
}

function fallbackMaterialIcon(providerId: string): string {
  if (providerId.includes('openai')) return 'key';
  if (providerId.includes('azure')) return 'cloud';
  if (providerId.includes('copilot')) return 'hub';
  if (providerId.includes('router')) return 'route';
  return 'memory';
}

// ProviderIcon renders a provider's official brand mark when one is bundled,
// otherwise a material icon — the user-picked one for custom providers, or a
// per-provider fallback. className controls size/color in both branches
// (font-size drives the 1em SVG).
export function ProviderIcon({
  providerId,
  customIcon,
  className,
}: {
  providerId: string;
  customIcon?: string;
  className?: string;
}) {
  const brand = BRAND_SVGS[providerId];
  if (brand) {
    return (
      <span
        className={cn('inline-flex', className)}
        aria-hidden="true"
        data-provider-brand-icon={providerId}
        // Trusted build-time asset from the bundled icon package, not
        // external content.
        dangerouslySetInnerHTML={{ __html: brand }}
      />
    );
  }
  const icon = (providerIsIconCustomizable(providerId) && customIcon) || fallbackMaterialIcon(providerId);
  return (
    <span className={cn('material-symbols-outlined', className)} aria-hidden="true">
      {icon}
    </span>
  );
}
