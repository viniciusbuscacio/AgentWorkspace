import { useEffect, useState } from 'react';
import { appInfoService } from '@services/appinfo.service';
import { openExternalUrl } from '@/app/external-links';
import { Card, CardContent, CardHeader, CardTitle } from '@ui/card';

const GITHUB_URL = 'https://github.com/viniciusbuscacio/AgentWorkspace';

export function AboutPage() {
  // "dev" is both the ldflags default and the fallback when the binding is
  // unavailable (tests, web mode) — a missing bridge never breaks the page.
  const [version, setVersion] = useState('dev');

  useEffect(() => {
    appInfoService.getVersion().then(setVersion).catch(() => setVersion('dev'));
  }, []);

  return (
    <Card className="max-w-xl rounded-lg">
      <CardHeader>
        <CardTitle>Agent Workspace</CardTitle>
      </CardHeader>
      <CardContent className="space-y-3 text-sm text-muted-foreground">
        <p>Version: {version}</p>
        <p>Frontend: React + TypeScript + Tailwind + Vite</p>
        <p>Backend: Go + Wails</p>
        <p>
          GitHub:{' '}
          <a
            href={GITHUB_URL}
            className="text-primary underline underline-offset-2 hover:opacity-80"
            onClick={(event) => { event.preventDefault(); openExternalUrl(GITHUB_URL); }}
          >
            viniciusbuscacio/AgentWorkspace
          </a>
        </p>
      </CardContent>
    </Card>
  );
}
