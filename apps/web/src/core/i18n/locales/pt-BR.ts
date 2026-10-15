import type { Messages } from './en';

export const ptBR: Messages = {
  shell: {
    home: 'Início',
    servers: 'Servidores',
    serversEmpty: 'Os servidores que você criar ou entrar aparecerão aqui.',
    welcomeTitle: 'Boas-vindas ao {{product}}',
    welcomeBody:
      'Canais de voz, compartilhamento de tela em alta qualidade e desenho ao vivo para sua equipe, hospedados em uma das suas próprias máquinas.',
  },
  health: {
    title: 'Conexão com o hub',
    checking: 'Verificando o hub…',
    serving: 'Hub online',
    degraded: 'Hub com problemas',
    unreachable: 'Hub inacessível',
    version: 'Versão',
    latency: 'Latência',
    latencyValue: '{{ms}} ms',
    started: 'Iniciado',
    unreachableHint:
      'Tentando novamente a cada poucos segundos. Verifique se o hub está em execução.',
  },
  notFound: {
    title: 'Página não encontrada',
    body: 'Este link não leva a lugar nenhum.',
    back: 'Voltar ao início',
  },
};
