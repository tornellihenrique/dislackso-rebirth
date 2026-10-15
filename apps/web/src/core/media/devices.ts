/** The browser's default device; selecting it follows the system choice. */
export const DEFAULT_DEVICE = 'default';

export interface AudioDevice {
  readonly id: string;
  readonly label: string;
}

export interface AudioDevices {
  readonly inputs: readonly AudioDevice[];
  readonly outputs: readonly AudioDevice[];
}

type MediaDevicesLike = Pick<
  MediaDevices,
  'enumerateDevices' | 'addEventListener' | 'removeEventListener'
>;

/** Lists microphones and speakers. Labels are empty until a mic permission was granted. */
export async function listAudioDevices(
  md: MediaDevicesLike = navigator.mediaDevices,
): Promise<AudioDevices> {
  const all = await md.enumerateDevices();
  const pick = (kind: MediaDeviceKind) =>
    all
      .filter((d) => d.kind === kind && d.deviceId !== '' && d.deviceId !== 'communications')
      .map((d) => ({ id: d.deviceId, label: d.label }));
  return { inputs: pick('audioinput'), outputs: pick('audiooutput') };
}

/** The device to use: the chosen one while it is plugged in, else the default. */
export function resolveDevice(chosen: string, available: readonly AudioDevice[]): string {
  return chosen !== '' && available.some((d) => d.id === chosen) ? chosen : DEFAULT_DEVICE;
}

/** Calls onChange when a device is plugged in or removed. */
export function watchDevices(
  onChange: () => void,
  md: MediaDevicesLike = navigator.mediaDevices,
): () => void {
  md.addEventListener('devicechange', onChange);
  return () => {
    md.removeEventListener('devicechange', onChange);
  };
}

/** Whether the browser can route audio to a chosen output (not Firefox or Safari today). */
export function canChooseOutput(): boolean {
  return typeof HTMLMediaElement !== 'undefined' && 'setSinkId' in HTMLMediaElement.prototype;
}
