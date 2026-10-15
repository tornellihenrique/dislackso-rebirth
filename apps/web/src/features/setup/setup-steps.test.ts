import { describe, expect, it } from 'vitest';

import { firstInvalidStep, stepForField } from './setup-steps';

describe('setup steps', () => {
  it.each([
    ['instance_name', 'instance'],
    ['public_url', 'instance'],
    ['admin_username', 'admin'],
    ['admin_password', 'admin'],
    ['smtp.host', 'email'],
    ['something_new', 'instance'],
  ])('%s belongs to %s', (field, step) => {
    expect(stepForField(field)).toBe(step);
  });

  it('picks the earliest step with an error', () => {
    expect(firstInvalidStep(['smtp.host', 'admin_email'])).toBe('admin');
    expect(firstInvalidStep([])).toBeUndefined();
  });
});
