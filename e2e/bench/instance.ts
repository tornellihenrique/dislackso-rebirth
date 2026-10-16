import { request } from 'node:http';

export interface Account {
  readonly username: string;
  /** The session cookie, "name=value". */
  readonly cookie: string;
}

export interface Bench {
  readonly accounts: readonly Account[];
  readonly guildId: string;
  readonly channelId: string;
}

interface Answer {
  readonly data: Record<string, unknown>;
  readonly cookie: string;
}

/**
 * Calls the hub's Connect API with JSON. localAddress picks the loopback
 * address requests come from: the hub limits registrations per address.
 */
function rpc(
  base: URL,
  method: string,
  body: unknown,
  opts: { cookie?: string; localAddress?: string } = {},
): Promise<Answer> {
  const payload = JSON.stringify(body);
  return new Promise((resolve, reject) => {
    const req = request(
      {
        host: base.hostname,
        port: base.port,
        path: `/rpc/dislackso.v1.${method}`,
        method: 'POST',
        ...(opts.localAddress ? { localAddress: opts.localAddress } : {}),
        headers: {
          'Content-Type': 'application/json',
          'Connect-Protocol-Version': '1',
          'Content-Length': Buffer.byteLength(payload),
          ...(opts.cookie ? { Cookie: opts.cookie } : {}),
        },
      },
      (res) => {
        let text = '';
        res.setEncoding('utf8');
        res.on('data', (chunk: string) => {
          text += chunk;
        });
        res.on('end', () => {
          if ((res.statusCode ?? 500) >= 300) {
            reject(new Error(`${method}: ${String(res.statusCode)} ${text}`));
            return;
          }
          const cookie = (res.headers['set-cookie'] ?? []).map((c) => c.split(';')[0]).join('; ');
          resolve({
            data: text === '' ? {} : (JSON.parse(text) as Record<string, unknown>),
            cookie,
          });
        });
      },
    );
    req.on('error', reject);
    req.end(payload);
  });
}

/**
 * Sets up a fresh instance: the admin, `members` more accounts, and a server
 * everyone is in. Returns the accounts (admin first) and its voice channel.
 */
export async function createBench(base: URL, setupToken: string, members: number): Promise<Bench> {
  const password = 'bench horse staple battery';
  const admin = { username: 'benchadmin', password };
  const setup = await rpc(base, 'SetupService/CompleteSetup', {
    token: setupToken,
    instanceName: 'Bench',
    publicUrl: base.origin,
    adminUsername: admin.username,
    adminEmail: 'benchadmin@bench.test',
    adminPassword: password,
  });
  const cookie = setup.cookie;
  if (cookie === '') throw new Error('setup did not sign the admin in');
  await rpc(
    base,
    'InstanceService/UpdateSettings',
    { registrationMode: 'REGISTRATION_MODE_OPEN' },
    { cookie },
  );
  const created = await rpc(base, 'GuildService/CreateGuild', { name: 'Bench' }, { cookie });
  const state = created.data.state as {
    guild: { id: string };
    channels: { id: string; type: string }[];
  };
  const voice = state.channels.find((c) => c.type === 'CHANNEL_TYPE_VOICE');
  if (!voice) throw new Error('the new server has no voice channel');
  const invite = await rpc(
    base,
    'InviteService/CreateInvite',
    { guildId: state.guild.id },
    { cookie },
  );
  const code = (invite.data.invite as { code: string }).code;

  const accounts: Account[] = [{ username: admin.username, cookie }];
  for (let i = 1; i <= members; i++) {
    const username = `bench${String(i)}`;
    // Five registrations per address, then one per ten minutes.
    const localAddress = `127.0.0.${String(1 + Math.floor((i - 1) / 5))}`;
    const reg = await rpc(
      base,
      'AuthService/Register',
      { username, email: `${username}@bench.test`, password },
      { localAddress },
    );
    const session =
      reg.cookie !== ''
        ? reg.cookie
        : (await rpc(base, 'AuthService/Login', { login: username, password }, { localAddress }))
            .cookie;
    await rpc(base, 'InviteService/AcceptInvite', { code }, { cookie: session, localAddress });
    accounts.push({ username, cookie: session });
  }
  return { accounts, guildId: state.guild.id, channelId: voice.id };
}
