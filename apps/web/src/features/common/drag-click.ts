/**
 * dnd-kit swallows the click that ends a drag by stopping its propagation
 * only: React never sees it, but the browser still follows the link under
 * the pointer, which is the dragged item itself, and loads the whole app
 * again. This cancels the click's default too, from window, which sees it
 * before dnd-kit's listener on the document, for the same 50 ms after the
 * drag that dnd-kit keeps its own.
 */

const AFTER_DRAG_MS = 50;

function cancel(e: MouseEvent): void {
  e.preventDefault();
}

/** Call when a drag starts. */
export function guardDragClick(): void {
  window.addEventListener('click', cancel, true);
}

/** Call when a drag ends or is cancelled; the click that ends it follows. */
export function releaseDragClick(): void {
  setTimeout(() => {
    window.removeEventListener('click', cancel, true);
  }, AFTER_DRAG_MS);
}
