import { create, type MessageInitShape } from '@bufbuild/protobuf';
import {
  AnnotationPacketSchema,
  ClearScope,
  Tool,
  type AnnotationPacket,
} from '@dislackso/contracts/gen/dislackso/annotation/v1/annotation_pb';
import { beforeEach, describe, expect, it } from 'vitest';

import { decodePacket, encodePacket } from './codec';
import { AnnotationEngine, type Outgoing, type StrokeStyle } from './engine';
import { packPoints, type Point } from './geometry';
import { FADE_MS, LASER_IDLE_MS, NAME_TAG_MS } from './timing';

const ANA_SHARE = 'TR_ana';
const BOB_SHARE = 'TR_bob';
const pen: StrokeStyle = { tool: 'pen', color: 0xff0000ff, widthRel: 0.004, holdMs: 3000 };

let now = 0;
let ana: AnnotationEngine;
let bob: AnnotationEngine;

/** Delivers packets the way LiveKit does: encoded, with the sender's identity. */
function deliver(from: AnnotationEngine, to: AnnotationEngine, out: readonly Outgoing[]) {
  for (const o of out) {
    const p = decodePacket(encodePacket(o.packet));
    if (p) to.receive(p, from.self);
  }
}

function raw(
  kind: NonNullable<MessageInitShape<typeof AnnotationPacketSchema>['kind']>,
): AnnotationPacket {
  return create(AnnotationPacketSchema, { kind });
}

/** Draws a line of n points 10 ms apart, sending each batch as it goes. */
function drawLine(target: string, n: number, style = pen, lose = false): Point[] {
  const pts = Array.from({ length: n }, (_, i) => ({ x: 0.1 + i * 0.01, y: 0.5 }));
  deliver(ana, bob, ana.begin(target, style, pts[0] ?? { x: 0, y: 0 }));
  for (const p of pts.slice(1)) {
    now += 10;
    deliver(ana, bob, ana.extend([p]));
    const batch = ana.flush();
    if (!lose) deliver(ana, bob, batch);
  }
  deliver(ana, bob, ana.end());
  return pts;
}

beforeEach(() => {
  now = 1000;
  const clock = () => now;
  ana = new AnnotationEngine({ self: 'ana', now: clock, firstStrokeId: 1 });
  bob = new AnnotationEngine({ self: 'bob', now: clock, firstStrokeId: 100 });
  const targets = new Map([
    [ANA_SHARE, { owner: 'ana' }],
    [BOB_SHARE, { owner: 'bob' }],
  ]);
  ana.setTargets(targets);
  bob.setTargets(targets);
});

describe('strokes', () => {
  it('reach the other client with the same points, then fade after the hold', () => {
    const pts = drawLine(BOB_SHARE, 5);
    const seen = bob.scene(BOB_SHARE).strokes;
    expect(seen).toHaveLength(1);
    expect(seen[0]?.author).toBe('ana');
    expect(seen[0]?.points.map((p) => p.x)).toEqual(
      pts.map((p) => expect.closeTo(p.x, 4) as unknown as number),
    );
    expect(ana.scene(BOB_SHARE).strokes).toHaveLength(1);

    now += 3000;
    expect(bob.scene(BOB_SHARE).strokes[0]?.alpha).toBe(1);
    now += FADE_MS / 2;
    expect(bob.scene(BOB_SHARE).strokes[0]?.alpha).toBeCloseTo(0.5);
    now += FADE_MS / 2;
    expect(bob.scene(BOB_SHARE).strokes).toHaveLength(0);
    expect(bob.active(BOB_SHARE)).toBe(false);
    expect(ana.active(BOB_SHARE)).toBe(false);
  });

  it('are repaired by the end when the lossy points were lost', () => {
    drawLine(BOB_SHARE, 6, pen, true);
    expect(bob.scene(BOB_SHARE).strokes[0]?.points).toHaveLength(6);
  });

  it('keep the hold within the instance cap', () => {
    bob.setHoldMax(2000);
    drawLine(BOB_SHARE, 2, { ...pen, holdMs: 8000 });
    expect(bob.scene(BOB_SHARE).strokes[0]?.holdMs).toBe(2000);
  });

  it('keep only the start and the end of an arrow', () => {
    drawLine(BOB_SHARE, 5, { ...pen, tool: 'arrow' });
    const [arrow] = bob.scene(BOB_SHARE).strokes;
    expect(arrow?.points).toHaveLength(2);
    expect(arrow?.points[1]?.x).toBeCloseTo(0.14, 4);
  });

  it('show the author next to the pen tip while drawing and shortly after', () => {
    deliver(ana, bob, ana.begin(BOB_SHARE, pen, { x: 0.2, y: 0.2 }));
    deliver(ana, bob, ana.flush());
    expect(bob.scene(BOB_SHARE).tags).toEqual([
      { author: 'ana', at: expect.anything() as unknown, color: pen.color },
    ]);
    deliver(ana, bob, ana.end());
    now += NAME_TAG_MS + 1;
    expect(bob.scene(BOB_SHARE).tags).toEqual([]);
  });

  it('drops points closer or faster than anyone could see', () => {
    ana.begin(ANA_SHARE, pen, { x: 0.5, y: 0.5 });
    now += 1;
    ana.extend([{ x: 0.6, y: 0.5 }]);
    now += 10;
    ana.extend([
      { x: 0.50001, y: 0.5 },
      { x: 0.7, y: 0.5 },
    ]);
    expect(ana.scene(ANA_SHARE).strokes[0]?.points.map((p) => p.x)).toEqual([0.5, 0.7]);
  });
});

