import { expect, test, type Browser, type BrowserContext, type Page } from '@playwright/test';

import { ADMIN, MEMBER, signIn, trackErrors } from './fixtures';

// Runs after guilds.spec.ts, on its own server: the member was banned from
// the one before. Chromium gets a synthetic microphone (playwright.config.ts).
test.describe.configure({ mode: 'serial' });

const LIVE = { timeout: 1000 };
/** Connecting to the media server takes ICE and DTLS on top of the hub. */
const CONNECT = { timeout: 15_000 };
const SERVER = 'Voice Lab';

interface Session {
  context: BrowserContext;
  page: Page;
  errors: string[];
}

async function open(
  browser: Browser,
  storageState?: Awaited<ReturnType<BrowserContext['storageState']>>,
): Promise<Session> {
  const context = await browser.newContext(storageState ? { storageState } : {});
  const page = await context.newPage();
  return { context, page, errors: trackErrors(page) };
}

const channels = (page: Page) => page.getByRole('navigation', { name: 'Channels' });
const servers = (page: Page) => page.getByRole('navigation', { name: 'Servers' });
const panel = (page: Page) => page.getByTestId('voice-panel');
// The stage has the same toggles; these tests use the ones under the user's name.
const userPanel = (page: Page) => page.getByTestId('user-panel');
const occupant = (page: Page, name: string) =>
  page.getByTestId('voice-occupant').filter({ hasText: name });

async function joinVoice(page: Page, channel: string) {
  await channels(page).getByRole('link', { name: channel, exact: true }).click();
  await expect(panel(page)).toContainText('Voice connected', CONNECT);
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
  await servers(page).getByRole('button', { name: 'Add a server' }).click();
  await page.getByRole('dialog').getByLabel('Server name').fill(SERVER);
  await page.getByRole('dialog').getByRole('button', { name: 'Create server' }).click();
  await expect(page).toHaveURL(/\/channels\/\d+\/\d+$/);
  await page.getByRole('button', { name: SERVER, exact: true }).click();
  await page.getByRole('menuitem', { name: 'Create channel' }).click();
  await page.getByRole('dialog').getByRole('radio', { name: 'Voice' }).check();
  await page.getByRole('dialog').getByLabel('Channel name').fill('Focus');
  await page.getByRole('dialog').getByRole('button', { name: 'Create channel' }).click();
  await page.getByRole('button', { name: SERVER, exact: true }).click();
  await page.getByRole('menuitem', { name: 'Invite people' }).click();
  const link = await page.getByRole('dialog').getByLabel('Invite link').inputValue();
  await page.keyboard.press('Escape');
  await ana.page.goto(new URL(link).pathname);
  await ana.page.getByRole('button', { name: 'Accept invite' }).click();
  await expect(ana.page).toHaveURL(/\/channels\/\d+\/\d+$/);
});

test.afterAll(async () => {
  await admin.context.close();
  await ana.context.close();
});

test('people in voice show in every member sidebar, and switching is fast', async () => {
  await joinVoice(admin.page, 'General');
  await expect(occupant(admin.page, ADMIN.username)).toBeVisible();
  await expect(occupant(ana.page, ADMIN.username)).toBeVisible(LIVE);

  await joinVoice(ana.page, 'General');
  await expect(occupant(admin.page, MEMBER.username)).toBeVisible(LIVE);
  // Ana's synthetic microphone reaches the admin as a playing audio element.
  await expect
    .poll(
      () =>
        admin.page.evaluate(
          () => [...document.querySelectorAll('audio')].filter((a) => !a.paused).length,
        ),
      CONNECT,
    )
    .toBeGreaterThan(0);

  const started = Date.now();
  await channels(ana.page).getByRole('link', { name: 'Focus', exact: true }).click();
  await expect(panel(ana.page)).toContainText('Voice connected', CONNECT);
  await expect(admin.page.getByTestId('voice-occupant')).toHaveCount(2);
  const focusRows = channels(admin.page).locator('li', { hasText: MEMBER.username });
  await expect(focusRows).toHaveCount(1, LIVE);
  expect(Date.now() - started).toBeLessThan(5000);
  await joinVoice(ana.page, 'General');
});

