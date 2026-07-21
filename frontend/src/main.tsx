import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import './theme/globals.css';
import './theme/fonts.css';
import './theme/tokens.css';
import './theme/aw-chat.css';
import './theme/aw-sidebar.css';
import './theme/aw-home.css';
import './theme/aw-toast.css';
import { App } from './app/App';
import { applyAppTheme } from './lib/app-theme';
import { applyAppFont } from './lib/app-font';
import { installPipBindings, readPipConfig } from './pip/pip-bindings';
import { bootWebMode, readWebConfig } from './web/web-bindings';

applyAppTheme();
applyAppFont();

function renderApp() {
  const container = document.getElementById('root');
  if (!container) {
    throw new Error('root container #root not found');
  }
  createRoot(container).render(
    <StrictMode>
      <App />
    </StrictMode>,
  );
}

// Boot order: web mode (remote browser) → PiP process → desktop window.
const webConfig = readWebConfig();
const pipConfig = readPipConfig();
if (webConfig) {
  // Remote browser: the host injects window.aw_WEB. Bindings go through the
  // HTTP bridge + SSE; a login gate runs before React renders.
  await bootWebMode(webConfig, renderApp);
} else if (pipConfig) {
  // PiP process: bindings go through the host app's token-scoped localhost API.
  installPipBindings(pipConfig);
  renderApp();
} else {
  if (import.meta.env.DEV && !('go' in window)) {
    // DEV-only: when running in a plain browser (npm run dev) the Wails runtime
    // is absent. Install an in-memory mock so the UI boots for inspection/debug.
    const { installWailsMock } = await import('./dev/wails-mock');
    installWailsMock();
  }
  renderApp();
}
