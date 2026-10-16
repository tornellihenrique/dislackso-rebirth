import { expect, test, type Browser, type BrowserContext, type Page } from '@playwright/test';

import { ADMIN, MEMBER, signIn, trackErrors } from './fixtures';

// Runs after app.spec.ts (files run in name order on one worker), which
// creates the admin and the member accounts.
test.describe.configure({ mode: 'serial' });

/** ROADMAP Phase 2: changes reach the other browser within 500 ms. */
const LIVE = { timeout: 500 };
const SERVER = 'Game Dev';

interface Session {
  context: BrowserContext;
  page: Page;
  errors: string[];
}

async function signedIn(
  browser: Browser,
  who: { username: string; password: string },
): Promise<Session> {
  const context = await browser.newContext();
  const page = await context.newPage();
  const errors = trackErrors(page);
  await page.goto('/login');
  await signIn(page, who.username, who.password);
  await expect(page).toHaveURL('/');
  return { context, page, errors };
}

const members = (page: Page) => page.getByRole('complementary', { name: 'Members' });
const channels = (page: Page) => page.getByRole('navigation', { name: 'Channels' });
const servers = (page: Page) => page.getByRole('navigation', { name: 'Servers' });

async function openInvite(page: Page): Promise<string> {
  await page.getByRole('button', { name: SERVER, exact: true }).click();
  await page.getByRole('menuitem', { name: 'Invite people' }).click();
  const link = await page.getByRole('dialog').getByLabel('Invite link').inputValue();
  await page.keyboard.press('Escape');
  return new URL(link).pathname;
}

async function accept(page: Page, invitePath: string) {
  await page.goto(invitePath);
  await page.getByRole('button', { name: 'Accept invite' }).click();
  await expect(page).toHaveURL(/\/channels\/\d+\/\d+$/);
}

let admin: Session;
let ana: Session;

test.beforeAll(async ({ browser }) => {
  admin = await signedIn(browser, ADMIN);
  ana = await signedIn(browser, MEMBER);
});

test.afterAll(async () => {
  await admin.context.close();
  await ana.context.close();
});

test('creating a server and joining it with an invite shows up live', async () => {
  const { page } = admin;
  await servers(page).getByRole('button', { name: 'Add a server' }).click();
  await page.getByRole('dialog').getByLabel('Server name').fill(SERVER);
  await page.getByRole('dialog').getByRole('button', { name: 'Create server' }).click();
  await expect(page).toHaveURL(/\/channels\/\d+\/\d+$/);
  await expect(channels(page).getByRole('link', { name: 'general', exact: true })).toBeVisible();
  await expect(members(page).getByText(ADMIN.username, { exact: true })).toBeVisible();

  await accept(ana.page, await openInvite(page));
  await expect(
    members(page).getByRole('button', { name: new RegExp(MEMBER.username) }),
  ).toBeVisible(LIVE);
  await expect(servers(ana.page).getByRole('link', { name: SERVER })).toBeVisible();
});

test('channels created, renamed and reordered by an admin appear live', async () => {
  const { page } = admin;
  await page.getByRole('button', { name: SERVER, exact: true }).click();
  await page.getByRole('menuitem', { name: 'Create channel' }).click();
  await page.getByRole('dialog').getByLabel('Channel name').fill('Release Notes');
  await page.getByRole('dialog').getByRole('button', { name: 'Create channel' }).click();
  await expect(channels(page).getByRole('link', { name: 'release-notes' })).toBeVisible();
  await expect(channels(ana.page).getByRole('link', { name: 'release-notes' })).toBeVisible(LIVE);

  await channels(page).getByRole('link', { name: 'release-notes' }).click({ button: 'right' });
  await page.getByRole('menuitem', { name: 'Edit channel' }).click();
  await page.getByRole('dialog').getByLabel('Channel name').fill('changelog');
  await page.getByRole('dialog').getByRole('button', { name: 'Save' }).click();
  await expect(channels(page).getByRole('link', { name: 'changelog' })).toBeVisible();
  await expect(channels(ana.page).getByRole('link', { name: 'changelog' })).toBeVisible(LIVE);

  // Drag #changelog onto the voice channel, into the voice category.
  // dnd-kit starts a drag only after a few pixels of movement.
  const order = (p: Page) => channels(p).getByRole('link').allTextContents();
  await expect.poll(() => order(ana.page)).toEqual(['changelog', 'general', 'General']);
  const from = await channels(page).getByRole('link', { name: 'changelog' }).boundingBox();
  const to = await channels(page).getByRole('link', { name: 'General', exact: true }).boundingBox();
  if (!from || !to) throw new Error('channel rows are not visible');
  await page.mouse.move(from.x + from.width / 2, from.y + from.height / 2);
  await page.mouse.down();
  await page.mouse.move(from.x + from.width / 2, from.y + from.height / 2 + 10, { steps: 5 });
  await page.mouse.move(to.x + to.width / 2, to.y + to.height - 2, { steps: 15 });
  await page.mouse.up();
  await expect.poll(() => order(page)).toEqual(['general', 'General', 'changelog']);
  await expect.poll(() => order(ana.page), LIVE).toEqual(['general', 'General', 'changelog']);
});

test('role and presence changes appear live', async () => {
  await members(admin.page)
    .getByRole('button', { name: new RegExp(MEMBER.username) })
    .click({ button: 'right' });
  await admin.page.getByRole('menuitem', { name: 'Make admin' }).click();
  await expect(members(ana.page).getByRole('heading', { name: 'Admins — 1' })).toBeVisible(LIVE);

  await ana.page.getByRole('button', { name: 'Set status' }).click();
  await ana.page.getByRole('menuitem', { name: 'Do not disturb' }).click();
  await expect(
    members(admin.page).getByRole('button', { name: new RegExp(MEMBER.username) }),
  ).toContainText('Do not disturb', LIVE);
});

