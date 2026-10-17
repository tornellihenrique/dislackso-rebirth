import { create } from '@bufbuild/protobuf';
import {
  CandidateType,
  InstanceInfoSchema,
  VoiceForcedAction,
  VoiceForcedSchema,
} from '@dislackso/contracts/gen/dislackso/gateway/v1/gateway_pb';
import { ChannelSchema } from '@dislackso/contracts/gen/dislackso/v1/channel_pb';
import {
  MeSchema,
  StatusPreference,
  VoiceStateSchema,
} from '@dislackso/contracts/gen/dislackso/v1/common_pb';
import type { TFunction } from 'i18next';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import type { HubClients } from '@/core/api/clients';
import type { VoiceSessionListener } from '@/core/media/voice-session';
import { callStore, idleCall } from '@/stores/call';
import { entitiesStore } from '@/stores/entities';
import { noticesStore } from '@/stores/notices';
import { dispatchRealtime } from '@/stores/realtime';
import { dispatchSession, initialSessionState, sessionStore } from '@/stores/session';
import { defaultShareSettings, shareSettingsStore } from '@/stores/share-settings';
import { defaultVoiceSettings, voiceSettingsStore } from '@/stores/voice-settings';
import { dispatchVoiceStates, voiceStatesStore } from '@/stores/voice-states';

import { VoiceController } from './voice';

/** The media session, recording what the controller asks of it. */
class FakeSession {
  static last: FakeSession | undefined;
  readonly calls: string[] = [];
  constructor(readonly listener: VoiceSessionListener) {
    FakeSession.last = this;
  }
  connect(
    target: { url: string; token: string },
    _prefs: unknown,
    local: { muted: boolean; deafened: boolean; micAllowed: boolean },
  ) {
    this.calls.push(
      `connect ${target.url} ${target.token} muted=${String(local.muted)} deaf=${String(local.deafened)}`,
    );
    return Promise.resolve();
  }
  disconnect() {
    this.calls.push('disconnect');
    return Promise.resolve();
  }
  setMuted(m: boolean) {
    this.calls.push(`muted=${String(m)}`);
  }
  setDeafened(d: boolean) {
    this.calls.push(`deaf=${String(d)}`);
  }
  setMicAllowed(a: boolean) {
    this.calls.push(`micAllowed=${String(a)}`);
  }
  setPushToTalk() {
    this.calls.push('setPushToTalk');
  }
  setOutputVolume() {
    this.calls.push('setOutputVolume');
  }
  setUserVolume() {
    this.calls.push('setUserVolume');
  }
  setUserMuted() {
    this.calls.push('setUserMuted');
  }
  updatePrefs() {
    return Promise.resolve();
  }
  startAudio() {
    return Promise.resolve();
  }
  setDeafenStreams() {
    return undefined;
  }
  setStreamVolume(identity: string, volume: number) {
    this.calls.push(`streamVolume ${identity}=${String(volume)}`);
  }
  startShare(req: { preset: string }) {
    this.calls.push(`startShare ${req.preset}`);
    return Promise.resolve({
      trackSid: 'TR_self',
      codec: 'vp9',
      audio: false,
      ownAudioWarning: false,
    });
  }
  stopShare() {
    this.calls.push('stopShare');
    return Promise.resolve();
  }
  readings = new Map<string, unknown>();
  transportReadings() {
    return Promise.resolve(this.readings);
  }
}

vi.mock('@/core/media/voice-session', () => ({ VoiceSession: FakeSession }));

const state = (session: string, channelId: string, extra: Record<string, unknown> = {}) =>
  create(VoiceStateSchema, {
    userId: '1',
    guildId: 'g',
    channelId,
    voiceSessionId: session,
    ...extra,
  });

