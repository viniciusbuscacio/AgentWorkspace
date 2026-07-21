import { createHash } from 'node:crypto';
import { existsSync, readFileSync } from 'node:fs';
import path from 'node:path';
import { describe, expect, it } from 'vitest';

const EXPECTED_AGENT_WORKSPACE_ICON_HASHES = {
  // Source PNG copied from:
  // %USERPROFILE%\Desktop\New folder\Agent Workspace icon.png
  // Do not replace with the Wails "W" icon or a generated generic glasses icon.
  'build/appicon.png': '201871d807935426890511d84bc2a2c9292fca42cd3326e3ab3aa6f74236b983',

  // Windows ICOs generated from build/appicon.png. These two must stay identical.
  'build/windows/icon.ico': '7603632dd61c89b0ae152580f8d9bf3f330448f875e83512491801d0c3302e2e',
  'build/windows/agent-workspace-glasses-v2.ico': '7603632dd61c89b0ae152580f8d9bf3f330448f875e83512491801d0c3302e2e',
} as const;

function findProjectRoot(): string {
  let dir = process.cwd();
  for (;;) {
    if (existsSync(path.join(dir, 'wails.json')) && existsSync(path.join(dir, 'go.mod'))) {
      return dir;
    }
    const parent = path.dirname(dir);
    if (parent === dir) {
      throw new Error('Could not find Agent Workspace project root from test cwd.');
    }
    dir = parent;
  }
}

function sha256(filePath: string): string {
  return createHash('sha256').update(readFileSync(filePath)).digest('hex');
}

describe('Agent Workspace app icon guard', () => {
  it('fails the build if the app icon assets are not the approved glasses icon', () => {
    const root = findProjectRoot();

    for (const [relativePath, expectedHash] of Object.entries(EXPECTED_AGENT_WORKSPACE_ICON_HASHES)) {
      const absolutePath = path.join(root, relativePath);
      expect(existsSync(absolutePath), `${relativePath} must exist`).toBe(true);
      expect(sha256(absolutePath), `${relativePath} hash must match the approved Agent Workspace icon`).toBe(expectedHash);
    }
  });
});