describe('undo and clear', () => {
  it('undo takes back the author’s latest stroke everywhere', () => {
    drawLine(BOB_SHARE, 3);
    drawLine(BOB_SHARE, 3);
    deliver(ana, bob, ana.undo(BOB_SHARE));
    expect(bob.scene(BOB_SHARE).strokes).toHaveLength(1);
    expect(ana.scene(BOB_SHARE).strokes).toHaveLength(1);
  });

  it('clearing all is only for the share’s owner', () => {
    drawLine(BOB_SHARE, 3);
    bob.receive(
      raw({ case: 'clear', value: { targetTrackSid: BOB_SHARE, scope: ClearScope.ALL } }),
      'ana',
    );
    expect(bob.scene(BOB_SHARE).strokes).toHaveLength(1);
    expect(ana.clear(BOB_SHARE, 'all')).toEqual([]);
    deliver(bob, ana, bob.clear(BOB_SHARE, 'all'));
    expect(ana.scene(BOB_SHARE).strokes).toHaveLength(0);
  });

  it('clearing mine leaves the others', () => {
    drawLine(BOB_SHARE, 3);
    deliver(bob, ana, bob.begin(BOB_SHARE, pen, { x: 0.9, y: 0.9 }));
    deliver(bob, ana, bob.end());
    deliver(ana, bob, ana.clear(BOB_SHARE, 'mine'));
    expect(bob.scene(BOB_SHARE).strokes.map((s) => s.author)).toEqual(['bob']);
  });
});

describe('drawing policy', () => {
  it('lets the owner stop others from drawing and clears what they drew', () => {
    drawLine(BOB_SHARE, 3);
    deliver(bob, ana, bob.setAllowed(BOB_SHARE, false));
    expect(ana.canDraw(BOB_SHARE)).toBe(false);
    expect(ana.scene(BOB_SHARE).strokes).toHaveLength(0);
    expect(bob.scene(BOB_SHARE).strokes).toHaveLength(0);
    expect(ana.begin(BOB_SHARE, pen, { x: 0.5, y: 0.5 })).toEqual([]);
    // The owner still draws on their own share.
    expect(bob.canDraw(BOB_SHARE)).toBe(true);
    deliver(bob, ana, bob.setAllowed(BOB_SHARE, true));
    expect(ana.canDraw(BOB_SHARE)).toBe(true);
  });

  it('ignores a policy, or a stroke, for someone else’s share or an unknown one', () => {
    ana.receive(
      raw({ case: 'policy', value: { targetTrackSid: BOB_SHARE, allowed: false } }),
      'eve',
    );
    expect(ana.canDraw(BOB_SHARE)).toBe(true);
    bob.receive(
      raw({
        case: 'strokeBegin',
        value: { strokeId: 9, targetTrackSid: 'TR_gone', tool: Tool.PEN, widthRel: 0.004 },
      }),
      'ana',
    );
    expect(bob.dropped.strokes).toBe(1);
  });

  it('forgets a share that ended, with what was drawn on it', () => {
    drawLine(BOB_SHARE, 3);
    bob.setTargets(new Map([[ANA_SHARE, { owner: 'ana' }]]));
    expect(bob.active(BOB_SHARE)).toBe(false);
  });
});

