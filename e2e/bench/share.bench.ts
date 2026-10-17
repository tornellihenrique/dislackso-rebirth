import { mkdirSync, writeFileSync } from 'node:fs';

import {
  chromium,
  expect,
  test,
  type Browser,
  type BrowserContext,
  type Page,
} from '@playwright/test';

import { SETUP_TOKEN } from '../tests/fixtures';
import { installFakeScreen, type Scene } from './fake-screen';
import { createBench, type Account, type Bench } from './instance';
import {
  received,
  sent,
  snapshot,
  sourceFps,
  treeCpu,
  type InStream,
  type OutLayer,
} from './measure';

// Screen share measurements for ARCHITECTURE §9.5 and docs/adr/0002. Each
// role runs in a browser of its own, so its CPU can be told apart.

// Long enough for the bandwidth estimate to ramp up.
const WARMUP_MS = 15_000;
const WINDOW_MS = 15_000;
const CODECS = ['vp9', 'vp8', 'h264', 'av1'] as const;
const args = ['--use-fake-ui-for-media-stream', '--use-fake-device-for-media-stream'];
// Full Chromium with the GPU, like the browsers people share from: canvas
// drawing and (where the driver offers them) hardware encoders.
const gpu =
  process.platform === 'win32'
    ? ['--enable-gpu', '--ignore-gpu-blocklist', '--use-angle=d3d11']
    : [];

let base: URL;
let bench: Bench;
const report: string[] = [];

test.describe.configure({ mode: 'serial' });

test.beforeAll(async ({ baseURL }) => {
  base = new URL(baseURL ?? '');
  bench = await createBench(base, SETUP_TOKEN, 7);
});

test.afterAll(() => {
  const dir = new URL('../bench-results/', import.meta.url);
  mkdirSync(dir, { recursive: true });
  const stamp = new Date().toISOString().slice(0, 16).replace(/[:T]/g, '-');
  writeFileSync(new URL(`share-${stamp}.md`, dir), report.join('\n') + '\n');
  console.log(report.join('\n'));
});

function account(i: number): Account {
  const a = bench.accounts[i];
  if (!a) throw new Error(`no bench account ${String(i)}`);
  return a;
}

interface Joined {
  context: BrowserContext;
  page: Page;
}

/** Signs `who` in with their cookie and joins the bench's voice channel. */
async function join(
  browser: Browser,
  who: Account,
  scene: Scene,
  viewport: { width: number; height: number },
  share: Record<string, unknown> = {},
): Promise<Joined> {
  const context = await browser.newContext({ baseURL: base.origin, viewport, locale: 'en-US' });
  const eq = who.cookie.indexOf('=');
  await context.addCookies([
    { name: who.cookie.slice(0, eq), value: who.cookie.slice(eq + 1), url: base.origin },
  ]);
  await context.addInitScript(installFakeScreen, scene);
  const settings = {
    preset: '1080p60',
    codec: 'auto',
    autoFocus: true,
    selfPreview: false,
    hideNoVideo: false,
    filmstripHidden: false,
    statsOverlay: false,
    streamVolumes: {},
    ...share,
  };
  await context.addInitScript((data) => {
    localStorage.setItem('dsx.share', JSON.stringify({ v: 1, data }));
  }, settings);
  const page = await context.newPage();
  await page.goto(`/channels/${bench.guildId}/${bench.channelId}`);
  await page.getByRole('button', { name: 'Join voice' }).click();
  await expect(page.getByTestId('voice-panel')).toContainText('Voice connected', {
    timeout: 20_000,
  });
  return { context, page };
}

async function startShare(page: Page) {
  await page.getByTestId('stage').getByRole('button', { name: 'Share your screen' }).click();
  await expect(
    page.getByTestId('stage').getByRole('button', { name: 'Stop sharing' }),
  ).toBeVisible();
}

interface Launched {
  browser: Browser;
  /** The browser's main process; its CPU counts its children too. */
  pid: number;
  close: () => Promise<void>;
}