test('the gateway resumes after the network drops, without a reload', async ({ browser }) => {
  // A fresh page whose gateway runs through a route the test can cut.
  const context = await browser.newContext({ storageState: await ana.context.storageState() });
  const page = await context.newPage();
  const net = { down: false, firstFrames: [] as number[], cut: (): void => undefined };
  await page.routeWebSocket(/\/gateway$/, (ws) => {
    if (net.down) {
      void ws.close({ code: 4000, reason: 'offline' });
      return;
    }
    const server = ws.connectToServer();
    let first = true;
    ws.onMessage((m) => {
      if (first && typeof m !== 'string') net.firstFrames.push(m[0] ?? 0);
      first = false;
      server.send(m);
    });
    server.onMessage((m) => {
      ws.send(m);
    });
    net.cut = () => {
      void server.close({ code: 4000, reason: 'network down' });
    };
  });
  await page.goto('/');
  await servers(page).getByRole('link', { name: SERVER }).click();
  await expect(channels(page).getByRole('link', { name: 'changelog' })).toBeVisible();
  await page.evaluate(() => {
    (window as unknown as { marker: number }).marker = 42;
  });

  net.down = true;
  net.cut();
  await expect(page.getByRole('status').filter({ hasText: 'Reconnecting' })).toBeVisible({
    timeout: 5000,
  });

  // While offline, the admin renames a channel.
  await channels(admin.page).getByRole('link', { name: 'changelog' }).click({ button: 'right' });
  await admin.page.getByRole('menuitem', { name: 'Edit channel' }).click();
  await admin.page.getByRole('dialog').getByLabel('Channel name').fill('while-offline');
  await admin.page.getByRole('dialog').getByRole('button', { name: 'Save' }).click();
  await expect(channels(admin.page).getByRole('link', { name: 'while-offline' })).toBeVisible();

  net.down = false;
  await page.evaluate(() => window.dispatchEvent(new Event('online')));
  await expect(channels(page).getByRole('link', { name: 'while-offline' })).toBeVisible({
    timeout: 10_000,
  });
  await expect(page.getByRole('status').filter({ hasText: 'Reconnecting' })).toBeHidden();
  // The first frame on the last connection was Resume (field 2), not Identify (field 1).
  expect(net.firstFrames.at(0)).toBe(0x0a);
  expect(net.firstFrames.at(-1)).toBe(0x12);
  expect(await page.evaluate(() => (window as unknown as { marker?: number }).marker)).toBe(42);
  await context.close();
});

test('a kicked member loses the server at once; a banned one cannot come back', async () => {
  const { page } = admin;
  const anaRow = () => members(page).getByRole('button', { name: new RegExp(MEMBER.username) });

  await anaRow().click({ button: 'right' });
  await page.getByRole('menuitem', { name: `Kick ${MEMBER.username}` }).click();
  await page.getByRole('dialog').getByRole('button', { name: 'Kick' }).click();
  await expect(servers(ana.page).getByRole('link', { name: SERVER })).toBeHidden(LIVE);
  await expect(ana.page.getByText(`You were removed from ${SERVER}.`)).toBeVisible();
  await expect(ana.page).toHaveURL('/');

  const invite = await openInvite(page);
  await accept(ana.page, invite);
  await anaRow().click({ button: 'right' });
  await page.getByRole('menuitem', { name: `Ban ${MEMBER.username}` }).click();
  await page.getByRole('dialog').getByLabel('Reason (optional)').fill('e2e');
  await page.getByRole('dialog').getByRole('button', { name: 'Ban' }).click();
  await expect(servers(ana.page).getByRole('link', { name: SERVER })).toBeHidden(LIVE);

  await ana.page.goto(invite);
  await expect(ana.page.getByText("You're banned from this server.")).toBeVisible();
  expect(admin.errors).toEqual([]);
  expect(ana.errors).toEqual([]);
});

test('servers reorder by dragging in the rail without leaving the page', async () => {
  const { page } = admin;
  const before = page.url();
  await servers(page).getByRole('button', { name: 'Add a server' }).click();
  await page.getByRole('dialog').getByLabel('Server name').fill('Ops');
  await page.getByRole('dialog').getByRole('button', { name: 'Create server' }).click();
  await expect(page).not.toHaveURL(before);
  await expect(page).toHaveURL(/\/channels\/\d+\/\d+$/);
  const url = page.url();
  await page.evaluate(() => {
    (window as unknown as { stayed?: boolean }).stayed = true;
  });
  const order = () =>
    servers(page)
      .getByRole('link')
      .evaluateAll((links) => links.map((l) => l.getAttribute('aria-label')));
  await expect.poll(order).toEqual(['Home', SERVER, 'Ops']);

  const from = await servers(page).getByRole('link', { name: SERVER }).boundingBox();
  const to = await servers(page).getByRole('link', { name: 'Ops' }).boundingBox();
  if (!from || !to) throw new Error('rail icons not laid out');
  await page.mouse.move(from.x + from.width / 2, from.y + from.height / 2);
  await page.mouse.down();
  // Sideways first: the rail only moves icons up and down.
  await page.mouse.move(from.x + from.width / 2 + 60, from.y + from.height / 2, { steps: 5 });
  await page.mouse.move(to.x + to.width / 2, to.y + (to.height * 3) / 4, { steps: 10 });
  await page.mouse.up();

  await expect.poll(order).toEqual(['Home', 'Ops', SERVER]);
  // The release neither opened the dragged server nor reloaded the app.
  expect(page.url()).toBe(url);
  expect(await page.evaluate(() => (window as unknown as { stayed?: boolean }).stayed)).toBe(true);
  expect(admin.errors).toEqual([]);
});