describe('receiver limits', () => {
  const begin = (id: number) =>
    raw({
      case: 'strokeBegin',
      value: {
        strokeId: id,
        targetTrackSid: BOB_SHARE,
        tool: Tool.PEN,
        widthRel: 0.004,
        holdMs: 3000,
      },
    });
  const points = (id: number, n: number) =>
    raw({
      case: 'strokePoints',
      value: {
        strokeId: id,
        packedPoints: packPoints(Array.from({ length: n }, (_, i) => ({ x: i / n, y: 0.5 }))),
      },
    });

  it('throttle a flood of 2,000 points a second to what one hand can draw', () => {
    bob.receive(begin(1), 'ana');
    for (let i = 0; i < 100; i++) {
      now += 10;
      bob.receive(points(1, 20), 'ana');
    }
    const kept = bob.scene(BOB_SHARE).strokes[0]?.points.length ?? 0;
    // One second's burst plus one second of refill, give or take a batch.
    expect(kept).toBeLessThanOrEqual(500);
    expect(kept).toBeGreaterThan(200);
    expect(bob.dropped.points).toBe(2000 - kept);
  });

  it('cap concurrent strokes per author, active strokes per share and points per stroke', () => {
    for (let id = 1; id <= 10; id++) bob.receive(begin(id), 'ana');
    expect(bob.dropped.strokes).toBe(2);
    for (let id = 1; id <= 8; id++) {
      bob.receive(
        raw({
          case: 'strokeEnd',
          value: { strokeId: id, packedPoints: packPoints([{ x: 0.1, y: 0.1 }]) },
        }),
        'ana',
      );
    }
    for (let id = 100; id < 140; id++) {
      bob.receive(begin(id), 'ana');
      bob.receive(
        raw({
          case: 'strokeEnd',
          value: { strokeId: id, packedPoints: packPoints([{ x: 0.2, y: 0.2 }]) },
        }),
        'ana',
      );
    }
    expect(bob.scene(BOB_SHARE).strokes).toHaveLength(40);

    bob.receive(begin(500), 'eve');
    const many = Array.from({ length: 5000 }, (_, i) => ({ x: (i % 100) / 100, y: 0.3 }));
    bob.receive(
      raw({ case: 'strokeEnd', value: { strokeId: 500, packedPoints: packPoints(many) } }),
      'eve',
    );
    const long = bob.scene(BOB_SHARE).strokes.find((s) => s.author === 'eve');
    expect(long?.points).toHaveLength(4000);
  });
});

describe('laser', () => {
  it('shows a trail while it moves, then goes when put away or left still', () => {
    ana.laser(BOB_SHARE, { x: 0.1, y: 0.1 });
    deliver(ana, bob, ana.flush());
    now += 20;
    ana.laser(BOB_SHARE, { x: 0.2, y: 0.1 });
    deliver(ana, bob, ana.flush());
    expect(bob.scene(BOB_SHARE).lasers[0]?.trail).toHaveLength(2);
    expect(bob.scene(BOB_SHARE).tags.map((t) => t.author)).toEqual(['ana']);
    ana.laser(BOB_SHARE, undefined);
    deliver(ana, bob, ana.flush());
    expect(bob.scene(BOB_SHARE).lasers).toHaveLength(0);

    ana.laser(BOB_SHARE, { x: 0.3, y: 0.3 });
    deliver(ana, bob, ana.flush());
    now += LASER_IDLE_MS + 1;
    expect(bob.active(BOB_SHARE)).toBe(false);
  });
});

describe('fading', () => {
  it('holds every stroke of an author while they keep drawing, then fades them together', () => {
    drawLine(BOB_SHARE, 3);
    now += 2500;
    deliver(ana, bob, ana.begin(BOB_SHARE, pen, { x: 0.2, y: 0.8 }));
    deliver(ana, bob, ana.flush());
    // The first line's own hold is over, but its author is drawing again.
    now += 1000;
    expect(bob.scene(BOB_SHARE).strokes.map((s) => s.alpha)).toEqual([1, 1]);
    deliver(ana, bob, ana.end());
    now += 3000;
    expect(bob.scene(BOB_SHARE).strokes.map((s) => s.alpha)).toEqual([1, 1]);
    now += FADE_MS;
    expect(bob.scene(BOB_SHARE).strokes).toHaveLength(0);
  });

  it("lets each author's strokes fade on their own", () => {
    drawLine(BOB_SHARE, 3);
    now += 2500;
    deliver(bob, ana, bob.begin(BOB_SHARE, pen, { x: 0.9, y: 0.9 }));
    deliver(bob, ana, bob.flush());
    now += 1000 + FADE_MS;
    expect(ana.scene(BOB_SHARE).strokes.map((s) => s.author)).toEqual(['bob']);
  });
});

describe('laser color', () => {
  it('travels with the laser, whatever color the last stroke had', () => {
    drawLine(BOB_SHARE, 2);
    ana.laser(BOB_SHARE, { x: 0.5, y: 0.5 }, 0x00ff00ff);
    deliver(ana, bob, ana.flush());
    expect(bob.scene(BOB_SHARE).lasers[0]?.color).toBe(0x00ff00ff);
    expect(ana.scene(BOB_SHARE).lasers[0]?.color).toBe(0x00ff00ff);
  });
});