async function launch(): Promise<Launched> {
  const server = await chromium.launchServer({ channel: 'chromium', args: [...args, ...gpu] });
  const browser = await chromium.connect(server.wsEndpoint());
  return {
    browser,
    pid: server.process().pid ?? 0,
    close: async () => {
      await browser.close();
      await server.close();
    },
  };
}

const size = (l: { width: number; height: number }) => `${String(l.width)}×${String(l.height)}`;
const layer = (l: OutLayer | undefined) =>
  l
    ? `${size(l)} ${String(l.fps)} fps ${String(l.kbps)} kbps ${String(l.msPerFrame)} ms/f${l.active ? '' : ' (paused)'}`
    : '—';
const stream = (s: InStream | undefined) =>
  s ? `${size(s)} ${String(s.fps)} fps ${String(s.kbps)} kbps ${String(s.msPerFrame)} ms/f` : '—';

test('codec and optimization at 1440p60, one sharer and one viewer in focus', async () => {
  test.setTimeout(20 * 60_000);
  const sharerBrowser = await launch();
  const viewerBrowser = await launch();
  // Large enough that the focused tile asks for the full 2560-wide layer.
  const viewer = await join(viewerBrowser.browser, account(2), 'edit', {
    width: 3200,
    height: 1800,
  });
  report.push(
    '## 1440p60, one sharer, one viewer in focus',
    '',
    `Measured ${String(WINDOW_MS / 1000)} s after ${String(WARMUP_MS / 1000)} s of warm-up, in full Chromium with the GPU. CPU is the whole browser (all its processes) in cores; the sharer's includes drawing the synthetic screen. "scroll" scrolls the whole editor every frame (the worst case for text), "edit" types with a short scroll every 3 s; the 1440p60 preset tunes every scene for motion.`,
    '',
    '| Content | Codec | Encoder | Captured | Bandwidth estimate | Sent: full layer | Sent: low layer | Encode busy | Sharer CPU | Received in focus | Decoder | Freezes | Viewer CPU | Limited by |',
    '|---|---|---|---|---|---|---|---|---|---|---|---|---|---|',
  );
  for (const scene of ['scroll', 'edit', 'motion'] as const) {
    for (const codec of CODECS) {
      const sharer = await join(
        sharerBrowser.browser,
        account(1),
        scene,
        { width: 1280, height: 720 },
        { preset: '1440p60', codec },
      );
      await startShare(sharer.page);
      await expect(viewer.page.getByTestId('share-tile')).toHaveAttribute('data-video', 'true', {
        timeout: 20_000,
      });
      await viewer.page.waitForTimeout(WARMUP_MS);
      const c0 = { s: treeCpu(sharerBrowser.pid), v: treeCpu(viewerBrowser.pid), at: Date.now() };
      const [s0, v0] = await Promise.all([snapshot(sharer.page), snapshot(viewer.page)]);
      await viewer.page.waitForTimeout(WINDOW_MS);
      const [s1, v1] = await Promise.all([snapshot(sharer.page), snapshot(viewer.page)]);
      const c1 = { s: treeCpu(sharerBrowser.pid), v: treeCpu(viewerBrowser.pid), at: Date.now() };
      const secs = (c1.at - c0.at) / 1000;
      const layers = sent(s0, s1);
      const full = layers.find((l) => l.active) ?? layers[0];
      const low = layers.find((l) => l !== full && l.width > 0 && l.width < (full?.width ?? 0));
      const busy = layers.reduce((sum, l) => sum + (l.msPerFrame * l.fps) / 10, 0);
      const inbound = received(v0, v1).sort((a, b) => b.width - a.width)[0];
      report.push(
        `| ${scene} | ${codec} (${full?.codec.replace('video/', '') ?? '?'}${full?.mode ? ` ${full.mode}` : ''}) | ${full?.encoder ?? ''} | ${String(sourceFps(s0, s1))} fps | ${(s1.bwe / 1e6).toFixed(1)} Mbps | ${layer(full)} | ${layer(low)} | ${busy.toFixed(1)} % | ${((c1.s - c0.s) / secs).toFixed(2)} | ${stream(inbound)} | ${inbound?.decoder ?? ''} | ${String(inbound?.freezes ?? 0)} | ${((c1.v - c0.v) / secs).toFixed(2)} | ${full?.limitation ?? ''} |`,
      );
      await sharer.page
        .getByTestId('voice-panel')
        .getByRole('button', { name: 'Disconnect' })
        .click();
      await sharer.context.close();
      await expect(viewer.page.getByTestId('share-tile')).toBeHidden({ timeout: 20_000 });
    }
  }
  report.push('');
  await sharerBrowser.close();
  await viewerBrowser.close();
});