function setup() {
  let n = 0;
  const join = vi.fn((req: { channelId: string }) => {
    n++;
    return Promise.resolve({
      url: '/lk',
      token: `token-${String(n)}`,
      iceServers: [],
      state: state(`s${String(n)}`, req.channelId),
      limits: { micBitrateMaxKbps: 64 },
    });
  });
  const leave = vi.fn(() => Promise.resolve({}));
  const hub = { voice: { join, leave } } as unknown as HubClients;
  const played: string[] = [];
  const gateway = { setVoiceSelf: vi.fn(), sendTelemetry: vi.fn() };
  const t = ((key: string) => key) as unknown as TFunction;
  const host = document.createElement('div');
  const voice = new VoiceController({ hub, sounds: { play: (k) => played.push(k) }, t, host });
  voice.attachGateway(gateway);
  const notices = () =>
    noticesStore.getState().items.map((i) => (i.kind === 'message' ? i.text : i.kind));
  return { voice, join, leave, gateway, played, notices, session: () => FakeSession.last };
}

beforeEach(() => {
  dispatchSession({
    type: 'signedIn',
    me: create(MeSchema, { user: { id: '1' }, status: StatusPreference.ONLINE }),
  });
  voiceSettingsStore.setState(defaultVoiceSettings, true);
});

afterEach(() => {
  callStore.setState(idleCall, true);
  noticesStore.setState({ items: [], nextId: 1 }, true);
  sessionStore.setState(initialSessionState, true);
  voiceStatesStore.setState({}, true);
  shareSettingsStore.setState(defaultShareSettings, true);
  dispatchRealtime({ type: 'cleared' });
  FakeSession.last = undefined;
});

