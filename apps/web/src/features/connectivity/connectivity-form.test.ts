import { create } from '@bufbuild/protobuf';
import {
  DynamicDnsKind,
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
  nearBudget,
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

  it('keeps a DuckDNS name and never shows the stored DNS token', () => {
    const form = exposureFormFrom(
      create(ExposureSettingsSchema, {
        kind: ExposureKind.DIRECT,
        directDomain: 'team.duckdns.org',
        dynamicDns: DynamicDnsKind.DUCKDNS,
        dnsTokenSet: true,
      }),
    );
    expect(form).toMatchObject({ choice: 'direct', dynamicDns: 'duckdns', dnsToken: '' });
    expect(
      exposureSettingsFrom({ ...form, directDomain: ' Team.DuckDNS.org ', dnsToken: ' abc ' }),
    ).toMatchObject({
      kind: ExposureKind.DIRECT,
      directDomain: 'team.duckdns.org',
      dynamicDns: DynamicDnsKind.DUCKDNS,
      dnsToken: 'abc',
    });
  });

  it('starts local when nothing is configured', () => {
    expect(exposureFormFrom(undefined)).toMatchObject({ choice: 'local', dynamicDns: 'none' });
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

describe('nearBudget', () => {
  it('warns from 90 % of a budget, and never without one', () => {
    expect(nearBudget({ monthBytes: 899_000_000_000n, budgetGb: 1000 })).toBe(false);
    expect(nearBudget({ monthBytes: 900_000_000_000n, budgetGb: 1000 })).toBe(true);
    expect(nearBudget({ monthBytes: 5_000_000_000_000n, budgetGb: 0 })).toBe(false);
  });
});

describe('gigabytes', () => {
  it('rounds to one decimal', () => {
    expect(gigabytes(1_234_567_890n)).toBe(1.2);
    expect(gigabytes(0n)).toBe(0);
  });
});
