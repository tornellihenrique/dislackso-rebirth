import { afterEach, describe, expect, it, vi } from 'vitest';

import { guardDragClick, releaseDragClick } from './drag-click';

function click(el: HTMLElement): boolean {
  const e = new MouseEvent('click', { bubbles: true, cancelable: true });
  el.dispatchEvent(e);
  return e.defaultPrevented;
}

afterEach(() => {
  vi.useRealTimers();
});

describe('drag click guard', () => {
  it('cancels the click that ends a drag, and only that one', () => {
    vi.useFakeTimers();
    const link = document.createElement('a');
    link.href = '/channels/1/2';
    document.body.append(link);
    // dnd-kit stops the click on the document; the guard must act before it.
    document.addEventListener(
      'click',
      (e) => {
        e.stopPropagation();
      },
      { capture: true, once: true },
    );

    guardDragClick();
    releaseDragClick();
    expect(click(link)).toBe(true);
    vi.advanceTimersByTime(60);
    expect(click(link)).toBe(false);
    link.remove();
  });
});
