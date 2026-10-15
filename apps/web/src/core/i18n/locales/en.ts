export const en = {
  shell: {
    home: 'Home',
    servers: 'Servers',
    serversEmpty: 'Servers you create or join will show up here.',
    welcomeTitle: 'Welcome to {{product}}',
    welcomeBody:
      'Voice channels, high-quality screen sharing and live drawing for your team, hosted on one of your own machines.',
  },
  health: {
    title: 'Hub connection',
    checking: 'Checking hub…',
    serving: 'Hub online',
    degraded: 'Hub degraded',
    unreachable: 'Hub unreachable',
    version: 'Version',
    latency: 'Latency',
    latencyValue: '{{ms}} ms',
    started: 'Started',
    unreachableHint: 'Retrying every few seconds. Make sure the hub is running.',
  },
  notFound: {
    title: 'Page not found',
    body: 'This link does not lead anywhere.',
    back: 'Back to home',
  },
};

export type Messages = DeepStrings<typeof en>;

type DeepStrings<T> = { [K in keyof T]: T[K] extends string ? string : DeepStrings<T[K]> };
