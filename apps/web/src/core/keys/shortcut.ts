/** A key with its modifiers, by physical key (KeyboardEvent.code), so layouts do not matter. */
export interface Shortcut {
  readonly code: string;
  readonly ctrl: boolean;
  readonly shift: boolean;
  readonly alt: boolean;
  readonly meta: boolean;
}

/** What a shortcut is read from: a KeyboardEvent, or the desktop shell's global hook. */
export interface KeyPress {
  readonly code: string;
  readonly ctrlKey: boolean;
  readonly shiftKey: boolean;
  readonly altKey: boolean;
  readonly metaKey: boolean;
}

const MODIFIER_CODES = new Set([
  'ControlLeft',
  'ControlRight',
  'ShiftLeft',
  'ShiftRight',
  'AltLeft',
  'AltRight',
  'MetaLeft',
  'MetaRight',
]);

export function isModifierCode(code: string): boolean {
  return MODIFIER_CODES.has(code);
}

export const shortcut = (
  code: string,
  mods: { ctrl?: boolean; shift?: boolean; alt?: boolean; meta?: boolean } = {},
): Shortcut => ({
  code,
  ctrl: mods.ctrl ?? false,
  shift: mods.shift ?? false,
  alt: mods.alt ?? false,
  meta: mods.meta ?? false,
});

/** The shortcut a key press makes; undefined while only modifiers are down. */
export function shortcutOf(e: KeyPress): Shortcut | undefined {
  if (e.code === '' || isModifierCode(e.code)) return undefined;
  return shortcut(e.code, { ctrl: e.ctrlKey, shift: e.shiftKey, alt: e.altKey, meta: e.metaKey });
}

export function matches(e: KeyPress, s: Shortcut): boolean {
  return (
    e.code === s.code &&
    e.ctrlKey === s.ctrl &&
    e.shiftKey === s.shift &&
    e.altKey === s.alt &&
    e.metaKey === s.meta
  );
}

export function sameShortcut(a: Shortcut | null, b: Shortcut | null): boolean {
  if (!a || !b) return a === b;
  return (
    a.code === b.code &&
    a.ctrl === b.ctrl &&
    a.shift === b.shift &&
    a.alt === b.alt &&
    a.meta === b.meta
  );
}

const NAMED: Readonly<Record<string, string>> = {
  ArrowUp: '↑',
  ArrowDown: '↓',
  ArrowLeft: '←',
  ArrowRight: '→',
  Backquote: '`',
  Minus: '-',
  Equal: '=',
  BracketLeft: '[',
  BracketRight: ']',
  Backslash: '\\',
  Semicolon: ';',
  Quote: "'",
  Comma: ',',
  Period: '.',
  Slash: '/',
  Escape: 'Esc',
  Space: 'Space',
  Enter: 'Enter',
  Backspace: 'Backspace',
  Tab: 'Tab',
  CapsLock: 'Caps Lock',
  ControlLeft: 'Ctrl',
  ControlRight: 'Right Ctrl',
  ShiftLeft: 'Shift',
  ShiftRight: 'Right Shift',
  AltLeft: 'Alt',
  AltRight: 'Right Alt',
  MetaLeft: 'Meta',
  MetaRight: 'Right Meta',
};

/** How a key is written on the keyboard: KeyM → M, Digit1 → 1, Numpad1 → Num 1. */
export function keyLabel(code: string): string {
  const named = NAMED[code];
  if (named !== undefined) return named;
  if (/^Key[A-Z]$/.test(code)) return code.slice(3);
  if (/^Digit\d$/.test(code)) return code.slice(5);
  if (code.startsWith('Numpad')) return `Num ${code.slice(6)}`;
  return code;
}

/** "Ctrl + Shift + M"; Cmd and Option on a Mac. */
export function shortcutLabel(s: Shortcut, mac = false): string {
  const parts: string[] = [];
  if (s.ctrl) parts.push('Ctrl');
  if (s.meta) parts.push(mac ? 'Cmd' : 'Win');
  if (s.alt) parts.push(mac ? 'Option' : 'Alt');
  if (s.shift) parts.push('Shift');
  parts.push(keyLabel(s.code));
  return parts.join(' + ');
}

/** Reads a stored shortcut; anything malformed is none. */
export function parseShortcut(v: unknown): Shortcut | null {
  if (typeof v !== 'object' || v === null) return null;
  const r = v as Record<string, unknown>;
  if (typeof r.code !== 'string' || r.code === '' || r.code.length > 32) return null;
  const flag = (k: string) => r[k] === true;
  return shortcut(r.code, {
    ctrl: flag('ctrl'),
    shift: flag('shift'),
    alt: flag('alt'),
    meta: flag('meta'),
  });
}

/** Typing in a field keeps plain keys for the field; shortcuts with Ctrl, Alt or Meta still work. */
export function typingTarget(target: EventTarget | null): boolean {
  if (!(target instanceof HTMLElement)) return false;
  return (
    target.isContentEditable ||
    target instanceof HTMLTextAreaElement ||
    (target instanceof HTMLInputElement &&
      !['checkbox', 'radio', 'range', 'button', 'submit'].includes(target.type)) ||
    target instanceof HTMLSelectElement
  );
}
