import type { UpdateCheck } from '@dislackso/desktop-bridge';

import type { ShellStrings } from '../shared/pages';

const en: ShellStrings = {
  appName: 'Dislackso',
  title: 'Connect to a server',
  hint: 'The address your host shared with you, like https://name.tailnet.ts.net.',
  address: 'Server address',
  connect: 'Connect',
  connecting: 'Connecting…',
  saved: 'Your servers',
  remove: 'Remove',
  invalid: "That doesn't look like a web address.",
  insecure: 'Use an https:// address. Plain http:// works only on this computer.',
  unreachable: "Couldn't reach that server. Check the address and that its hub is running.",
  loadFailed: "Couldn't load the server. Check your connection, or connect to another one.",
  menuServer: 'Server',
  menuSwitch: 'Switch server…',
  menuView: 'View',
  menuEdit: 'Edit',
  updateReady: 'Dislackso {version} is ready. Restart now to update?',
  updateAvailable: 'Dislackso {version} is available. Download it from the releases page.',
  restart: 'Restart now',
  download: 'Download',
  later: 'Later',
  trayOpen: 'Open Dislackso',
  trayQuit: 'Quit Dislackso',
  trayCheckUpdates: 'Check for updates',
  upToDate: 'Dislackso {version} is the latest version.',
  updateDownloading:
    "Dislackso {version} is downloading. You'll be asked to restart when it's ready.",
  updatesUnavailable: 'Updates work in the installed app only.',
  updateCheckFailed:
    "Couldn't check for updates. Check your connection, or there may be no release to update from yet.",
};

const ptBR: ShellStrings = {
  appName: 'Dislackso',
  title: 'Conectar a um servidor',
  hint: 'O endereço que quem hospeda compartilhou com você, como https://nome.tailnet.ts.net.',
  address: 'Endereço do servidor',
  connect: 'Conectar',
  connecting: 'Conectando…',
  saved: 'Seus servidores',
  remove: 'Remover',
  invalid: 'Isso não parece um endereço da web.',
  insecure: 'Use um endereço https://. O http:// funciona só neste computador.',
  unreachable:
    'Não foi possível acessar esse servidor. Confira o endereço e se o hub dele está rodando.',
  loadFailed: 'Não foi possível carregar o servidor. Confira sua conexão ou conecte-se a outro.',
  menuServer: 'Servidor',
  menuSwitch: 'Trocar de servidor…',
  menuView: 'Exibir',
  menuEdit: 'Editar',
  updateReady: 'O Dislackso {version} está pronto. Reiniciar agora para atualizar?',
  updateAvailable: 'O Dislackso {version} está disponível. Baixe na página de versões.',
  restart: 'Reiniciar agora',
  download: 'Baixar',
  later: 'Depois',
  trayOpen: 'Abrir o Dislackso',
  trayQuit: 'Sair do Dislackso',
  trayCheckUpdates: 'Procurar atualizações',
  upToDate: 'O Dislackso {version} é a versão mais recente.',
  updateDownloading:
    'O Dislackso {version} está sendo baixado. Você será avisado para reiniciar quando estiver pronto.',
  updatesUnavailable: 'As atualizações funcionam só no app instalado.',
  updateCheckFailed:
    'Não foi possível procurar atualizações. Confira sua conexão, ou talvez ainda não haja uma versão para atualizar.',
};

/**
 * What the tray tells after a check; undefined when the updater shows its
 * own dialog (a version that is ready or must be downloaded).
 */
export function updateCheckText(check: UpdateCheck, s: ShellStrings): string | undefined {
  switch (check.status) {
    case 'current':
      return s.upToDate.replace('{version}', check.version);
    case 'downloading':
      return s.updateDownloading.replace('{version}', check.version);
    case 'unavailable':
      return s.updatesUnavailable;
    case 'failed':
      return s.updateCheckFailed;
    case 'ready':
    case 'manual':
      return undefined;
  }
}

/** Portuguese for any pt locale, English otherwise. */
export function shellStrings(locale: string): ShellStrings {
  return locale.toLowerCase().startsWith('pt') ? ptBR : en;
}
