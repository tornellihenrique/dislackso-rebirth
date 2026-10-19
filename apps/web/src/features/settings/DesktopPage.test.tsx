import type { DesktopBridge, OverlayApi, ScreenApi, UpdateCheck } from '@dislackso/desktop-bridge';
import { screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';

import { renderWithProviders } from '@/test/render';

import { DesktopPage } from './DesktopPage';

function bridge(check: () => Promise<UpdateCheck>): DesktopBridge {
  return {
    bridgeVersion: 1,
    platform: 'win32',
    screen: {} as ScreenApi,
    overlay: {} as OverlayApi,
    updates: { version: () => Promise.resolve('0.7.0'), check },
  };
}

describe('DesktopPage', () => {
  it('shows the version and what a check found', async () => {
    const user = userEvent.setup();
    const check = vi
      .fn<() => Promise<UpdateCheck>>()
      .mockResolvedValueOnce({ status: 'downloading', version: '0.8.0' })
      .mockRejectedValueOnce(new Error('gone'));
    await renderWithProviders(<DesktopPage />, { services: { desktop: bridge(check) } });

    expect(await screen.findByText('Version 0.7.0')).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Check for updates' }));
    expect(await screen.findByRole('status')).toHaveTextContent(
      "Version 0.8.0 is downloading. You'll be asked to restart when it's ready.",
    );
    await user.click(screen.getByRole('button', { name: 'Check for updates' }));
    expect(await screen.findByRole('status')).toHaveTextContent("Couldn't check for updates.");
  });

  it('says where the settings are in a browser', async () => {
    await renderWithProviders(<DesktopPage />);
    expect(screen.getByText('These settings are in the desktop app.')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Check for updates' })).not.toBeInTheDocument();
  });
});
