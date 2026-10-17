import { CanvasRenderer, contentRect, toNormalized, type Point } from '@dislackso/annotation';
import { useEffect, useRef, type PointerEvent } from 'react';

import { useServices } from '@/app/services';
import { useDrawingSettings } from '@/stores/drawing-settings';

import { useNameOf } from './hooks';

interface AnnotationLayerProps {
  /** The share's track SID. */
  target: string;
  /** The sharer, whose video sets where the content is. */
  userId: string;
  self: boolean;
  /** Pointer input draws instead of reaching the tile. */
  drawing: boolean;
}

/**
 * The strokes on one share tile, drawn on a canvas over the video, and the
 * pen input while the tile is in draw mode. Points are normalized to the
 * video's content rect, so they land on the same content for everyone.
 */
export function AnnotationLayer({ target, userId, self, drawing }: AnnotationLayerProps) {
  const { drawing: actions, voice } = useServices();
  const showTags = useDrawingSettings((s) => s.nameTags);
  const tool = useDrawingSettings((s) => s.tool);
  const nameOf = useNameOf();
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const pressed = useRef(false);

  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas) return;
    const renderer = new CanvasRenderer(canvas);
    let frame = 0;
    const draw = () => {
      frame = 0;
      const video = voice.shareElement(userId, self);
      const w = canvas.clientWidth;
      const h = canvas.clientHeight;
      renderer.resize(w, h, window.devicePixelRatio);
      const scene = actions.scene(target);
      const rect = contentRect(video?.videoWidth ?? 0, video?.videoHeight ?? 0, w, h);
      renderer.render(scene, rect, { nameOf, showTags, now: performance.now() });
      canvas.dataset.strokes = String(scene.strokes.length);
      canvas.dataset.points = String(scene.strokes.reduce((n, s) => n + s.points.length, 0));
      if (actions.active(target)) frame = requestAnimationFrame(draw);
    };
    const kick = () => {
      if (frame === 0) frame = requestAnimationFrame(draw);
    };
    const stop = actions.subscribe(kick);
    const observer = new ResizeObserver(kick);
    observer.observe(canvas);
    const video = voice.shareElement(userId, self);
    video?.addEventListener('resize', kick);
    kick();
    return () => {
      cancelAnimationFrame(frame);
      stop();
      observer.disconnect();
      video?.removeEventListener('resize', kick);
    };
  }, [actions, voice, target, userId, self, nameOf, showTags]);

  const point = (e: { clientX: number; clientY: number }): Point => {
    const canvas = canvasRef.current;
    const box = canvas?.getBoundingClientRect();
    if (!canvas || !box) return { x: 0, y: 0 };
    const video = voice.shareElement(userId, self);
    const rect = contentRect(
      video?.videoWidth ?? 0,
      video?.videoHeight ?? 0,
      box.width,
      box.height,
    );
    return toNormalized(e.clientX - box.left, e.clientY - box.top, rect);
  };

  const release = (e: PointerEvent<HTMLCanvasElement>) => {
    if (!pressed.current) return;
    pressed.current = false;
    if (e.currentTarget.hasPointerCapture(e.pointerId)) {
      e.currentTarget.releasePointerCapture(e.pointerId);
    }
    if (tool === 'laser') actions.laser(target, undefined);
    else actions.end();
  };

  return (
    <canvas
      ref={canvasRef}
      data-testid="annotation-layer"
      data-drawing={drawing || undefined}
      className={`absolute inset-0 size-full ${drawing ? 'cursor-crosshair touch-none' : 'pointer-events-none'}`}
      onPointerDown={(e) => {
        if (!drawing || e.button !== 0) return;
        e.preventDefault();
        e.stopPropagation();
        e.currentTarget.setPointerCapture(e.pointerId);
        pressed.current = true;
        if (tool === 'laser') actions.laser(target, point(e));
        else actions.begin(target, point(e));
      }}
      onPointerMove={(e) => {
        if (!drawing || !pressed.current) return;
        if (tool === 'laser') {
          actions.laser(target, point(e));
          return;
        }
        // A fast pen sends several positions per frame; each is a point of the line.
        const events = e.nativeEvent.getCoalescedEvents();
        actions.extend((events.length > 0 ? events : [e.nativeEvent]).map(point));
      }}
      onPointerUp={release}
      onPointerCancel={release}
      onDoubleClick={(e) => {
        if (drawing) e.stopPropagation();
      }}
      onContextMenu={(e) => {
        if (!drawing) return;
        e.preventDefault();
        e.stopPropagation();
        actions.setMode(null);
      }}
    />
  );
}
