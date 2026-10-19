import { useSyncExternalStore } from 'react';

function subscribe(onChange: () => void): () => void {
  window.addEventListener('focus', onChange);
  window.addEventListener('blur', onChange);
  document.addEventListener('visibilitychange', onChange);
  return () => {
    window.removeEventListener('focus', onChange);
    window.removeEventListener('blur', onChange);
    document.removeEventListener('visibilitychange', onChange);
  };
}

function focused(): boolean {
  return !document.hidden && document.hasFocus();
}

/** Whether this window is visible and has the focus. */
export function useWindowFocused(): boolean {
  return useSyncExternalStore(subscribe, focused);
}
