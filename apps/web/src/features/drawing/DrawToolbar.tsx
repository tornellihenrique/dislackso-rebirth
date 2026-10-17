import { COLOR_PRESETS, colorForUser, STROKE_SIZES, type Tool } from '@dislackso/annotation';
import {
  ArrowUpRight,
  Camera,
  Eraser,
  Highlighter,
  Lock,
  LockOpen,
  Pen,
  Sparkle,
  Trash2,
  Undo2,
  X,
} from 'lucide-react';
import { useEffect, type ReactNode } from 'react';
import { useTranslation } from 'react-i18next';

import { useServices } from '@/app/services';
import { useDrawing } from '@/stores/drawing';
import { useDrawingSettings } from '@/stores/drawing-settings';
import { useSession } from '@/stores/session';
import { IconButton } from '@/ui/IconButton';
import { Tooltip } from '@/ui/Tooltip';

interface DrawToolbarProps {
  /** The share's track SID. */
  target: string;
  /** This device's own share: its owner may clear everything and stop others drawing. */
  own: boolean;
  onSnapshot: () => void;
}

const toolIcons: Readonly<Record<Tool, ReactNode>> = {
  pen: <Pen aria-hidden className="size-4" />,
  highlighter: <Highlighter aria-hidden className="size-4" />,
  arrow: <ArrowUpRight aria-hidden className="size-4" />,
  laser: <Sparkle aria-hidden className="size-4" />,
};

/** The pen's controls on a tile in draw mode (ARCHITECTURE §11.1). Escape leaves draw mode. */
export function DrawToolbar({ target, own, onSnapshot }: DrawToolbarProps) {
  const { t } = useTranslation();
  const { drawing } = useServices();
  const s = useDrawingSettings((v) => v);
  const me = useSession((v) => v.me?.user?.id ?? '');
  const othersAllowed = useDrawing((v) => v.othersAllowed);
  const auto = colorForUser(me);
  const color = s.color === '' ? auto : s.color;

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key !== 'Escape') return;
      e.preventDefault();
      drawing.setMode(null);
    };
    window.addEventListener('keydown', onKey);
    return () => {
      window.removeEventListener('keydown', onKey);
    };
  }, [drawing]);

  return (
    <div
      role="toolbar"
      aria-label={t('drawing.toolbar')}
      onDoubleClick={(e) => {
        e.stopPropagation();
      }}
      onContextMenu={(e) => {
        e.stopPropagation();
      }}
      className="absolute top-2 left-2 flex max-w-[calc(100%-1rem)] flex-wrap items-center gap-1 rounded-md bg-floating/95 p-1 shadow-lg"
    >
      {(['pen', 'highlighter', 'arrow', 'laser'] as const).map((tool) => (
        <Button
          key={tool}
          label={t(`drawing.tools.${tool}`)}
          pressed={s.tool === tool}
          onClick={() => {
            drawing.setSettings({ tool });
          }}
        >
          {toolIcons[tool]}
        </Button>
      ))}
      <span aria-hidden className="mx-0.5 h-5 w-px bg-border" />
      <div role="radiogroup" aria-label={t('drawing.color')} className="flex items-center gap-1">
        {COLOR_PRESETS.map((c) => (
          <Tooltip key={c} label={c}>
            <button
              type="button"
              role="radio"
              aria-checked={color === c}
              aria-label={c}
              onClick={() => {
                drawing.setSettings({ color: c === auto ? '' : c });
              }}
              style={{ backgroundColor: c }}
              className={`size-5 rounded-circle border border-border ${color === c ? 'ring-2 ring-fg ring-offset-1 ring-offset-floating' : ''}`}
            />
          </Tooltip>
        ))}
        <Tooltip label={t('drawing.customColor')}>
          <input
            type="color"
            aria-label={t('drawing.customColor')}
            value={color}
            onChange={(e) => {
              drawing.setSettings({ color: e.target.value });
            }}
            className="size-6 cursor-pointer rounded-sm bg-transparent"
          />
        </Tooltip>
      </div>
      <span aria-hidden className="mx-0.5 h-5 w-px bg-border" />
      <div role="radiogroup" aria-label={t('drawing.size')} className="flex items-center">
        {STROKE_SIZES.map((size) => (
          <button
            key={size}
            type="button"
            role="radio"
            aria-checked={s.size === size}
            aria-label={t(`drawing.sizes.${size}`)}
            onClick={() => {
              drawing.setSettings({ size });
            }}
            className={`rounded-sm px-1.5 py-0.5 text-xs font-semibold uppercase ${s.size === size ? 'bg-hover text-fg' : 'text-fg-muted hover:text-fg'}`}
          >
            {size}
          </button>
        ))}
      </div>
      <span aria-hidden className="mx-0.5 h-5 w-px bg-border" />
      <Button
        label={t('drawing.undo')}
        onClick={() => {
          drawing.undo(target);
        }}
      >
        <Undo2 aria-hidden className="size-4" />
      </Button>
      <Button
        label={t('drawing.clearMine')}
        onClick={() => {
          drawing.clear(target, 'mine');
        }}
      >
        <Eraser aria-hidden className="size-4" />
      </Button>
      {own && (
        <>
          <Button
            label={t('drawing.clearAll')}
            onClick={() => {
              drawing.clear(target, 'all');
            }}
          >
            <Trash2 aria-hidden className="size-4" />
          </Button>
          <Button
            label={othersAllowed ? t('drawing.stopOthers') : t('drawing.letOthers')}
            pressed={!othersAllowed}
            onClick={() => {
              drawing.setOthersAllowed(!othersAllowed);
            }}
          >
            {othersAllowed ? (
              <LockOpen aria-hidden className="size-4" />
            ) : (
              <Lock aria-hidden className="size-4" />
            )}
          </Button>
        </>
      )}
      <Button label={t('stage.tile.snapshot')} onClick={onSnapshot}>
        <Camera aria-hidden className="size-4" />
      </Button>
      <Button
        label={t('drawing.done')}
        onClick={() => {
          drawing.setMode(null);
        }}
      >
        <X aria-hidden className="size-4" />
      </Button>
    </div>
  );
}

interface ButtonProps {
  label: string;
  pressed?: boolean;
  onClick: () => void;
  children: ReactNode;
}

function Button({ label, pressed, onClick, children }: ButtonProps) {
  return (
    <Tooltip label={label}>
      <IconButton label={label} aria-pressed={pressed} active={pressed ?? false} onClick={onClick}>
        {children}
      </IconButton>
    </Tooltip>
  );
}
