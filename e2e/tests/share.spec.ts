import {
  expect,
  test,
  type Browser,
  type BrowserContext,
  type Locator,
  type Page,
} from '@playwright/test';

import { ADMIN, MEMBER, signIn, trackErrors } from './fixtures';

// Its own server, like voice.spec.ts. Chromium's fake capture devices
// (playwright.config.ts) answer getDisplayMedia with a moving test pattern.
// The drawing tests are here too: another spec's sign-ins would pass the
// per-account login limit.
test.describe.configure({ mode: 'serial' });

const LIVE = { timeout: 1000 };
const CONNECT = { timeout: 15_000 };
const SERVER = 'Share Lab';

interface Session {
  context: BrowserContext;
  page: Page;
  errors: string[];
}

async function open(browser: Browser): Promise<Session> {
  const context = await browser.newContext();
  const page = await context.newPage();
  return { context, page, errors: trackErrors(page) };
}

const channels = (page: Page) => page.getByRole('navigation', { name: 'Channels' });
const stage = (page: Page) => page.getByTestId('stage');
const shareTile = (page: Page) => page.getByTestId('share-tile');
const occupant = (page: Page, name: string) =>
  page.getByTestId('voice-occupant').filter({ hasText: name });

async function joinVoice(page: Page) {
  await channels(page).getByRole('link', { name: 'General', exact: true }).click();
  await expect(page.getByTestId('voice-panel')).toContainText('Voice connected', CONNECT);
}

/** The other person's share on this page, and this device's own. */
const theirs = (page: Page) => page.locator('[data-testid="share-tile"]:not([data-self])');
const mine = (page: Page) => page.locator('[data-testid="share-tile"][data-self]');

type P = readonly [number, number];

/** Where the tile's video shows its frame (object-fit: contain), in page pixels. */
async function contentBox(tile: Locator) {
  return tile.evaluate((el) => {
    const v = el.querySelector('video');
    const r = el.getBoundingClientRect();
    if (!v || v.videoWidth === 0) throw new Error('no frame yet');
    const scale = Math.min(r.width / v.videoWidth, r.height / v.videoHeight);
    const width = v.videoWidth * scale;
    const height = v.videoHeight * scale;
    return { x: r.left + (r.width - width) / 2, y: r.top + (r.height - height) / 2, width, height };
  });
}

/** Draws a straight line between two points of the content, normalized. */
async function line(page: Page, tile: Locator, from: P, to: P) {
  const b = await contentBox(tile);
  const at = (p: P) => [b.x + p[0] * b.width, b.y + p[1] * b.height] as const;
  await page.mouse.move(...at(from));
  await page.mouse.down();
  for (let i = 1; i <= 15; i++) {
    const t = i / 15;
    await page.mouse.move(
      ...at([from[0] + (to[0] - from[0]) * t, from[1] + (to[1] - from[1]) * t]),
    );
  }
  await page.mouse.up();
}

/** The most opaque drawn pixel around a normalized point of the tile's content. */
async function inkAt(tile: Locator, p: P): Promise<number> {
  return tile.evaluate((el, [px, py]) => {
    const v = el.querySelector('video');
    const c = el.querySelector('canvas');
    if (!v || !c) return 0;
    const r = c.getBoundingClientRect();
    const scale = Math.min(r.width / v.videoWidth, r.height / v.videoHeight);
    const w = v.videoWidth * scale;
    const h = v.videoHeight * scale;
    const dpr = c.width / r.width;
    const x = Math.round(((r.width - w) / 2 + px * w) * dpr);
    const y = Math.round(((r.height - h) / 2 + py * h) * dpr);
    const data = c.getContext('2d')?.getImageData(x - 3, y - 3, 7, 7).data ?? [];
    let max = 0;
    for (let i = 3; i < data.length; i += 4) max = Math.max(max, data[i] ?? 0);
    return max;
  }, p);
}