describe('VoiceController', () => {
  it('joins through the hub and connects with the token', async () => {
    const { voice, join, gateway, played, session } = setup();
    await voice.join('g', 'c1');
    expect(join).toHaveBeenCalledWith({ channelId: 'c1', selfMute: false, selfDeaf: false });
    expect(session()?.calls).toContain('connect /lk token-1 muted=false deaf=false');
    expect(callStore.getState()).toMatchObject({
      status: 'connected',
      channelId: 'c1',
      sessionId: 's1',
    });
    expect(gateway.setVoiceSelf).toHaveBeenCalledWith('s1', false, false);
    expect(played).toEqual(['join']);
  });

  it('reports what the call carried every 30 s, and stops when it ends', async () => {
    vi.useFakeTimers();
    try {
      const { voice, gateway, session } = setup();
      await voice.join('g', 'c1');
      const path = (candidate: string) => ({ candidate, protocol: 'udp', rttMs: 30 });
      const readings = session()?.readings;
      readings?.set('publisher', { path: path('host'), bytesSent: 1000, bytesReceived: 10 });
      readings?.set('subscriber', { path: path('relay'), bytesSent: 20, bytesReceived: 5000 });
      await vi.advanceTimersByTimeAsync(30_000);
      expect(gateway.sendTelemetry).toHaveBeenCalledWith({
        voiceSessionId: 's1',
        candidate: CandidateType.RELAY,
        bytesSent: 1020n,
        bytesReceived: 5010n,
        rttMs: 30,
      });
      await voice.leave();
      await vi.advanceTimersByTimeAsync(60_000);
      expect(gateway.sendTelemetry).toHaveBeenCalledTimes(1);
    } finally {
      vi.useRealTimers();
    }
  });

  it('deafens in a call and tells the hub', async () => {
    const { voice, gateway, played, session } = setup();
    await voice.join('g', 'c1');
    voice.toggleDeafen();
    expect(session()?.calls.slice(-2)).toEqual(['muted=true', 'deaf=true']);
    expect(gateway.setVoiceSelf).toHaveBeenLastCalledWith('s1', true, true);
    expect(played.at(-1)).toBe('deafen');
  });

  it('ends the call when another device takes over or the hub ends it', async () => {
    const { voice, notices, session } = setup();
    await voice.join('g', 'c1');
    voice.voiceUpdated(state('s1', 'c1', { serverMute: true }));
    expect(session()?.calls.at(-1)).toBe('micAllowed=false');

    voice.voiceUpdated(state('elsewhere', 'c1'));
    expect(callStore.getState().status).toBe('idle');
    expect(notices()).toEqual(['voice.notices.anotherDevice']);

    await voice.join('g', 'c1');
    voice.voiceUpdated(state('s2', ''));
    expect(callStore.getState().status).toBe('idle');
    expect(session()?.calls.at(-1)).toBe('disconnect');
  });

  it('follows a move to the new channel', async () => {
    const { voice, join, notices } = setup();
    await voice.join('g', 'c1');
    voice.voiceForced(
      create(VoiceForcedSchema, {
        action: VoiceForcedAction.MOVED,
        guildId: 'g',
        channelId: 'c2',
        voiceSessionId: 's1',
      }),
    );
    await vi.waitFor(() => {
      expect(callStore.getState()).toMatchObject({ status: 'connected', channelId: 'c2' });
    });
    expect(join).toHaveBeenLastCalledWith({ channelId: 'c2', selfMute: false, selfDeaf: false });
    expect(notices()).toEqual(['voice.notices.movedBy']);
  });

  it('ignores the old room ending while switching, and leaves the hub when media fails', async () => {
    const { voice, join, leave, notices, session } = setup();
    await voice.join('g', 'c1');
    let release = (): void => undefined;
    join.mockImplementationOnce((req: { channelId: string }) => {
      // The hub removes the old connection before answering.
      session()?.listener.onEnded('removed');
      return new Promise((resolve) => {
        release = () => {
          resolve({
            url: '/lk',
            token: 'token-x',
            iceServers: [],
            state: state('s9', req.channelId),
            limits: { micBitrateMaxKbps: 64 },
          });
        };
      });
    });
    const switching = voice.join('g', 'c2');
    release();
    await switching;
    expect(callStore.getState()).toMatchObject({ status: 'connected', channelId: 'c2' });

    session()?.listener.onEnded('failed');
    expect(callStore.getState().status).toBe('idle');
    expect(notices()).toEqual(['voice.notices.connectionLost']);
    expect(leave).toHaveBeenCalledWith({ voiceSessionId: 's9' });
  });

  it('shares with the saved settings, capped by the instance and the channel', async () => {
    const { voice, played, session } = setup();
    dispatchRealtime({
      type: 'ready',
      instance: create(InstanceInfoSchema, { maxSharePreset: '1440p30', maxSharesPerChannel: 2 }),
    });
    entitiesStore.setState((s) => ({
      ...s,
      channels: {
        ...s.channels,
        c1: create(ChannelSchema, { id: 'c1', guildId: 'g', maxShareQuality: '1080p60' }),
      },
    }));
    shareSettingsStore.setState({ ...defaultShareSettings, preset: '1440p60' }, true);
    await voice.join('g', 'c1');
    await voice.startShare();
    expect(session()?.calls.at(-1)).toBe('startShare 1080p60');
    expect(callStore.getState().localShare).toMatchObject({ status: 'live', preset: '1080p60' });
    expect(played.at(-1)).toBe('streamStart');

    // Stopped with the browser's own button: no message, just the sound.
    session()?.listener.onShareEnded?.('ended');
    expect(callStore.getState().localShare).toBeNull();
    expect(played.at(-1)).toBe('streamStop');
  });

  it('keeps to the share limit and says why the media server stopped a share', async () => {
    const { voice, notices, session } = setup();
    dispatchRealtime({
      type: 'ready',
      instance: create(InstanceInfoSchema, { maxSharesPerChannel: 1 }),
    });
    await voice.join('g', 'c1');
    const other = (id: string, session: string, channelId: string, streaming: boolean) => {
      dispatchVoiceStates({
        type: 'updated',
        state: state(session, channelId, { userId: id, streaming }),
      });
    };
    other('2', 'o2', 'c1', true);
    await voice.startShare();
    expect(session()?.calls.some((c) => c.startsWith('startShare'))).toBe(false);
    expect(notices()).toEqual(['stage.notices.shareLimit']);

    other('2', 'o2', '', false);
    await voice.startShare();
    expect(callStore.getState().localShare?.status).toBe('live');
    other('3', 'o3', 'c1', true);
    session()?.listener.onShareEnded?.('removed');
    expect(callStore.getState().localShare).toBeNull();
    expect(notices().at(-1)).toBe('stage.notices.shareLimit');
  });
});
