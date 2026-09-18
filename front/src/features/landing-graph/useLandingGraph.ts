import { useCallback, useEffect, useRef } from 'react';

import { pad2 } from '@/shared/lib/ascii';

interface BgNode {
  id: string;
  /** Normalized 0..1 coordinates, so the layout survives any canvas size. */
  x: number;
  y: number;
  vx: number;
  vy: number;
  r: number;
  done: boolean;
  label: boolean;
}

interface BgEdge {
  a: number;
  b: number;
  /** Position of the travelling pulse, negative while it waits its turn. */
  pulse: number;
}

const NODE_COUNT = 30;

/** Deterministic LCG with the design's seed, so the backdrop is always the same. */
function seededRandom(): () => number {
  let s = 20260918;
  return () => (s = (s * 1103515245 + 12345) & 0x7fffffff) / 0x7fffffff;
}

function buildGraph(): { nodes: BgNode[]; edges: BgEdge[] } {
  const random = seededRandom();
  const nodes: BgNode[] = [];

  for (let i = 0; i < NODE_COUNT; i += 1) {
    nodes.push({
      id: `N${pad2(i + 1)}`,
      x: random(),
      y: random(),
      vx: (random() - 0.5) * 0.000075,
      vy: (random() - 0.5) * 0.000075,
      r: random() < 0.18 ? 3.5 : random() < 0.55 ? 2.6 : 1.9,
      done: random() < 0.3,
      label: i % 5 === 2,
    });
  }

  // every node links to its two or three nearest neighbours
  const edges: BgEdge[] = [];
  const seen = new Set<string>();
  nodes.forEach((node, i) => {
    const near = nodes
      .map((other, j) => ({ j, d: Math.hypot(other.x - node.x, other.y - node.y) }))
      .filter((candidate) => candidate.j !== i)
      .sort((a, b) => a.d - b.d)
      .slice(0, random() < 0.3 ? 3 : 2);

    near.forEach((candidate) => {
      const key = `${Math.min(i, candidate.j)}-${Math.max(i, candidate.j)}`;
      if (seen.has(key)) return;
      seen.add(key);
      edges.push({ a: i, b: candidate.j, pulse: -random() * 7 });
    });
  });

  return { nodes, edges };
}

/**
 * Animated task network drawn behind the landing hero: drifting nodes, edges
 * that fade with distance, and green pulses travelling along them.
 * Honours `prefers-reduced-motion` by rendering a still frame.
 */
export function useLandingGraph(): (element: HTMLCanvasElement | null) => void {
  const canvasRef = useRef<HTMLCanvasElement | null>(null);
  const graphRef = useRef(buildGraph());

  const setCanvas = useCallback((element: HTMLCanvasElement | null) => {
    canvasRef.current = element;
  }, []);

  useEffect(() => {
    const canvas = canvasRef.current;
    const ctx = canvas?.getContext('2d');
    if (!canvas || !ctx) return undefined;

    const { nodes, edges } = graphRef.current;
    const random = seededRandom();
    let width = 0;
    let height = 0;
    let raf = 0;

    const resize = () => {
      const dpr = Math.min(window.devicePixelRatio || 1, 2);
      width = canvas.clientWidth;
      height = canvas.clientHeight;
      canvas.width = Math.round(width * dpr);
      canvas.height = Math.round(height * dpr);
      ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
    };

    resize();
    window.addEventListener('resize', resize);

    const still =
      typeof window.matchMedia === 'function' &&
      window.matchMedia('(prefers-reduced-motion: reduce)').matches;

    let last = performance.now();

    const frame = (now: number) => {
      const dt = Math.min(now - last, 48);
      last = now;
      if (width === 0 || height === 0) resize();

      ctx.clearRect(0, 0, width, height);

      if (!still) {
        nodes.forEach((node) => {
          node.x += node.vx * dt;
          node.y += node.vy * dt;
          if (node.x < 0.02 || node.x > 0.98) node.vx *= -1;
          if (node.y < 0.02 || node.y > 0.98) node.vy *= -1;
        });
      }

      edges.forEach((edge) => {
        const a = nodes[edge.a];
        const b = nodes[edge.b];
        if (!a || !b) return;

        const ax = a.x * width;
        const ay = a.y * height;
        const bx = b.x * width;
        const by = b.y * height;
        const distance = Math.hypot(bx - ax, by - ay);
        const limit = Math.max(width, height) * 0.52;
        if (distance > limit) return;

        const k = 1 - distance / limit;
        ctx.lineWidth = 0.8 + k * 0.35;
        ctx.strokeStyle = `rgba(154,168,176,${(0.07 + k * 0.2).toFixed(3)})`;
        ctx.beginPath();
        ctx.moveTo(ax, ay);
        ctx.lineTo(bx, by);
        ctx.stroke();

        if (!still) edge.pulse += dt / 2200;

        if (edge.pulse > 0 && edge.pulse < 1) {
          const px = ax + (bx - ax) * edge.pulse;
          const py = ay + (by - ay) * edge.pulse;
          ctx.fillStyle = `rgba(95,179,127,${(0.64 * Math.sin(edge.pulse * Math.PI)).toFixed(3)})`;
          ctx.beginPath();
          ctx.arc(px, py, 2, 0, Math.PI * 2);
          ctx.fill();
        } else if (edge.pulse >= 1) {
          edge.pulse = -1.5 - random() * 9;
        }
      });

      nodes.forEach((node) => {
        const x = node.x * width;
        const y = node.y * height;

        // knock a hole in the edges behind the node
        ctx.fillStyle = '#0a0b0c';
        ctx.beginPath();
        ctx.arc(x, y, node.r + 2.6, 0, Math.PI * 2);
        ctx.fill();

        ctx.fillStyle = node.done ? 'rgba(128,140,146,0.46)' : 'rgba(216,225,230,0.76)';
        ctx.beginPath();
        ctx.arc(x, y, node.r, 0, Math.PI * 2);
        ctx.fill();

        if (node.r > 3) {
          ctx.lineWidth = 1;
          ctx.strokeStyle = 'rgba(95,179,127,0.42)';
          ctx.beginPath();
          ctx.arc(x, y, node.r + 4.5, 0, Math.PI * 2);
          ctx.stroke();
        }

        if (node.label) {
          ctx.font = '9px "JetBrains Mono", monospace';
          ctx.fillStyle = 'rgba(152,166,174,0.38)';
          ctx.fillText(node.id, x + 7, y + 3);
        }
      });

      raf = requestAnimationFrame(frame);
    };

    raf = requestAnimationFrame(frame);

    return () => {
      window.removeEventListener('resize', resize);
      cancelAnimationFrame(raf);
    };
  }, []);

  return setCanvas;
}
