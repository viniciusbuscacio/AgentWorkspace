import { cn } from '@/lib/utils';
// Monochrome brand marks from @lobehub/icons-static-svg (MIT), bundled at
// build time — same pattern as settings/pages/provider-icon.tsx. Each SVG is
// 1em-sized with fill="currentColor", so the surrounding font-size and color
// drive it exactly like a material icon.
import mcpSvg from '@lobehub/icons-static-svg/icons/mcp.svg?raw';

function stripSvgTitle(svg: string): string {
  return svg.replace(/<title>.*?<\/title>/, '');
}

// Module/service icon tokens that resolve to a bundled brand SVG instead of a
// material-symbol ligature (module.go Icon fields may carry these).
const BRAND_SVGS: Record<string, string> = {
  mcp: stripSvgTitle(mcpSvg),
};

// AwIcon renders an icon token from the module catalog: a bundled brand mark
// when one exists for the token, otherwise the material-symbol ligature.
// className controls size/color in both branches (font-size drives the 1em
// SVG), mirroring ProviderIcon.
export function AwIcon({ icon, className }: { icon: string; className?: string }) {
  const brand = BRAND_SVGS[icon];
  if (brand) {
    return (
      <span
        className={cn('aw-brand-icon inline-flex items-center justify-center', className)}
        aria-hidden="true"
        data-brand-icon={icon}
        // Trusted build-time asset from the bundled icon package, not
        // external content.
        dangerouslySetInnerHTML={{ __html: brand }}
      />
    );
  }
  return (
    <span className={cn('material-symbols-outlined', className)} aria-hidden="true">
      {icon}
    </span>
  );
}
