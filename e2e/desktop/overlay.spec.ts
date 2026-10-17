import { mkdtempSync, readFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';

import {
  _electron as electron,
  expect,
  test,
  type BrowserContext,
  type ElectronApplication,
  type Locator,
  type Page,
} from '@playwright/test';

import { ADMIN, MEMBER, signIn, trackErrors } from '../tests/fixtures';

// The admin shares this machine's screen from the desktop app; Ana watches
// in a browser and draws on it.
test.describe.configure({ mode: 'serial' });

const CONNECT = { timeout: 15_000 };
const SERVER = 'Desk Lab';
/** Ana's ink: a color a desktop rarely shows, so the video can be searched for it. */
const INK = '#ff00ff';

const desktopDir = fileURLToPath(new URL('../../apps/desktop/', import.meta.url));
const electronPath = join(
  desktopDir,
  'node_modules/electron/dist',
  readFileSync(join(desktopDir, 'node_modules/electron/path.txt'), 'utf8'),
);

const channels = (page: Page) => page.getByRole('navigation', { name: 'Channels' });
const stage = (page: Page) => page.getByTestId('stage');
const theirs = (page: Page) => page.locator('[data-testid="share-tile"]:not([data-self])');
const mine = (page: Page) => page.locator('[data-testid="share-tile"][data-self]');

async function joinVoice(page: Page) {
  await channels(page).getByRole('link', { name: 'General', exact: true }).click();
  await expect(page.getByTestId('voice-panel')).toContainText('Voice connected', CONNECT);
}

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

/** Pixels like the ink along y = 0.5 of the share's video frame itself, without strokes. */
async function inkInVideo(tile: Locator): Promise<number> {
  return tile.evaluate((el) => {
    const v = el.querySelector('video');
    if (!v) return -1;
    const c = document.createElement('canvas');
    c.width = v.videoWidth;
    c.height = v.videoHeight;
    const ctx = c.getContext('2d');
    if (!ctx) return -1;
    ctx.drawImage(v, 0, 0);
    const y = Math.round(c.height / 2);
    const row = ctx.getImageData(Math.round(c.width * 0.35), y - 4, Math.round(c.width * 0.3), 9);
    let n = 0;
    for (let i = 0; i < row.data.length; i += 4) {
      const [r = 0, g = 0, b = 0] = [row.data[i], row.data[i + 1], row.data[i + 2]];
      if (r > 200 && g < 90 && b > 200) n++;
    }
    return n;
  });
}

async function overlayPage(app: ElectronApplication): Promise<Page> {
  await expect
    .poll(() => app.windows().some((w) => w.url().includes('/overlay/')), CONNECT)
    .toBe(true);
  const page = app.windows().find((w) => w.url().includes('/overlay/'));
  if (!page) throw new Error('no overlay window');
  return page;
}

let app: ElectronApplication;
let desk: Page;
let ana: { context: BrowserContext; page: Page; errors: string[] };

test.beforeAll(async ({ browser, baseURL }) => {
  app = await electron.launch({
    executablePath: electronPath,
    // The test reads English labels, whatever this machine's language.
    args: [join(desktopDir, 'out/main/index.js'), '--lang=en-US'],
    env: {
      ...process.env,
      DSX_INSTANCE_URL: baseURL ?? '',
      DSX_USER_DATA: mkdtempSync(join(tmpdir(), 'dislackso-desktop-')),
    },
  });
  desk = await app.firstWindow();
  await expect(desk).toHaveURL(/\/login/, CONNECT);
  await signIn(desk, ADMIN.username, ADMIN.password);
  await expect(desk).toHaveURL(/\/$/);

  const context = await browser.newContext({ viewport: { width: 1280, height: 800 } });
  await context.addInitScript((ink) => {
    localStorage.setItem('dsx.drawing', JSON.stringify({ v: 1, data: { color: ink, size: 'l' } }));
  }, INK);
  const page = await context.newPage();
  ana = { context, page, errors: trackErrors(page) };
  await page.goto('/login');
  await signIn(page, MEMBER.username, MEMBER.password);
  await expect(page).toHaveURL('/');

  await desk
    .getByRole('navigation', { name: 'Servers' })
    .getByRole('button', { name: 'Add a server' })
    .click();
  await desk.getByRole('dialog').getByLabel('Server name').fill(SERVER);
  await desk.getByRole('dialog').getByRole('button', { name: 'Create server' }).click();
  await expect(desk).toHaveURL(/\/channels\/\d+\/\d+$/);
  await desk.getByRole('button', { name: SERVER, exact: true }).click();
  await desk.getByRole('menuitem', { name: 'Invite people' }).click();
  const link = await desk.getByRole('dialog').getByLabel('Invite link').inputValue();
  await desk.keyboard.press('Escape');
  await page.goto(new URL(link).pathname);
  await page.getByRole('button', { name: 'Accept invite' }).click();
  await expect(page).toHaveURL(/\/channels\/\d+\/\d+$/);

  await joinVoice(desk);
  await joinVoice(page);
});

test.afterAll(async () => {
  await ana.context.close();
  await app.close();
});

test('the desktop app shares a screen through its own picker, with an overlay on it', async () => {
  await expect.poll(() => desk.evaluate(() => 'desktop' in window)).toBe(true);
  await stage(desk).getByRole('button', { name: 'Share your screen' }).click();
  const picker = desk.getByRole('dialog', { name: 'Share your screen' });
  await expect(picker.getByRole('tab', { name: 'Screens', selected: true })).toBeVisible();
  await expect(picker.getByRole('listitem').first().locator('img')).toBeVisible(CONNECT);
  await picker.getByRole('button', { name: 'Share', exact: true }).click();
  await expect(mine(desk)).toHaveAttribute('data-video', 'true', CONNECT);
  await expect(theirs(ana.page)).toHaveAttribute('data-video', 'true', CONNECT);

  const overlay = await overlayPage(app);
  await expect(overlay.getByRole('toolbar')).toBeVisible();
  // The overlay covers the display and is kept out of the capture.
  const shape = await app.evaluate(({ BrowserWindow }) =>
    BrowserWindow.getAllWindows()
      .filter((w) => w.webContents.getURL().includes('/overlay/'))
      .map((w) => ({
        top: w.isAlwaysOnTop(),
        protected: w.isContentProtected(),
        focusable: w.isFocusable(),
      })),
  );
  expect(shape).toEqual([{ top: true, protected: true, focusable: false }]);
});

test("a viewer's stroke shows on the sharer's real screen, and not twice in the video", async () => {
  // Out of the way, so the captured screen shows what is behind the app.
  await app.evaluate(({ BrowserWindow }) => {
    for (const w of BrowserWindow.getAllWindows()) {
      if (!w.webContents.getURL().includes('/overlay/')) w.minimize();
    }
  });
  const overlay = await overlayPage(app);
  const tile = theirs(ana.page);
  await tile.hover();
  await tile.getByRole('button', { name: 'Draw' }).click();
  const b = await contentBox(tile);
  await ana.page.mouse.move(b.x + b.width * 0.35, b.y + b.height * 0.5);
  await ana.page.mouse.down();
  for (let i = 1; i <= 15; i++) {
    await ana.page.mouse.move(b.x + b.width * (0.35 + 0.3 * (i / 15)), b.y + b.height * 0.5);
  }
  await ana.page.mouse.up();

  await expect.poll(() => overlay.locator('#ink').getAttribute('data-strokes')).toBe('1');
  const inked = await overlay.locator('#ink').evaluate((c: HTMLCanvasElement) => {
    const data = c.getContext('2d')?.getImageData(c.width / 2 - 3, c.height / 2 - 3, 7, 7).data;
    let max = 0;
    for (let i = 3; i < (data?.length ?? 0); i += 4) max = Math.max(max, data?.[i] ?? 0);
    return max;
  });
  expect(inked).toBeGreaterThan(100);
  // Frames captured while the stroke is on screen do not carry it.
  await ana.page.waitForTimeout(700);
  expect(await inkInVideo(tile)).toBe(0);
  await ana.page.keyboard.press('Escape');
});

test('the sharer draws on the overlay and the others see it on the share', async () => {
  const overlay = await overlayPage(app);
  const before = Number(
    (await theirs(ana.page).getByTestId('annotation-layer').getAttribute('data-strokes')) ?? 0,
  );
  await overlay.getByRole('button', { name: 'Draw on your screen' }).click();
  await expect(overlay.locator('body')).toHaveClass(/drawing/);
  const size = await overlay.evaluate(() => ({ w: window.innerWidth, h: window.innerHeight }));
  await overlay.mouse.move(size.w * 0.3, size.h * 0.3);
  await overlay.mouse.down();
  for (let i = 1; i <= 10; i++) {
    await overlay.mouse.move(size.w * (0.3 + 0.02 * i), size.h * (0.3 + 0.02 * i));
  }
  await overlay.mouse.up();
  await expect
    .poll(async () =>
      Number(
        (await theirs(ana.page).getByTestId('annotation-layer').getAttribute('data-strokes')) ?? 0,
      ),
    )
    .toBeGreaterThan(before);
  await overlay.keyboard.press('Escape');
  await expect(overlay.locator('body')).not.toHaveClass(/drawing/);
});

test('stopping the share from the overlay closes it', async () => {
  const overlay = await overlayPage(app);
  await overlay.getByRole('button', { name: 'Stop sharing' }).click();
  await expect
    .poll(() => app.windows().some((w) => w.url().includes('/overlay/')), CONNECT)
    .toBe(false);
  await expect(theirs(ana.page)).toBeHidden(CONNECT);
  expect(ana.errors).toEqual([]);
});
