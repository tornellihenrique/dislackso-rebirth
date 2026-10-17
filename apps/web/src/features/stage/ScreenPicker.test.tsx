import type {
  CaptureSource,
  DesktopBridge,
  OverlayApi,
  PickRequest,
} from '@dislackso/desktop-bridge';
import { act, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';

import { desktopBridge } from '@/core/platform/desktop';
import { renderWithProviders } from '@/test/render';

import { ScreenPicker } from './ScreenPicker';

const sources: CaptureSource[] = [
  { id: 'screen:0:0', name: 'Screen 1', kind: 'screen', thumbnail: 'data:,', appIcon: '' },
  { id: 'screen:1:0', name: 'Screen 2', kind: 'screen', thumbnail: 'data:,', appIcon: '' },
  { id: 'window:42:0', name: 'Unreal Editor', kind: 'window', thumbnail: 'data:,', appIcon: '' },
];

function fakeBridge() {
  let ask: ((r: PickRequest) => void) | undefined;
  const choose = vi.fn();
  const cancel = vi.fn();
  const bridge: DesktopBridge = {
    bridgeVersion: 1,
    platform: 'win32',
    screen: {
      getSources: () => Promise.resolve(sources),
      choose,
      cancel,
      onPickRequested: (cb) => {
        ask = cb;
        return () => undefined;
      },
    },
    overlay: {} as OverlayApi,
  };
  const request = (r: PickRequest) => {
    act(() => {
      ask?.(r);
    });
  };
  return { bridge, choose, cancel, request };
}

describe('ScreenPicker', () => {
  it('lists screens first, and shares the chosen window with the system sound', async () => {
    const user = userEvent.setup();
    const { bridge, choose, request } = fakeBridge();
    await renderWithProviders(<ScreenPicker desktop={bridge} />);
    request({ audioRequested: true, systemAudio: true });

    const dialog = await screen.findByRole('dialog', { name: 'Share your screen' });
    expect(await screen.findByRole('button', { name: 'Screen 1' })).toHaveAttribute(
      'aria-pressed',
      'true',
    );
    expect(screen.queryByRole('button', { name: 'Unreal Editor' })).not.toBeInTheDocument();
    await user.click(screen.getByRole('tab', { name: 'Windows' }));
    await user.click(screen.getByRole('button', { name: 'Unreal Editor' }));
    expect(screen.getByLabelText("Share your computer's sound")).toBeChecked();
    await user.click(screen.getByRole('button', { name: 'Share' }));

    expect(choose).toHaveBeenCalledWith('window:42:0', { audio: true });
    expect(dialog).not.toBeInTheDocument();
  });

  it('cancels when closed, and offers no sound where the app cannot capture it', async () => {
    const user = userEvent.setup();
    const { bridge, choose, cancel, request } = fakeBridge();
    await renderWithProviders(<ScreenPicker desktop={bridge} />);
    request({ audioRequested: true, systemAudio: false });

    await screen.findByRole('button', { name: 'Screen 1' });
    expect(screen.queryByLabelText("Share your computer's sound")).not.toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Cancel' }));
    expect(cancel).toHaveBeenCalledOnce();
    expect(choose).not.toHaveBeenCalled();
  });
});

describe('desktopBridge', () => {
  it('is there only in a desktop app with the same bridge version', () => {
    expect(desktopBridge({})).toBeUndefined();
    expect(desktopBridge({ desktop: { bridgeVersion: 99 } })).toBeUndefined();
    const { bridge } = fakeBridge();
    expect(desktopBridge({ desktop: bridge })).toBe(bridge);
  });
});
