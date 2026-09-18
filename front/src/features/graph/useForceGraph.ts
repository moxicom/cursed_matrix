import { useCallback, useEffect, useRef, useState } from 'react';

import { QUADRANT_BY_ID, TASK_COLOR_BY_ID } from '@/shared/config/domain';
import type { Task, TaskLink } from '@/shared/types/domain';

interface NodeState {
  x: number;
  y: number;
  vx: number;
  vy: number;
  fixed: boolean;
}

interface View {
  x: number;
  y: number;
  k: number;
}

export interface ForceGraphInput {
  nodes: readonly Task[];
  links: readonly TaskLink[];
  /** Effective quadrant per task id — subtasks inherit it from their parent. */
  quadrantOf: (task: Task) => string;
  linkCountOf: (id: string) => number;
  subtaskCountOf: (id: string) => number;
  selectedId: string | null;
  onSelect: (id: string) => void;
  onHover: (id: string | null) => void;
}

export interface ForceGraphApi {
  canvasRef: (element: HTMLCanvasElement | null) => void;
  zoom: number;
  zoomBy: (factor: number) => void;
  fit: () => void;
}

const MIN_ZOOM = 0.28;
const MAX_ZOOM = 3.4;

/**
 * Force-directed simulation on a canvas: repulsion between every pair, springs
 * along links (108px) and parent-child edges (46px), gentle centering, damping.
 * Ported from the design's tick()/draw() so the motion feels identical.
 */
