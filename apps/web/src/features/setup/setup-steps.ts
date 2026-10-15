export const SETUP_STEPS = ['instance', 'admin', 'email', 'connectivity'] as const;
export type SetupStep = (typeof SETUP_STEPS)[number];

/** The wizard step that holds a field the hub reported as invalid. */
export function stepForField(field: string): SetupStep {
  if (field === 'instance_name' || field === 'public_url') return 'instance';
  if (field.startsWith('admin_')) return 'admin';
  if (field.startsWith('smtp.')) return 'email';
  return 'instance';
}

/** The earliest step among the invalid fields, or undefined if there are none. */
export function firstInvalidStep(fields: readonly string[]): SetupStep | undefined {
  const steps = new Set(fields.map(stepForField));
  return SETUP_STEPS.find((s) => steps.has(s));
}
