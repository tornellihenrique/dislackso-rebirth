import { BRIDGE_VERSION, type DesktopBridge } from '@dislackso/desktop-bridge';

/**
 * The desktop shell's bridge when the app runs inside it (ARCHITECTURE
 * §16.3); undefined in a browser, or in a shell of another bridge version.
 */
export function desktopBridge(
  scope: { readonly desktop?: unknown } = globalThis as { readonly desktop?: unknown },
): DesktopBridge | undefined {
  const d = scope.desktop;
  if (typeof d !== 'object' || d === null || !('bridgeVersion' in d)) return undefined;
  return d.bridgeVersion === BRIDGE_VERSION ? (d as DesktopBridge) : undefined;
}
