import { describe, expect, it } from 'vitest';

import { DEFAULT_DEVICE, listAudioDevices, resolveDevice } from './devices';

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

describe('listAudioDevices', () => {
  it('splits inputs from outputs and skips unusable entries', async () => {
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
  });
});
