const api = window.launcher;

function el<K extends keyof HTMLElementTagNameMap>(
  tag: K,
  props: Partial<HTMLElementTagNameMap[K]> = {},
  ...children: (Node | string)[]
): HTMLElementTagNameMap[K] {
  const node = Object.assign(document.createElement(tag), props);
  node.append(...children);
  return node;
}

async function main(): Promise<void> {
  const s = await api.strings();
  const notice = await api.notice();
  document.title = s.appName;
  const app = document.getElementById('app');
  if (!app) return;

  const input = el('input', {
    id: 'address',
    type: 'url',
    placeholder: 'https://',
    autocomplete: 'on',
    spellcheck: false,
  });
  const button = el('button', { type: 'submit', textContent: s.connect });
  const error = el('p', { className: 'error', role: 'alert' });
  const list = el('ul');
  const savedTitle = el('h2', { textContent: s.saved });

  const problems = { invalid: s.invalid, insecure: s.insecure, unreachable: s.unreachable };
  const connect = async (address: string) => {
    error.textContent = '';
    button.disabled = true;
    button.textContent = s.connecting;
    const result = await api.connect(address);
    button.disabled = false;
    button.textContent = s.connect;
    if (!result.ok) error.textContent = problems[result.problem];
  };

  const render = (origins: readonly string[]) => {
    savedTitle.hidden = origins.length === 0;
    list.replaceChildren(
      ...origins.map((origin) => {
        const open = el('button', { type: 'button', className: 'open', textContent: origin });
        open.addEventListener('click', () => void connect(origin));
        const remove = el('button', { type: 'button', className: 'remove', textContent: s.remove });
        remove.addEventListener('click', () => {
          void api.remove(origin).then(render);
        });
        return el('li', {}, open, remove);
      }),
    );
  };

  const form = el(
    'form',
    {},
    el('label', { htmlFor: 'address', textContent: s.address }),
    el('div', { className: 'row' }, input, button),
    error,
  );
  form.addEventListener('submit', (e) => {
    e.preventDefault();
    void connect(input.value);
  });

  app.append(
    el('h1', { textContent: s.title }),
    el('p', { textContent: s.hint }),
    ...(notice !== '' ? [el('p', { className: 'notice', textContent: notice })] : []),
    form,
    savedTitle,
    list,
  );
  render(await api.saved());
  input.focus();
}

void main();