test('mute and deafen show live, and undeafening restores the mute from before', async () => {
  const row = () => occupant(admin.page, MEMBER.username);
  await userPanel(ana.page).getByRole('button', { name: 'Mute', exact: true }).click();
  await expect(row().getByRole('img', { name: 'Muted' })).toBeVisible(LIVE);

  await userPanel(ana.page).getByRole('button', { name: 'Deafen', exact: true }).click();
  await expect(row().getByRole('img', { name: 'Deafened' })).toBeVisible(LIVE);
  await expect(row().getByRole('img', { name: 'Muted' })).toBeHidden();
  // Deafened, the admin's microphone is no longer played.
  await expect
    .poll(() =>
      ana.page.evaluate(() => [...document.querySelectorAll('audio')].every((a) => a.muted)),
    )
    .toBe(true);

  await userPanel(ana.page).getByRole('button', { name: 'Undeafen', exact: true }).click();
  await expect(row().getByRole('img', { name: 'Muted' })).toBeVisible(LIVE);
  await expect(row().getByRole('img', { name: 'Deafened' })).toBeHidden();

  // The state outlives a reload and applies when joining again.
  await ana.page.reload();
  await expect(
    userPanel(ana.page).getByRole('button', { name: 'Unmute', exact: true }),
  ).toHaveAttribute('aria-pressed', 'true');
  await joinVoice(ana.page, 'General');
  await expect(row().getByRole('img', { name: 'Muted' })).toBeVisible(LIVE);
  await userPanel(ana.page).getByRole('button', { name: 'Unmute', exact: true }).click();
  await expect(row().getByRole('img', { name: 'Muted' })).toBeHidden(LIVE);
});

test('server mute is shown to everyone and holds across a rejoin', async () => {
  await occupant(admin.page, MEMBER.username).click({ button: 'right' });
  await admin.page.getByRole('menuitemcheckbox', { name: 'Server mute' }).click();
  await expect(
    occupant(ana.page, MEMBER.username).getByRole('img', { name: 'Server muted' }),
  ).toBeVisible(LIVE);
  await expect(
    userPanel(ana.page).getByRole('button', { name: 'Muted by a moderator' }),
  ).toBeVisible();

  await panel(ana.page).getByRole('button', { name: 'Disconnect' }).click();
  await expect(occupant(admin.page, MEMBER.username)).toBeHidden(LIVE);
  await joinVoice(ana.page, 'General');
  await expect(
    occupant(admin.page, MEMBER.username).getByRole('img', { name: 'Server muted' }),
  ).toBeVisible(LIVE);

  await occupant(admin.page, MEMBER.username).click({ button: 'right' });
  await admin.page.getByRole('menuitemcheckbox', { name: 'Server mute' }).click();
  await expect(
    occupant(admin.page, MEMBER.username).getByRole('img', { name: 'Server muted' }),
  ).toBeHidden(LIVE);
});

test('joining from a second device moves voice there', async ({ browser }) => {
  const second = await open(browser, await ana.context.storageState());
  await second.page.goto('/');
  await servers(second.page).getByRole('link', { name: SERVER }).click();
  await joinVoice(second.page, 'General');
  await expect(ana.page.getByText('Voice connected on another device.')).toBeVisible(CONNECT);
  await expect(panel(ana.page)).toBeHidden();
  await expect(occupant(admin.page, MEMBER.username)).toHaveCount(1);
  expect(second.errors).toEqual([]);
  // Closing the tab leaves the call (LiveKit leaves on page unload).
  await second.page.close({ runBeforeUnload: true });
  await expect(occupant(admin.page, MEMBER.username)).toBeHidden(CONNECT);
  await second.context.close();
});

test('a moderator can disconnect someone from voice', async () => {
  await joinVoice(ana.page, 'General');
  await occupant(admin.page, MEMBER.username).click({ button: 'right' });
  await admin.page.getByRole('menuitem', { name: 'Disconnect from voice' }).click();
  await expect(ana.page.getByText(`${ADMIN.username} disconnected you from voice.`)).toBeVisible(
    LIVE,
  );
  await expect(panel(ana.page)).toBeHidden();
  await expect(occupant(admin.page, MEMBER.username)).toBeHidden(LIVE);

  await panel(admin.page).getByRole('button', { name: 'Disconnect' }).click();
  await expect(panel(admin.page)).toBeHidden();
  expect(admin.errors).toEqual([]);
  expect(ana.errors).toEqual([]);
});