const strokes = (tile: Locator) =>
  tile.getByTestId('annotation-layer').getAttribute('data-strokes');

async function drawMode(tile: Locator) {
  await tile.hover();
  await tile.getByRole('button', { name: 'Draw' }).click();
  await expect(tile.getByTestId('annotation-layer')).toHaveAttribute('data-drawing', 'true');
}

/** The share's <video>, its size and whether frames keep coming. */
async function video(page: Page) {
  return page
    .getByTestId('share-tile')
    .locator('video')
    .evaluate(async (v: HTMLVideoElement) => {
      const before = v.currentTime;
      await new Promise((r) => setTimeout(r, 400));
      return {
        width: v.videoWidth,
        playing: !v.paused && v.currentTime > before,
        tagged: (v as HTMLVideoElement & { tag?: string }).tag ?? '',
      };
    });
}

let admin: Session;
let ana: Session;

test.beforeAll(async ({ browser }) => {
  admin = await open(browser);
  await admin.page.goto('/login');
  await signIn(admin.page, ADMIN.username, ADMIN.password);
  await expect(admin.page).toHaveURL('/');
  ana = await open(browser);
  await ana.page.goto('/login');
  await signIn(ana.page, MEMBER.username, MEMBER.password);
  await expect(ana.page).toHaveURL('/');

  const page = admin.page;
  await page
    .getByRole('navigation', { name: 'Servers' })
    .getByRole('button', { name: 'Add a server' })
    .click();
  await page.getByRole('dialog').getByLabel('Server name').fill(SERVER);
  await page.getByRole('dialog').getByRole('button', { name: 'Create server' }).click();
  await expect(page).toHaveURL(/\/channels\/\d+\/\d+$/);
  await page.getByRole('button', { name: SERVER, exact: true }).click();
  await page.getByRole('menuitem', { name: 'Invite people' }).click();
  const link = await page.getByRole('dialog').getByLabel('Invite link').inputValue();
  await page.keyboard.press('Escape');
  await ana.page.goto(new URL(link).pathname);
  await ana.page.getByRole('button', { name: 'Accept invite' }).click();
  await expect(ana.page).toHaveURL(/\/channels\/\d+\/\d+$/);

  await joinVoice(admin.page);
  await joinVoice(ana.page);
});

test.afterAll(async () => {
  await admin.context.close();
  await ana.context.close();
});

test('a share shows live to everyone and the stage focuses it', async () => {
  await stage(admin.page).getByRole('button', { name: 'Share your screen' }).click();
  await expect(shareTile(admin.page)).toContainText('You are sharing', CONNECT);
  await expect(occupant(ana.page, ADMIN.username)).toContainText('LIVE', LIVE);

  // Someone else's first share takes the stage by itself.
  await expect(stage(ana.page)).toHaveAttribute('data-mode', 'focus', CONNECT);
  await expect(shareTile(ana.page)).toHaveAttribute('data-video', 'true', CONNECT);
  await expect.poll(async () => (await video(ana.page)).playing, CONNECT).toBe(true);
  // The sharer's own share does not.
  await expect(stage(admin.page)).toHaveAttribute('data-mode', 'grid');
});

