import { create } from '@bufbuild/protobuf';
import {
  ExposureKind,
  ExposureSettingsSchema,
  TurnKind,
  TurnSettingsSchema,
} from '@dislackso/contracts/gen/dislackso/v1/connectivity_pb';
import { describe, expect, it } from 'vitest';

import {
  exposureFormFrom,
  exposureSettingsFrom,
  gigabytes,
  relayFormFrom,
  turnSettingsFrom,
} from './connectivity-form';

describe('exposure form', () => {
  it('round-trips the settings and never shows the stored token', () => {
    const settings = create(ExposureSettingsSchema, {
      kind: ExposureKind.CLOUDFLARE_TUNNEL,
      tunnelUrl: 'https://chat.example.com',
      tunnelTokenSet: true,
      lanAddr: '0.0.0.0:7443',
    });
    const form = exposureFormFrom(settings);
    expect(form).toMatchObject({
      choice: 'tunnel',
      tunnelToken: '',
      tunnelUrl: 'https://chat.example.com',
    });
    expect(exposureSettingsFrom({ ...form, tailscaleHostname: ' Team ' })).toMatchObject({
      kind: ExposureKind.CLOUDFLARE_TUNNEL,
      tailscaleHostname: 'team',
      tunnelToken: '',
      lanAddr: '0.0.0.0:7443',
    });
  });

  it('starts local when nothing is configured', () => {
    expect(exposureFormFrom(undefined).choice).toBe('local');
  });
});

describe('relay form', () => {
  it('turns one URL per line into the list and keeps the stored secret when empty', () => {
    const form = relayFormFrom(
      create(TurnSettingsSchema, {
        kind: TurnKind.STATIC,
        urls: ['turn:a:3478'],
        monthlyBudgetGb: 900,
      }),
    );
    expect(form).toMatchObject({ choice: 'static', urls: 'turn:a:3478', budgetGb: '900' });
    const out = turnSettingsFrom({
      ...form,
      urls: ' turn:a:3478 \n\nturns:b:5349\n',
      username: 'u',
    });
    expect(out).toMatchObject({
      kind: TurnKind.STATIC,
      urls: ['turn:a:3478', 'turns:b:5349'],
      secret: '',
      monthlyBudgetGb: 900,
    });
  });

  it('sends a budget the hub rejects when the field is not a number', () => {
    expect(
      turnSettingsFrom({ ...relayFormFrom(undefined), budgetGb: 'lots' }).monthlyBudgetGb,
    ).toBe(-1);
  });
});

describe('gigabytes', () => {
  it('rounds to one decimal', () => {
    expect(gigabytes(1_234_567_890n)).toBe(1.2);
    expect(gigabytes(0n)).toBe(0);
  });
});
