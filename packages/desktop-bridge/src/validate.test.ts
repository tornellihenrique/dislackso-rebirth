import { describe, expect, it } from 'vitest';

import {
  isSourceId,
  parseHotkeyBindings,
  parseTrayVoiceState,
  parseOverlayAction,
  parseOverlayInput,
  parseOverlayOpen,
  parseOverlayState,
  parsePacket,
} from './validate';

const labels = {
  draw: 'Draw',
  stopDrawing: 'Stop',
  clearAll: 'Clear',
  stopOthers: 'Lock',
  letOthers: 'Unlock',
  stopSharing: 'Stop sharing',
};

describe('overlay input', () => {
  it('keeps points inside the content and refuses anything else', () => {
    expect(parseOverlayInput({ kind: 'begin', point: { x: 1.5, y: -2 } })).toEqual({
      kind: 'begin',
      point: { x: 1, y: 0 },
    });
    expect(parseOverlayInput({ kind: 'move', points: [{ x: 0.1, y: 0.2 }] })).toEqual({
      kind: 'move',
      points: [{ x: 0.1, y: 0.2 }],
    });
    expect(parseOverlayInput({ kind: 'end' })).toEqual({ kind: 'end' });
    expect(parseOverlayInput({ kind: 'move', points: [{ x: Number.NaN, y: 0 }] })).toBeUndefined();
    expect(
      parseOverlayInput({ kind: 'move', points: Array(600).fill({ x: 0, y: 0 }) }),
    ).toBeUndefined();
    expect(parseOverlayInput({ kind: 'erase' })).toBeUndefined();
    expect(parseOverlayInput('begin')).toBeUndefined();
  });

  it('knows only the toolbar actions', () => {
    expect(parseOverlayAction('clearAll')).toBe('clearAll');
    expect(parseOverlayAction('rm -rf')).toBeUndefined();
  });
});

describe('overlay open and state', () => {
  it('needs a share, its owner and every label', () => {
    expect(parseOverlayOpen({ trackSid: 'TR_1', owner: '42', labels })).toEqual({
      trackSid: 'TR_1',
      owner: '42',
      labels,
      names: {},
      nameTags: true,
      othersAllowed: true,
      holdMaxMs: 0,
    });
    expect(parseOverlayOpen({ trackSid: 'TR_1', owner: '42', labels: {} })).toBeUndefined();
    expect(parseOverlayOpen({ trackSid: '', owner: '42', labels })).toBeUndefined();
  });

  it('takes partial state and drops malformed names', () => {
    expect(parseOverlayState({ othersAllowed: false })).toEqual({ othersAllowed: false });
    expect(parseOverlayState({ names: { '1': 'Ana', '2': 7 } })).toEqual({ names: { '1': 'Ana' } });
    expect(parseOverlayState({ nameTags: 'yes' })).toBeUndefined();
    expect(parseOverlayState({ holdMaxMs: 1e9 })).toEqual({ holdMaxMs: 60_000 });
  });
});

describe('sources and packets', () => {
  it('accept capture source ids and bounded packets only', () => {
    expect(isSourceId('screen:0:0')).toBe(true);
    expect(isSourceId('window:132412:0')).toBe(true);
    expect(isSourceId('file:///etc/passwd')).toBe(false);
    expect(parsePacket(new Uint8Array([1, 2]))).toEqual(new Uint8Array([1, 2]));
    expect(parsePacket(new Uint8Array(20_000))).toBeUndefined();
    expect(parsePacket('bytes')).toBeUndefined();
  });
});

describe('global keys', () => {
  it('keep well-formed bindings and refuse the rest', () => {
    const mute = { action: 'toggleMute', keys: { code: 'KeyM', ctrl: true, shift: true } };
    expect(parseHotkeyBindings([mute])).toEqual([
      {
        action: 'toggleMute',
        keys: { code: 'KeyM', ctrl: true, shift: true, alt: false, meta: false },
      },
    ]);
    expect(parseHotkeyBindings([{ ...mute, action: 'quit' }])).toBeUndefined();
    expect(parseHotkeyBindings([{ ...mute, keys: { code: 'Key M' } }])).toBeUndefined();
    expect(parseHotkeyBindings(Array.from({ length: 17 }, () => mute))).toBeUndefined();
    expect(parseHotkeyBindings('KeyM')).toBeUndefined();
  });

  it('take the tray state only whole', () => {
    expect(parseTrayVoiceState({ muted: true, deafened: false, inCall: true })).toEqual({
      muted: true,
      deafened: false,
      inCall: true,
    });
    expect(parseTrayVoiceState({ muted: true })).toBeUndefined();
  });
});