test('the video stays live across grid, focus and fullscreen', async () => {
  const page = ana.page;
  await shareTile(page)
    .locator('video')
    .evaluate((v) => {
      (v as HTMLVideoElement & { tag?: string }).tag = 'first';
    });
  await page.keyboard.press('g');
  await expect(stage(page)).toHaveAttribute('data-mode', 'grid');
  expect(await video(page)).toMatchObject({ tagged: 'first', playing: true });

  await shareTile(page).dblclick();
  await expect(stage(page)).toHaveAttribute('data-mode', 'focus');
  expect(await video(page)).toMatchObject({ tagged: 'first', playing: true });

  await page.keyboard.press('f');
  await expect(stage(page)).toHaveAttribute('data-fullscreen', 'true');
  expect(await video(page)).toMatchObject({ tagged: 'first', playing: true });
  await page.keyboard.press('f');
  await expect(stage(page)).not.toHaveAttribute('data-fullscreen', 'true');
  expect(await video(page)).toMatchObject({ tagged: 'first', playing: true });

  // The tile's own button toggles too.
  await shareTile(page).hover();
  await shareTile(page).getByRole('button', { name: 'Fullscreen' }).click();
  await expect(stage(page)).toHaveAttribute('data-fullscreen', 'true');
  await shareTile(page).hover();
  await shareTile(page).getByRole('button', { name: 'Exit fullscreen' }).click();
  await expect(stage(page)).not.toHaveAttribute('data-fullscreen', 'true');
  expect(await video(page)).toMatchObject({ tagged: 'first', playing: true });

  await page.keyboard.press('Control+Shift+S');
  await expect(page.getByTestId('share-stats')).toContainText('Resolution', CONNECT);
  // A 60 fps share keeps the frame rate with VP9 where the browser can send it.
  await expect(page.getByTestId('share-stats')).toContainText('VP9');
  await page.keyboard.press('Control+Shift+S');
  await expect(page.getByTestId('share-stats')).toBeHidden();
});

test('a viewer can stop watching and watch again', async () => {
  const page = ana.page;
  await shareTile(page).hover();
  await shareTile(page).getByRole('button', { name: 'Stop watching' }).click();
  await expect(shareTile(page)).toContainText(`You stopped watching ${ADMIN.username}'s screen.`);
  // The large button in the tile; the toolbar has one too.
  await shareTile(page).getByRole('button', { name: 'Watch' }).first().click();
  await expect.poll(async () => (await video(page)).playing, CONNECT).toBe(true);
});

test('two people draw at once on two shares and everyone sees it on the same content', async () => {
  // A second share, and another window size: tiles and their letterboxing differ.
  await ana.page.setViewportSize({ width: 980, height: 820 });
  await stage(ana.page).getByRole('button', { name: 'Share your screen' }).click();
  await expect(mine(ana.page)).toHaveAttribute('data-video', 'true', CONNECT);
  for (const page of [admin.page, ana.page]) {
    await expect(theirs(page)).toHaveAttribute('data-video', 'true', CONNECT);
    await expect.poll(() => contentBox(theirs(page)).catch(() => undefined), CONNECT).toBeTruthy();
  }
  await drawMode(theirs(admin.page));
  await drawMode(theirs(ana.page));
  await Promise.all([
    line(admin.page, theirs(admin.page), [0.3, 0.3], [0.6, 0.6]),
    line(ana.page, theirs(ana.page), [0.3, 0.7], [0.6, 0.4]),
  ]);

  // Each sharer sees the other's stroke on their own screen, at the same spot
  // of the content although their tiles have other sizes.
  for (const [page, at, off] of [
    [ana.page, [0.45, 0.45], [0.45, 0.65]],
    [admin.page, [0.45, 0.55], [0.45, 0.35]],
  ] as const) {
    await expect.poll(() => strokes(mine(page))).toBe('1');
    await expect.poll(() => inkAt(mine(page), at)).toBeGreaterThan(100);
    expect(await inkAt(mine(page), off)).toBe(0);
  }
  // And each drawer sees their own line where they drew it.
  expect(await inkAt(theirs(admin.page), [0.45, 0.45])).toBeGreaterThan(100);
  expect(await inkAt(theirs(ana.page), [0.45, 0.55])).toBeGreaterThan(100);
});

test('strokes hold, then fade, and nothing stays', async () => {
  // Drawn in the previous test with the default 3 s hold and 0.8 s fade.
  await expect.poll(() => strokes(mine(ana.page)), { timeout: 6000 }).toBe('0');
  await line(admin.page, theirs(admin.page), [0.2, 0.5], [0.8, 0.5]);
  await expect.poll(() => strokes(mine(ana.page))).toBe('1');
  await ana.page.waitForTimeout(2500);
  expect(await strokes(mine(ana.page))).toBe('1');
  await expect.poll(() => strokes(mine(ana.page)), { timeout: 3000 }).toBe('0');
  expect(await inkAt(mine(ana.page), [0.5, 0.5])).toBe(0);
});

