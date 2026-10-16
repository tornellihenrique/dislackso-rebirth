import { describe, expect, it, vi } from 'vitest';

import { ShareViews, type AttachableTrack } from './share-views';

function fakeTrack(): AttachableTrack & { attached: HTMLMediaElement[] } {
  const attached: HTMLMediaElement[] = [];
  return {
    attached,
    attach: (el) => {
      attached.push(el);
      return el;
    },
    detach: (el) => {
      attached.splice(attached.indexOf(el), 1);
      return el;
    },
  };
}

describe('ShareViews', () => {
  it('moves one element between containers, so a moved tile keeps its picture', () => {
    vi.spyOn(HTMLMediaElement.prototype, 'play').mockResolvedValue(undefined);
    const views = new ShareViews();
    const track = fakeTrack();
    views.setTrack('alice', track);
    const grid = document.createElement('div');
    const focus = document.createElement('div');
    document.body.append(grid, focus);

    const unmountGrid = views.mount('alice', grid);
    const el = grid.querySelector('video');
    expect(el).not.toBeNull();
    expect(track.attached).toEqual([el]);

    // React mounts the tile's new place while the old one is still there.
    const unmountFocus = views.mount('alice', focus);
    expect(focus.querySelector('video')).toBe(el);
    unmountGrid();
    expect(focus.querySelector('video')).toBe(el);
    expect(track.attached).toHaveLength(1);

    // Unmounting the latest place gives the element back to the earlier one.
    const again = views.mount('alice', grid);
    unmountFocus();
    expect(grid.querySelector('video')).toBe(el);
    again();
    expect(el?.isConnected).toBe(false);
  });

  it('shows a track that arrives after the tile, and forgets a share nobody shows', () => {
    const views = new ShareViews();
    const box = document.createElement('div');
    const unmount = views.mount('bob', box);
    const el = views.element('bob');
    const track = fakeTrack();
    views.setTrack('bob', track);
    expect(track.attached).toEqual([el]);
    views.setTrack('bob', undefined);
    expect(track.attached).toEqual([]);
    unmount();
    expect(views.element('bob')).toBeUndefined();
  });

  it('detaches every track at the end of a call but keeps the elements of mounted tiles', () => {
    const views = new ShareViews();
    const box = document.createElement('div');
    views.mount('cy', box);
    const track = fakeTrack();
    views.setTrack('cy', track);
    views.setTrack('dee', fakeTrack());
    views.detachAll();
    expect(track.attached).toEqual([]);
    expect(box.querySelector('video')).toBe(views.element('cy'));
    expect(views.element('dee')).toBeUndefined();
  });
});
