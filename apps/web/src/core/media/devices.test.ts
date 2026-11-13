import { describe, expect, it } from 'vitest';

import {
  DEFAULT_DEVICE,
  deviceConstraint,
  deviceMissing,
  listAudioDevices,
  listCameras,
  resolveDevice,
} from './devices';

describe('deviceConstraint', () => {
  it('holds a chosen device exactly and leaves the default to the browser', () => {
    expect(deviceConstraint('usb')).toEqual({ exact: 'usb' });
    expect(deviceConstraint('')).toBeUndefined();
    expect(deviceConstraint(DEFAULT_DEVICE)).toBeUndefined();
  });

  it('tells a device gone from other failures', () => {
    const named = (name: string) => Object.assign(new Error(name), { name });
    expect(deviceMissing(named('OverconstrainedError'))).toBe(true);
    expect(deviceMissing(named('NotFoundError'))).toBe(true);
    expect(deviceMissing(named('NotAllowedError'))).toBe(false);
  });
});

describe('resolveDevice', () => {
  const plugged = [
    { id: 'default', label: 'Default' },
    { id: 'usb', label: 'USB headset' },
  ];

  it('keeps the chosen device while it is plugged in and falls back to the default', () => {
    expect(resolveDevice('usb', plugged)).toBe('usb');
    expect(resolveDevice('usb', plugged.slice(0, 1))).toBe(DEFAULT_DEVICE);
    expect(resolveDevice('', plugged)).toBe(DEFAULT_DEVICE);
  });
});

describe('listAudioDevices and listCameras', () => {
  it('sort devices by kind and skip unusable entries', async () => {
    const devices = [
      { kind: 'audioinput', deviceId: 'default', label: 'Default' },
      { kind: 'audioinput', deviceId: 'communications', label: 'Communications' },
      { kind: 'audioinput', deviceId: '', label: '' },
      { kind: 'audiooutput', deviceId: 'spk', label: 'Speakers' },
      { kind: 'videoinput', deviceId: 'cam', label: 'Camera' },
    ] as MediaDeviceInfo[];
    const md = {
      enumerateDevices: () => Promise.resolve(devices),
      addEventListener: () => undefined,
      removeEventListener: () => undefined,
    };
    await expect(listAudioDevices(md)).resolves.toEqual({
      inputs: [{ id: 'default', label: 'Default' }],
      outputs: [{ id: 'spk', label: 'Speakers' }],
    });
    await expect(listCameras(md)).resolves.toEqual([{ id: 'cam', label: 'Camera' }]);
  });
});