test('a stroke reaches the sharer in under 100 ms', async () => {
  const tile = theirs(admin.page);
  const b = await contentBox(tile);
  // When ana's screen first shows a new point of the stroke.
  const shown = ana.page.evaluate(
    () =>
      new Promise<number>((resolve) => {
        const c = document.querySelector('[data-testid="share-tile"][data-self] canvas');
        if (!c) throw new Error('no canvas');
        const before = Number(c.getAttribute('data-points') ?? 0);
        new MutationObserver(() => {
          if (Number(c.getAttribute('data-points') ?? 0) > before) {
            resolve(performance.timeOrigin + performance.now());
          }
        }).observe(c, { attributes: true, attributeFilter: ['data-points'] });
      }),
  );
  await admin.page.mouse.move(b.x + b.width * 0.5, b.y + b.height * 0.2);
  const pressedAt = await admin.page.evaluate(() => performance.timeOrigin + performance.now());
  await admin.page.mouse.down();
  const latency = (await shown) - pressedAt;
  await admin.page.mouse.up();
  console.log(`stroke latency: ${latency.toFixed(1)} ms`);
  expect(latency).toBeLessThan(100);
});

test('the sharer stopping others hides their pen within 500 ms', async () => {
  await admin.page.keyboard.press('Escape');
  // In focus, this device's own share is a filmstrip thumbnail, without tools.
  await admin.page.keyboard.press('g');
  await expect(stage(admin.page)).toHaveAttribute('data-mode', 'grid');
  await drawMode(mine(admin.page));
  // Ana is still drawing on the admin's share from the first test.
  const layer = theirs(ana.page).getByTestId('annotation-layer');
  const pen = theirs(ana.page).getByRole('button', { name: 'Draw' });
  await expect(layer).toHaveAttribute('data-drawing', 'true');
  await admin.page.getByRole('button', { name: 'Stop others from drawing' }).click();
  await expect(layer).not.toHaveAttribute('data-drawing', 'true', { timeout: 500 });
  await expect(pen).toHaveCount(0);
  await admin.page.getByRole('button', { name: 'Let others draw' }).click();
  await expect(pen).toHaveCount(1, { timeout: 500 });

  await admin.page.keyboard.press('Escape');
  await ana.page.getByTestId('voice-panel').getByRole('button', { name: 'Stop sharing' }).click();
  await expect(mine(ana.page)).toBeHidden(CONNECT);
  await expect(theirs(admin.page)).toBeHidden(CONNECT);
});

test("the browser's stop button ends the share and focus goes back to the grid", async () => {
  // What Chrome's "Stop sharing" bar does to the captured track.
  await shareTile(admin.page)
    .locator('video')
    .evaluate((v: HTMLVideoElement) => {
      const track = (v.srcObject as MediaStream).getVideoTracks()[0];
      track?.stop();
      track?.dispatchEvent(new Event('ended'));
    });
  await expect(shareTile(admin.page)).toBeHidden(CONNECT);
  await expect(stage(admin.page).getByRole('button', { name: 'Share your screen' })).toBeEnabled();
  await expect(shareTile(ana.page)).toBeHidden(CONNECT);
  await expect(stage(ana.page)).toHaveAttribute('data-mode', 'grid');
  await expect(occupant(ana.page, ADMIN.username)).not.toContainText('LIVE');

  await admin.page.getByTestId('voice-panel').getByRole('button', { name: 'Disconnect' }).click();
  await ana.page.getByTestId('voice-panel').getByRole('button', { name: 'Disconnect' }).click();
  expect(admin.errors).toEqual([]);
  expect(ana.errors).toEqual([]);
});
