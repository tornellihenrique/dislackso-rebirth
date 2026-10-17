import type { MessageInitShape } from '@bufbuild/protobuf';
import {
  DynamicDnsKind,
  ExposureKind,
  type ExposureSettings,
  type ExposureSettingsSchema,
  TurnKind,
  type TurnSettings,
  type TurnSettingsSchema,
} from '@dislackso/contracts/gen/dislackso/v1/connectivity_pb';

/** How the hub is reached, as the page names the choices (and their text keys). */
export type ExposureChoice = 'local' | 'tailscale' | 'quick' | 'tunnel' | 'direct' | 'lan';

/** In the order the page offers them: the free, stable option first. */
export const EXPOSURE_CHOICES: readonly ExposureChoice[] = [
  'tailscale',
  'quick',
  'tunnel',
  'direct',
  'lan',
  'local',
];

const exposureKinds: Record<ExposureChoice, ExposureKind> = {
  local: ExposureKind.LOCAL,
  tailscale: ExposureKind.TAILSCALE_FUNNEL,
  quick: ExposureKind.CLOUDFLARE_QUICK,
  tunnel: ExposureKind.CLOUDFLARE_TUNNEL,
  direct: ExposureKind.DIRECT,
  lan: ExposureKind.LAN,
};

export function exposureChoice(kind: ExposureKind): ExposureChoice {
  return EXPOSURE_CHOICES.find((c) => exposureKinds[c] === kind) ?? 'local';
}

/** Who keeps the direct domain pointed at this machine. */
export type DynamicDnsChoice = 'none' | 'duckdns';

export const DYNAMIC_DNS_CHOICES: readonly DynamicDnsChoice[] = ['none', 'duckdns'];

const dynamicDnsKinds: Record<DynamicDnsChoice, DynamicDnsKind> = {
  none: DynamicDnsKind.NONE,
  duckdns: DynamicDnsKind.DUCKDNS,
};

export function dynamicDnsChoice(kind: DynamicDnsKind): DynamicDnsChoice {
  return DYNAMIC_DNS_CHOICES.find((c) => dynamicDnsKinds[c] === kind) ?? 'none';
}

export interface ExposureForm {
  readonly choice: ExposureChoice;
  readonly tailscaleHostname: string;
  readonly tunnelUrl: string;
  /** Empty keeps the stored token. */
  readonly tunnelToken: string;
  readonly directDomain: string;
  readonly directEmail: string;
  readonly directAddr: string;
  readonly dynamicDns: DynamicDnsChoice;
  /** Empty keeps the stored token. */
  readonly dnsToken: string;
  readonly lanAddr: string;
}

export function exposureFormFrom(s: ExposureSettings | undefined): ExposureForm {
  return {
    choice: exposureChoice(s?.kind ?? ExposureKind.LOCAL),
    tailscaleHostname: s?.tailscaleHostname ?? '',
    tunnelUrl: s?.tunnelUrl ?? '',
    tunnelToken: '',
    directDomain: s?.directDomain ?? '',
    directEmail: s?.directEmail ?? '',
    directAddr: s?.directAddr ?? '',
    dynamicDns: dynamicDnsChoice(s?.dynamicDns ?? DynamicDnsKind.NONE),
    dnsToken: '',
    lanAddr: s?.lanAddr ?? '',
  };
}

export function exposureSettingsFrom(
  f: ExposureForm,
): MessageInitShape<typeof ExposureSettingsSchema> {
  return {
    kind: exposureKinds[f.choice],
    tailscaleHostname: f.tailscaleHostname.trim().toLowerCase(),
    tunnelUrl: f.tunnelUrl.trim(),
    tunnelToken: f.tunnelToken.trim(),
    directDomain: f.directDomain.trim().toLowerCase(),
    directEmail: f.directEmail.trim(),
    directAddr: f.directAddr.trim(),
    dynamicDns: dynamicDnsKinds[f.dynamicDns],
    dnsToken: f.dnsToken.trim(),
    lanAddr: f.lanAddr.trim(),
  };
}

export type RelayChoice = 'none' | 'cloudflare' | 'static' | 'coturn';

export const RELAY_CHOICES: readonly RelayChoice[] = ['none', 'cloudflare', 'static', 'coturn'];

const turnKinds: Record<RelayChoice, TurnKind> = {
  none: TurnKind.NONE,
  cloudflare: TurnKind.CLOUDFLARE,
  static: TurnKind.STATIC,
  coturn: TurnKind.COTURN_REST,
};

export function relayChoice(kind: TurnKind): RelayChoice {
  return RELAY_CHOICES.find((c) => turnKinds[c] === kind) ?? 'none';
}

export interface RelayForm {
  readonly choice: RelayChoice;
  readonly cloudflareKeyId: string;
  /** One TURN URL per line. */
  readonly urls: string;
  readonly username: string;
  /** Empty keeps the stored secret. */
  readonly secret: string;
  readonly budgetGb: string;
  readonly budgetOverride: boolean;
}

export function relayFormFrom(s: TurnSettings | undefined): RelayForm {
  return {
    choice: relayChoice(s?.kind ?? TurnKind.NONE),
    cloudflareKeyId: s?.cloudflareKeyId ?? '',
    urls: (s?.urls ?? []).join('\n'),
    username: s?.username ?? '',
    secret: '',
    budgetGb: String(s?.monthlyBudgetGb ?? 0),
    budgetOverride: s?.budgetOverride ?? false,
  };
}

export function turnSettingsFrom(f: RelayForm): MessageInitShape<typeof TurnSettingsSchema> {
  const budget = Number.parseInt(f.budgetGb, 10);
  return {
    kind: turnKinds[f.choice],
    cloudflareKeyId: f.cloudflareKeyId.trim(),
    urls: f.urls
      .split('\n')
      .map((u) => u.trim())
      .filter((u) => u !== ''),
    username: f.username.trim(),
    secret: f.secret.trim(),
    // Out of range on purpose for text, so the hub rejects it with a field error.
    monthlyBudgetGb: Number.isNaN(budget) ? -1 : budget,
    budgetOverride: f.budgetOverride,
  };
}

const GB = 1e9;
/** From this share of the budget on, admins are warned (ARCHITECTURE §4.4). */
const WARN_SHARE = 0.9;

export function nearBudget(u: { monthBytes: bigint; budgetGb: number }): boolean {
  return u.budgetGb > 0 && Number(u.monthBytes) >= WARN_SHARE * u.budgetGb * GB;
}

/** Bytes as gigabytes with one decimal, the unit relay budgets use. */
export function gigabytes(bytes: bigint): number {
  return Math.round((Number(bytes) / GB) * 10) / 10;
}
