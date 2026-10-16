import { create, type MessageInitShape } from '@bufbuild/protobuf';
import {
  type EventSchema,
  GuildDeleteReason,
  ServerFrameSchema,
} from '@dislackso/contracts/gen/dislackso/gateway/v1/gateway_pb';
import { PresenceStatus } from '@dislackso/contracts/gen/dislackso/v1/common_pb';
import { describe, expect, it, vi } from 'vitest';

import { createGatewayDispatcher, type DispatchTargets } from './dispatcher';

function setup() {
  const t = {
    entities: vi.fn(),
    presence: vi.fn(),
    voiceStates: vi.fn(),
    voiceUpdated: vi.fn(),
    voiceForced: vi.fn(),
    me: vi.fn(),
    ready: vi.fn(),
    listChanged: vi.fn(),
    guildLost: vi.fn(),
    exposureChanged: vi.fn(),
  } satisfies DispatchTargets;
  const dispatch = createGatewayDispatcher(t);
  const event = (e: MessageInitShape<typeof ServerFrameSchema>) => {
    dispatch(create(ServerFrameSchema, e));
  };
  return { t, event };
}

describe('createGatewayDispatcher', () => {
  it('resets every store from Ready', () => {
    const { t, event } = setup();
    event({
      kind: {
        case: 'ready',
        value: {
          me: { email: 'a@x.io' },
          instance: { name: 'Hub' },
          guilds: [
            { guild: { id: '1' }, presences: [{ userId: '7', status: PresenceStatus.ONLINE }] },
            { guild: { id: '2' }, presences: [{ userId: '8', status: PresenceStatus.DND }] },
          ],
        },
      },
    });
    expect(t.entities).toHaveBeenCalledWith(expect.objectContaining({ type: 'reset' }));
    expect(t.presence.mock.calls[0]?.[0]).toMatchObject({
      type: 'reset',
      presences: [{ userId: '7' }, { userId: '8' }],
    });
    expect(t.me).toHaveBeenCalledWith(expect.objectContaining({ email: 'a@x.io' }));
    expect(t.ready).toHaveBeenCalledWith(expect.objectContaining({ name: 'Hub' }));
  });

  it('maps events to store events', () => {
    const { t, event } = setup();
    const dispatchOf = (e: MessageInitShape<typeof EventSchema>) => {
      event({ kind: { case: 'dispatch', value: { seq: 1n, event: e } } });
    };
    dispatchOf({ kind: { case: 'channelDelete', value: { guildId: '1', channelId: '5' } } });
    dispatchOf({
      kind: { case: 'guildDelete', value: { guildId: '1', reason: GuildDeleteReason.KICKED } },
    });
    dispatchOf({ kind: { case: 'banAdd', value: { guildId: '1' } } });
    dispatchOf({
      kind: { case: 'presenceUpdate', value: { userId: '7', status: PresenceStatus.IDLE } },
    });
    dispatchOf({
      kind: { case: 'exposureChanged', value: { url: 'https://team.example.ts.net' } },
    });
    expect(t.exposureChanged).toHaveBeenCalledWith(
      expect.objectContaining({ url: 'https://team.example.ts.net' }),
    );
    expect(t.entities.mock.calls.map((c) => (c[0] as { type: string }).type)).toEqual([
      'channelRemoved',
      'guildRemoved',
    ]);
    expect(t.guildLost).toHaveBeenCalledWith('1', GuildDeleteReason.KICKED);
    expect(t.listChanged).toHaveBeenCalledWith('bans', '1');
    expect(t.presence).toHaveBeenCalledWith({
      type: 'upserted',
      presences: [expect.objectContaining({ userId: '7' })],
    });
  });

  it('feeds voice states before telling the voice controller', () => {
    const { t, event } = setup();
    const order: string[] = [];
    t.voiceStates.mockImplementation(() => order.push('store'));
    t.voiceUpdated.mockImplementation(() => order.push('controller'));
    event({
      kind: {
        case: 'dispatch',
        value: {
          seq: 1n,
          event: {
            kind: {
              case: 'voiceStateUpdate',
              value: { userId: '7', guildId: '1', channelId: '9' },
            },
          },
        },
      },
    });
    event({
      kind: {
        case: 'dispatch',
        value: { seq: 2n, event: { kind: { case: 'voiceForced', value: { guildId: '1' } } } },
      },
    });
    expect(order).toEqual(['store', 'controller']);
    expect(t.voiceForced).toHaveBeenCalledWith(expect.objectContaining({ guildId: '1' }));
  });
});