test('three shares and five viewers in the grid: low layers, and dynacast pauses the full ones', async () => {
  test.setTimeout(10 * 60_000);
  const { browser, close } = await launch();
  const share = { preset: '1080p30', codec: 'auto', autoFocus: false };
  const viewport = { width: 1280, height: 800 };
  const sharers: Joined[] = [];
  for (const i of [1, 2, 3]) sharers.push(await join(browser, account(i), 'edit', viewport, share));
  const viewers: Joined[] = [];
  // Viewer 2 has a large window: a focused tile there needs the full layer.
  for (const i of [0, 4, 5, 6, 7]) {
    const size = i === 4 ? { width: 3200, height: 1800 } : viewport;
    viewers.push(await join(browser, account(i), 'edit', size, share));
  }
  for (const s of sharers) await startShare(s.page);
  for (const v of viewers) {
    await expect(v.page.locator('[data-testid="share-tile"][data-video="true"]')).toHaveCount(3, {
      timeout: 30_000,
    });
  }
  await viewers[0]?.page.waitForTimeout(10_000);

  const measure = async () => {
    const pages = [...sharers, ...viewers].map((j) => j.page);
    const a = await Promise.all(pages.map(snapshot));
    await viewers[0]?.page.waitForTimeout(10_000);
    const b = await Promise.all(pages.map(snapshot));
    return pages.map((_, i) => {
      const before = a[i];
      const after = b[i];
      return before && after
        ? { out: sent(before, after), in: received(before, after) }
        : { out: [], in: [] };
    });
  };

  const grid = await measure();
  report.push(
    '## Three 1080p30 shares, five more viewers, everyone in the grid',
    '',
    '| Who | Sent layers | Received |',
    '|---|---|---|',
  );
  grid.forEach((r, i) => {
    const who = i < 3 ? `sharer ${String(i + 1)}` : `viewer ${String(i - 2)}`;
    report.push(`| ${who} | ${r.out.map(layer).join('<br>')} | ${r.in.map(stream).join('<br>')} |`);
  });
  report.push('');
  for (const r of grid.slice(3)) {
    expect(r.in).toHaveLength(3);
    for (const s of r.in) expect(s.width).toBeLessThanOrEqual(960);
  }
  for (const r of grid.slice(0, 3)) {
    const full = r.out.find((l) => l.width === 1920);
    expect(!full || !full.active || full.kbps < 50).toBe(true);
  }

  // Viewer 2 focuses the first share: its full layer comes back.
  const watcher = viewers[1]?.page;
  if (!watcher) throw new Error('no watcher');
  await watcher.getByTestId('share-tile').first().dblclick();
  await watcher.waitForTimeout(8000);
  const focused = await measure();
  report.push(
    'After viewer 2 focuses a share in a large window:',
    '',
    '| Who | Sent layers | Received |',
    '|---|---|---|',
  );
  focused.forEach((r, i) => {
    const who = i < 3 ? `sharer ${String(i + 1)}` : `viewer ${String(i - 2)}`;
    report.push(`| ${who} | ${r.out.map(layer).join('<br>')} | ${r.in.map(stream).join('<br>')} |`);
  });
  report.push('');
  expect(focused[4]?.in.some((s) => s.width === 1920)).toBe(true);
  await close();
});
