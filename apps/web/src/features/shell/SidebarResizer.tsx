import { useRef } from 'react';
import { useTranslation } from 'react-i18next';

import { clampSidebarWidth, dispatchUi, SIDEBAR_WIDTH } from '@/stores/ui';

/** Arrow keys move the edge this far. */
const KEY_STEP = 16;

interface SidebarResizerProps {
  /** The channel sidebar's width now. */
  width: number;
  /** The width while dragging, saved when the drag ends; null when it ends. */
  onLive: (width: number | null) => void;
}

/**
 * The channel sidebar's right edge: drag it to resize within the limits,
 * double-click for the default width, or use the arrow keys.
 */
export function SidebarResizer({ width, onLive }: SidebarResizerProps) {
  const { t } = useTranslation();
  const drag = useRef<{ startX: number; startWidth: number } | null>(null);
  const save = (w: number) => {
    dispatchUi({ type: 'sidebarResized', width: w });
  };

  return (
    <div
      role="separator"
      aria-orientation="vertical"
      aria-label={t('shell.resizeSidebar')}
      aria-valuenow={width}
      aria-valuemin={SIDEBAR_WIDTH.min}
      aria-valuemax={SIDEBAR_WIDTH.max}
      tabIndex={0}
      data-testid="sidebar-resizer"
      onPointerDown={(e) => {
        e.currentTarget.setPointerCapture(e.pointerId);
        drag.current = { startX: e.clientX, startWidth: width };
      }}
      onPointerMove={(e) => {
        const d = drag.current;
        if (d) onLive(clampSidebarWidth(d.startWidth + e.clientX - d.startX));
      }}
      onPointerUp={(e) => {
        const d = drag.current;
        drag.current = null;
        if (!d) return;
        save(d.startWidth + e.clientX - d.startX);
        onLive(null);
      }}
      onDoubleClick={() => {
        save(SIDEBAR_WIDTH.initial);
      }}
      onKeyDown={(e) => {
        if (e.key === 'ArrowLeft') save(width - KEY_STEP);
        else if (e.key === 'ArrowRight') save(width + KEY_STEP);
      }}
      className="absolute top-0 -right-1 z-10 h-full w-2 cursor-col-resize touch-none hover:bg-accent/40 focus-visible:bg-accent/60 focus-visible:outline-none"
    />
  );
}
