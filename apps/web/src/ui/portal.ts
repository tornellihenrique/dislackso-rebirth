import { createContext, useContext } from 'react';

/**
 * Where menus and tooltips render. Null means the page body; the stage
 * points it at itself while fullscreen, since nothing outside it shows then.
 */
export const PortalContainer = createContext<HTMLElement | null>(null);

export function usePortalContainer(): HTMLElement | null {
  return useContext(PortalContainer);
}
