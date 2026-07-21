const cdpBase = process.env.aw_CDP_URL || 'http://127.0.0.1:9225';
const appURL = process.env.aw_URL || 'http://127.0.0.1:34115';
const password = process.env.aw_SMOKE_PASSWORD || '1234';

const wait = (ms) => new Promise((resolve) => setTimeout(resolve, ms));

async function getJSON(url, options = {}) {
  const response = await fetch(url, options);
  if (!response.ok) {
    throw new Error(`HTTP ${response.status} from ${url}`);
  }
  return response.json();
}

async function waitForHTTP(url, timeoutMs = 30000) {
  const deadline = Date.now() + timeoutMs;
  let lastError;
  while (Date.now() < deadline) {
    try {
      const response = await fetch(url);
      if (response.ok) return;
      lastError = new Error(`HTTP ${response.status}`);
    } catch (error) {
      lastError = error;
    }
    await wait(500);
  }
  throw new Error(`Timed out waiting for ${url}: ${lastError?.message || 'no response'}`);
}

async function connect(wsURL) {
  const ws = new WebSocket(wsURL);
  const pending = new Map();
  const events = [];
  let nextID = 1;

  ws.addEventListener('message', (event) => {
    const message = JSON.parse(event.data);
    if (message.id && pending.has(message.id)) {
      const { resolve, reject } = pending.get(message.id);
      pending.delete(message.id);
      if (message.error) reject(new Error(message.error.message || JSON.stringify(message.error)));
      else resolve(message.result || {});
      return;
    }
    events.push(message);
  });

  await new Promise((resolve, reject) => {
    ws.addEventListener('open', resolve, { once: true });
    ws.addEventListener('error', reject, { once: true });
  });

  return {
    events,
    send(method, params = {}) {
      const id = nextID++;
      ws.send(JSON.stringify({ id, method, params }));
      return new Promise((resolve, reject) => {
        pending.set(id, { resolve, reject });
      });
    },
    close() {
      ws.close();
    },
  };
}

async function evaluate(cdp, expression, timeoutMs = 10000) {
  const result = await cdp.send('Runtime.evaluate', {
    expression,
    awaitPromise: true,
    returnByValue: true,
    timeout: timeoutMs,
  });
  if (result.exceptionDetails) {
    throw new Error(result.exceptionDetails.text || 'Runtime.evaluate failed');
  }
  return result.result?.value;
}

async function waitFor(cdp, expression, timeoutMs = 10000) {
  const deadline = Date.now() + timeoutMs;
  let lastValue;
  while (Date.now() < deadline) {
    lastValue = await evaluate(cdp, expression, 5000);
    if (lastValue) return lastValue;
    await wait(250);
  }
  throw new Error(`Timed out waiting for expression: ${expression}. Last value: ${JSON.stringify(lastValue)}`);
}

async function main() {
  await waitForHTTP(appURL);
  await waitForHTTP(`${cdpBase}/json/version`);

  const targets = await getJSON(`${cdpBase}/json/list`);
  const target = targets.find((item) => (item.url || '').startsWith(appURL))
    || targets.find((item) => (item.url || '').startsWith('http://localhost:34115'))
    || await getJSON(`${cdpBase}/json/new?${encodeURIComponent(appURL)}`, { method: 'PUT' });
  const cdp = await connect(target.webSocketDebuggerUrl);
  const consoleErrors = [];

  await cdp.send('Runtime.enable');
  await cdp.send('Page.enable');

  cdp.events.push = new Proxy(cdp.events.push, {
    apply(targetPush, thisArg, args) {
      const [event] = args;
      if (event?.method === 'Runtime.exceptionThrown') {
        consoleErrors.push(event.params?.exceptionDetails?.text || 'Runtime exception');
      }
      if (event?.method === 'Runtime.consoleAPICalled' && event.params?.type === 'error') {
        consoleErrors.push((event.params.args || []).map((arg) => arg.value || arg.description || '').join(' '));
      }
      return Reflect.apply(targetPush, thisArg, args);
    },
  });

  await cdp.send('Page.navigate', { url: appURL });
  await waitFor(cdp, 'document.readyState === "complete"', 15000);
  await waitFor(cdp, 'document.querySelector("#app") && document.querySelector("#app").textContent.trim().length > 0', 15000);

  const gate = await evaluate(cdp, `(() => {
    const app = document.querySelector('#app');
    return {
      className: app?.className || '',
      text: (app?.textContent || '').replace(/\\s+/g, ' ').trim().slice(0, 500),
      profileRows: document.querySelectorAll('button.profile-row').length,
      unlockForms: document.querySelectorAll('form[data-auth-action="unlock-vault"]').length,
      chatComposers: document.querySelectorAll('.chat-composer').length,
    };
  })()`);

  if (gate.profileRows === 1 && gate.unlockForms === 0 && gate.chatComposers === 0) {
    await evaluate(cdp, `document.querySelector('button.profile-row').click()`);
    await waitFor(cdp, `!!document.querySelector('form[data-auth-action="unlock-vault"] input[type="password"]')`, 10000);
  }

  const needsUnlock = await evaluate(cdp, `!!document.querySelector('form[data-auth-action="unlock-vault"]')`);
  if (needsUnlock) {
    await evaluate(cdp, `(() => {
      const password = ${JSON.stringify(password)};
      const input = document.querySelector('form[data-auth-action="unlock-vault"] input[type="password"]');
      input.value = password;
      input.dispatchEvent(new Event('input', { bubbles: true }));
      document.querySelector('form[data-auth-action="unlock-vault"]').requestSubmit();
    })()`);
  }

  await waitFor(cdp, `!!document.querySelector('.chat-composer') && !!document.querySelector('[data-view="settings"]')`, 15000);

  await evaluate(cdp, `document.querySelector('[data-view="settings"]').click()`);
  await waitFor(cdp, `!!document.querySelector('form[data-settings-action="set-autolock"] select[name="minutes"]')`, 10000);

  const settings = await evaluate(cdp, `(() => {
    const select = document.querySelector('form[data-settings-action="set-autolock"] select[name="minutes"]');
    return {
      selectedAutoLock: select?.value || '',
      options: select ? [...select.options].map((option) => option.value) : [],
      hasProviderSettings: !!document.querySelector('.provider-settings'),
    };
  })()`);

  const requiredOptions = ['1', '5', '15', '30', '60'];
  for (const option of requiredOptions) {
    if (!settings.options.includes(option)) {
      throw new Error(`Missing auto-lock option ${option}`);
    }
  }

  if (consoleErrors.length) {
    throw new Error(`Console/runtime errors: ${consoleErrors.join(' | ')}`);
  }

  cdp.close();
  console.log(JSON.stringify({
    ok: true,
    appURL,
    cdpBase,
    initialGate: gate,
    settings,
  }, null, 2));
}

main().catch((error) => {
  console.error(error.stack || error.message || String(error));
  process.exitCode = 1;
});
