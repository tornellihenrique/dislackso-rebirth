import { expect, test, type Browser, type BrowserContext, type Page } from '@playwright/test';

import { ADMIN, MEMBER, signIn, trackErrors } from './fixtures';

// Its own server, like voice.spec.ts. Chromium's fake capture devices
// (playwright.config.ts) answer getDisplayMedia with a moving test pattern.
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