export function useForceGraph(input: ForceGraphInput): ForceGraphApi {
  const [zoom, setZoom] = useState(1);
  const canvasRef = useRef<HTMLCanvasElement | null>(null);
  const ctxRef = useRef<CanvasRenderingContext2D | null>(null);
  const nodesRef = useRef<Record<string, NodeState>>({});
  const viewRef = useRef<View>({ x: 0, y: 0, k: 1 });
  const alphaRef = useRef(1);
  /** Until the user pans/zooms, the world origin is kept in the canvas centre. */
  const viewportTouchedRef = useRef(false);
  const sizeRef = useRef({ w: 0, h: 0, dpr: 1 });
  const inputRef = useRef(input);
  const pointerRef = useRef<{
    startX: number;
    startY: number;
    panX: number;
    panY: number;
    nodeId: string | null;
    moved: boolean;
  } | null>(null);

  inputRef.current = input;

  const toWorld = useCallback((x: number, y: number) => {
    const view = viewRef.current;
    return { x: (x - view.x) / view.k, y: (y - view.y) / view.k };
  }, []);

  const radius = useCallback((task: Task): number => {
    const { linkCountOf, subtaskCountOf } = inputRef.current;
    const isSub = task.parentTaskId !== null;
    return (
      (isSub ? 3.6 : 5.6) +
      Math.min(5, linkCountOf(task.id) * 0.9) +
      (isSub ? 0 : subtaskCountOf(task.id) * 0.5)
    );
  }, []);

  const nodeAt = useCallback(
    (px: number, py: number): Task | null => {
      const point = toWorld(px, py);
      const view = viewRef.current;
      let hit: Task | null = null;
      inputRef.current.nodes.forEach((task) => {
        const node = nodesRef.current[task.id];
        if (!node) return;
        const r = Math.max(6 / view.k, radius(task) + 4 / view.k);
        if ((node.x - point.x) ** 2 + (node.y - point.y) ** 2 <= r * r) hit = task;
      });
      return hit;
    },
    [radius, toWorld],
  );

  const tick = useCallback(() => {
    const { nodes, links } = inputRef.current;
    const store = nodesRef.current;
    const alpha = alphaRef.current;

    nodes.forEach((task) => {
      if (!store[task.id]) {
        store[task.id] = {
          x: (Math.random() - 0.5) * 200,
          y: (Math.random() - 0.5) * 200,
          vx: 0,
          vy: 0,
          fixed: false,
        };
      }
    });

    // pairwise repulsion
    for (let i = 0; i < nodes.length; i += 1) {
      const a = store[nodes[i]!.id];
      if (!a) continue;
      for (let j = i + 1; j < nodes.length; j += 1) {
        const b = store[nodes[j]!.id];
        if (!b) continue;
        let dx = b.x - a.x;
        let dy = b.y - a.y;
        let d2 = dx * dx + dy * dy;
        if (d2 < 1) {
          d2 = 1;
          dx = Math.random() - 0.5;
          dy = Math.random() - 0.5;
        }
        const d = Math.sqrt(d2);
        const f = Math.min(6, 2600 / d2) * 0.02;
        a.vx -= (dx / d) * f;
        a.vy -= (dy / d) * f;
        b.vx += (dx / d) * f;
        b.vy += (dy / d) * f;
      }
    }

    const present = new Set(nodes.map((task) => task.id));
    const springs: Array<[string, string, number]> = [];
    links.forEach((link) => {
      if (present.has(link.sourceTaskId) && present.has(link.targetTaskId)) {
        springs.push([link.sourceTaskId, link.targetTaskId, 108]);
      }
    });
    nodes.forEach((task) => {
      if (task.parentTaskId !== null && present.has(task.parentTaskId)) {
        springs.push([task.id, task.parentTaskId, 46]);
      }
    });

    springs.forEach(([aId, bId, rest]) => {
      const a = store[aId];
      const b = store[bId];
      if (!a || !b) return;
      const dx = b.x - a.x;
      const dy = b.y - a.y;
      const d = Math.max(1, Math.hypot(dx, dy));
      const f = (d - rest) * 0.012;
      a.vx += (dx / d) * f;
      a.vy += (dy / d) * f;
      b.vx -= (dx / d) * f;
      b.vy -= (dy / d) * f;
    });

    nodes.forEach((task) => {
      const node = store[task.id];
      if (!node) return;
      node.vx -= node.x * 0.0016;
      node.vy -= node.y * 0.0016;
      if (node.fixed) {
        node.vx = 0;
        node.vy = 0;
        return;
      }
      node.vx *= 0.86;
      node.vy *= 0.86;
      node.x += node.vx * (0.35 + alpha);
      node.y += node.vy * (0.35 + alpha);
    });

    alphaRef.current = Math.max(0.06, alpha * 0.995);
  }, []);

  const hoverRef = useRef<string | null>(null);

  const draw = useCallback(() => {
    const ctx = ctxRef.current;
    if (!ctx) return;
    const { w, h, dpr } = sizeRef.current;
    const { nodes, links, quadrantOf, selectedId } = inputRef.current;
    const store = nodesRef.current;
    const view = viewRef.current;

    ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
    ctx.clearRect(0, 0, w, h);
    ctx.fillStyle = '#0a0b0c';
    ctx.fillRect(0, 0, w, h);

    // background grid: fine lines every 28 world units, a brighter one every 4th.
    // The screen-space step is kept between 16 and 64px by doubling/halving, so
    // the grid stays equally dense at any zoom level.
    let step = 28 * view.k;
    while (step < 16) step *= 2;
    while (step > 64) step /= 2;

    const drawGrid = (gridStep: number, color: string) => {
      ctx.strokeStyle = color;
      ctx.lineWidth = 1;
      ctx.beginPath();
      for (let x = view.x % gridStep; x < w; x += gridStep) {
        ctx.moveTo(Math.round(x) + 0.5, 0);
        ctx.lineTo(Math.round(x) + 0.5, h);
      }
      for (let y = view.y % gridStep; y < h; y += gridStep) {
        ctx.moveTo(0, Math.round(y) + 0.5);
        ctx.lineTo(w, Math.round(y) + 0.5);
      }
      ctx.stroke();
    };

    drawGrid(step, '#101315');
    drawGrid(step * 4, '#181c1f');

    const byId = new Map(nodes.map((task) => [task.id, task]));
    const project = (node: NodeState) => ({ x: node.x * view.k + view.x, y: node.y * view.k + view.y });
    const hovered = hoverRef.current;

    // parent-child edges (dashed, system relation)
    nodes.forEach((task) => {
      if (task.parentTaskId === null || !byId.has(task.parentTaskId)) return;
      const a = store[task.id];
      const b = store[task.parentTaskId];
      if (!a || !b) return;
      const pa = project(a);
      const pb = project(b);
      ctx.strokeStyle = '#242a2e';
      ctx.lineWidth = 1;
      ctx.setLineDash([3, 3]);
      ctx.beginPath();
      ctx.moveTo(pa.x, pa.y);
      ctx.lineTo(pb.x, pb.y);
      ctx.stroke();
      ctx.setLineDash([]);
    });

    // user links
    links.forEach((link) => {
      const source = byId.get(link.sourceTaskId);
      const target = byId.get(link.targetTaskId);
      if (!source || !target) return;
      const a = store[link.sourceTaskId];
      const b = store[link.targetTaskId];
      if (!a || !b) return;
      const pa = project(a);
      const pb = project(b);
      const hot =
        selectedId === link.sourceTaskId ||
        selectedId === link.targetTaskId ||
        hovered === link.sourceTaskId ||
        hovered === link.targetTaskId;
      const dim = source.status === 'COMPLETED' && target.status === 'COMPLETED';
      ctx.strokeStyle = hot ? '#5d8ea3' : dim ? '#1d2225' : '#2b3236';
      ctx.lineWidth = hot ? 1.4 : 1;
      ctx.beginPath();
      ctx.moveTo(pa.x, pa.y);
      ctx.lineTo(pb.x, pb.y);
      ctx.stroke();
    });

    // nodes
    nodes.forEach((task) => {
      const node = store[task.id];
      if (!node) return;
      const p = project(node);
      const r = radius(task) * view.k;
      const done = task.status === 'COMPLETED';
      const quadrant = quadrantOf(task);
      const accent = QUADRANT_BY_ID[quadrant as keyof typeof QUADRANT_BY_ID]?.accent ?? '#7d878c';
      const custom = TASK_COLOR_BY_ID[task.color].hex;
      const isSelected = selectedId === task.id;
      const isHovered = hovered === task.id;

      ctx.globalAlpha = done ? 0.34 : 1;
      ctx.beginPath();
      ctx.arc(p.x, p.y, Math.max(2, r), 0, Math.PI * 2);
      ctx.fillStyle = done ? '#191d20' : (custom ?? '#1c2226');
      ctx.fill();
      ctx.lineWidth = isSelected ? 2 : 1;
      ctx.strokeStyle = isSelected ? '#d8dde0' : isHovered ? '#8fb6c6' : done ? '#2b3236' : accent;
      ctx.stroke();

      // subtasks are drawn hollow
      if (task.parentTaskId !== null) {
        ctx.beginPath();
        ctx.arc(p.x, p.y, Math.max(1, r * 0.35), 0, Math.PI * 2);
        ctx.fillStyle = '#0a0b0c';
        ctx.fill();
      }

      if (view.k > 0.72) {
        ctx.globalAlpha = done ? 0.4 : 0.92;
        ctx.font = `${task.parentTaskId !== null ? 9 : 10}px 'JetBrains Mono', monospace`;
        ctx.fillStyle = isSelected || isHovered ? '#d8dde0' : '#7d878c';
        ctx.textAlign = 'center';
        const label = task.title.length > 26 ? `${task.title.slice(0, 25)}…` : task.title;
        ctx.fillText(label, p.x, p.y + r + 12);
        if (view.k > 1.25) {
          ctx.fillStyle = '#6f797e';
          ctx.font = "8px 'JetBrains Mono', monospace";
          ctx.fillText(
            `${task.id.toUpperCase()} · ${
              QUADRANT_BY_ID[quadrant as keyof typeof QUADRANT_BY_ID]?.code ?? 'SUB'
            }`,
            p.x,
            p.y + r + 23,
          );
        }
      }
      ctx.globalAlpha = 1;
    });
  }, [radius]);

  const setCanvas = useCallback(
    (element: HTMLCanvasElement | null) => {
      canvasRef.current = element;
      ctxRef.current = element?.getContext('2d') ?? null;
      if (element) alphaRef.current = 1;
    },
    [],
  );

  // resize + render loop
  useEffect(() => {
    let raf = 0;

    const resize = () => {
      const canvas = canvasRef.current;
      if (!canvas) return;
      const rect = canvas.getBoundingClientRect();
      if (rect.width === 0 || rect.height === 0) return;
      const dpr = window.devicePixelRatio || 1;
      canvas.width = Math.round(rect.width * dpr);
      canvas.height = Math.round(rect.height * dpr);
      sizeRef.current = { w: rect.width, h: rect.height, dpr };
      if (!viewportTouchedRef.current) {
        viewRef.current.x = rect.width / 2;
        viewRef.current.y = rect.height / 2;
      }
    };

    const loop = () => {
      if (canvasRef.current) {
        tick();
        draw();
      }
      raf = requestAnimationFrame(loop);
    };

    resize();
    const initial = requestAnimationFrame(resize);
    window.addEventListener('resize', resize);
    raf = requestAnimationFrame(loop);

    return () => {
      window.removeEventListener('resize', resize);
      cancelAnimationFrame(initial);
      cancelAnimationFrame(raf);
    };
  }, [draw, tick]);

  // pointer + wheel interaction
  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas) return undefined;

    const local = (event: PointerEvent | WheelEvent) => {
      const rect = canvas.getBoundingClientRect();
      return { x: event.clientX - rect.left, y: event.clientY - rect.top };
    };

    const onWheel = (event: WheelEvent) => {
      event.preventDefault();
      viewportTouchedRef.current = true;
      const { x, y } = local(event);
      const before = toWorld(x, y);
      const view = viewRef.current;
      const k = Math.max(MIN_ZOOM, Math.min(MAX_ZOOM, view.k * (event.deltaY < 0 ? 1.12 : 0.893)));
      view.k = k;
      view.x = x - before.x * k;
      view.y = y - before.y * k;
      setZoom(k);
    };

    const onDown = (event: PointerEvent) => {
      const { x, y } = local(event);
      const task = nodeAt(x, y);
      pointerRef.current = {
        startX: x,
        startY: y,
        panX: viewRef.current.x,
        panY: viewRef.current.y,
        nodeId: task?.id ?? null,
        moved: false,
      };
      alphaRef.current = 1;
      canvas.setPointerCapture(event.pointerId);
    };

    const onMove = (event: PointerEvent) => {
      const { x, y } = local(event);
      const pointer = pointerRef.current;

      if (pointer) {
        const dx = x - pointer.startX;
        const dy = y - pointer.startY;
        if (Math.abs(dx) + Math.abs(dy) > 3) pointer.moved = true;

        if (pointer.nodeId !== null) {
          const node = nodesRef.current[pointer.nodeId];
          if (node) {
            const world = toWorld(x, y);
            node.x = world.x;
            node.y = world.y;
            node.vx = 0;
            node.vy = 0;
            node.fixed = true;
          }
          alphaRef.current = Math.max(alphaRef.current, 0.7);
        } else {
          viewportTouchedRef.current = true;
          viewRef.current.x = pointer.panX + dx;
          viewRef.current.y = pointer.panY + dy;
        }
        return;
      }

      const task = nodeAt(x, y);
      const id = task?.id ?? null;
      if (id !== hoverRef.current) {
        hoverRef.current = id;
        inputRef.current.onHover(id);
      }
    };

    const onUp = () => {
      const pointer = pointerRef.current;
      pointerRef.current = null;
      if (!pointer) return;
      if (pointer.nodeId !== null) {
        const node = nodesRef.current[pointer.nodeId];
        if (node) node.fixed = false;
        if (!pointer.moved) inputRef.current.onSelect(pointer.nodeId);
      }
    };

    canvas.addEventListener('wheel', onWheel, { passive: false });
    canvas.addEventListener('pointerdown', onDown);
    canvas.addEventListener('pointermove', onMove);
    canvas.addEventListener('pointerup', onUp);
    canvas.addEventListener('pointercancel', onUp);

    return () => {
      canvas.removeEventListener('wheel', onWheel);
      canvas.removeEventListener('pointerdown', onDown);
      canvas.removeEventListener('pointermove', onMove);
      canvas.removeEventListener('pointerup', onUp);
      canvas.removeEventListener('pointercancel', onUp);
    };
  }, [nodeAt, toWorld]);

  const zoomBy = useCallback((factor: number) => {
    viewportTouchedRef.current = true;
    const view = viewRef.current;
    const { w, h } = sizeRef.current;
    const k = Math.max(MIN_ZOOM, Math.min(MAX_ZOOM, view.k * factor));
    const cx = w / 2;
    const cy = h / 2;
    const before = toWorld(cx, cy);
    view.k = k;
    view.x = cx - before.x * k;
    view.y = cy - before.y * k;
    setZoom(k);
  }, [toWorld]);

  const fit = useCallback(() => {
    viewportTouchedRef.current = true;
    const store = nodesRef.current;
    const points = inputRef.current.nodes
      .map((task) => store[task.id])
      .filter((node): node is NodeState => node !== undefined);
    if (points.length === 0) return;

    const xs = points.map((p) => p.x);
    const ys = points.map((p) => p.y);
    const x0 = Math.min(...xs);
    const x1 = Math.max(...xs);
    const y0 = Math.min(...ys);
    const y1 = Math.max(...ys);
    const { w, h } = sizeRef.current;
    const pad = 70;
    const k = Math.max(
      MIN_ZOOM,
      Math.min(2.4, Math.min(w / (x1 - x0 + pad * 2), h / (y1 - y0 + pad * 2))),
    );
    const view = viewRef.current;
    view.k = k;
    view.x = w / 2 - ((x0 + x1) / 2) * k;
    view.y = h / 2 - ((y0 + y1) / 2) * k;
    setZoom(k);
  }, []);

  return { canvasRef: setCanvas, zoom, zoomBy, fit };
}
