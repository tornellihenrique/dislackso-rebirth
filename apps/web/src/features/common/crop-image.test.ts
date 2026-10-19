import { describe, expect, it } from 'vitest';

import { previewStyle } from './crop-image';

describe('previewStyle', () => {
  it('scales and places the crop so it fills the preview', () => {
    expect(previewStyle('blob:x', { x: 25, y: 0, width: 50, height: 100 })).toEqual({
      backgroundImage: 'url("blob:x")',
      backgroundSize: '200% 100%',
      backgroundPosition: '50% 0%',
    });
    expect(previewStyle('blob:x', { x: 50, y: 60, width: 50, height: 40 }).backgroundPosition).toBe(
      '100% 100%',
    );
  });
});
