// Pure helpers that turn the Fn[]/Queue[] portion of a Definition into the
// nodes/edges React Flow renders, plus a dagre-driven auto-layout pass.
//
// The fn/queue list IS the source of truth — positions are recomputed on
// every render, so saving the doc never persists graph coordinates.

import dagre from "dagre";
import type { Action, Fn, Queue } from "../generated/config";
import { actionKind } from "../generated/config";

export type FnNodeData = {
  kind: "fn";
  fn: Fn;
  index: number;
  isEntry: boolean;
};

export type QueueNodeData = {
  kind: "queue";
  queue: Queue | null; // null for dangling dispatcher references
  name: string;
};

export type GraphNode =
  | { id: string; type: "fnNode"; position: { x: number; y: number }; data: FnNodeData }
  | { id: string; type: "queueNode"; position: { x: number; y: number }; data: QueueNodeData };

export type GraphEdge = {
  id: string;
  source: string;
  target: string;
  label: string;
  data: { kind: "dst" | "dispatcher"; broken: boolean };
};

const NODE_WIDTH = 280;
const NODE_HEIGHT_PER_ACTION = 22;
const NODE_HEIGHT_BASE = 64;

/** Derive nodes & edges from the current fn/queue arrays. */
export function buildNodesAndEdges(
  fns: Fn[],
  queues: Queue[],
  entrypoint: string,
): { nodes: GraphNode[]; edges: GraphEdge[] } {
  const fnNames = new Set(fns.map((f) => f.name).filter(Boolean));
  const queueNames = new Set(queues.map((q) => q.name).filter(Boolean));
  // Queue nodes only get rendered if some fn actually references them as
  // a dispatcher target. Otherwise the canvas fills with orphans.
  const referencedQueues = new Set<string>();

  const edges: GraphEdge[] = [];
  fns.forEach((fn, i) => {
    if (!fn.name) return;
    fn.actions.forEach((action, j) => {
      const target = dispatcherOrDst(action);
      if (!target) return;
      if (target.kind === "dispatcher") referencedQueues.add(target.target);
      const broken =
        target.kind === "dst"
          ? !fnNames.has(target.target)
          : !queueNames.has(target.target);
      edges.push({
        id: `e_${i}_${j}`,
        source: `fn_${fn.name}`,
        target:
          target.kind === "dst"
            ? `fn_${target.target}`
            : `queue_${target.target}`,
        label: dtmfLabel(action.num),
        data: { kind: target.kind, broken },
      });
    });
  });

  const nodes: GraphNode[] = [];

  fns.forEach((fn, i) => {
    if (!fn.name) return;
    nodes.push({
      id: `fn_${fn.name}`,
      type: "fnNode",
      position: { x: 0, y: 0 },
      data: { kind: "fn", fn, index: i, isEntry: fn.name === entrypoint },
    });
  });

  for (const name of referencedQueues) {
    const q = queues.find((q) => q.name === name) ?? null;
    nodes.push({
      id: `queue_${name}`,
      type: "queueNode",
      position: { x: 0, y: 0 },
      data: { kind: "queue", queue: q, name },
    });
  }

  runDagreLayout(nodes, edges);
  return { nodes, edges };
}

function dispatcherOrDst(a: Action):
  | { kind: "dst"; target: string }
  | { kind: "dispatcher"; target: string }
  | null {
  if (a.dst) return { kind: "dst", target: a.dst };
  if (a.dispatcher) return { kind: "dispatcher", target: a.dispatcher };
  return null;
}

/** Mutates node positions in place. Left-to-right layered graph. */
export function runDagreLayout(nodes: GraphNode[], edges: GraphEdge[]) {
  const g = new dagre.graphlib.Graph();
  g.setGraph({ rankdir: "LR", nodesep: 40, ranksep: 80, marginx: 20, marginy: 20 });
  g.setDefaultEdgeLabel(() => ({}));

  for (const n of nodes) {
    const height =
      n.type === "fnNode"
        ? NODE_HEIGHT_BASE + n.data.fn.actions.length * NODE_HEIGHT_PER_ACTION
        : 60;
    g.setNode(n.id, { width: NODE_WIDTH, height });
  }
  for (const e of edges) g.setEdge(e.source, e.target);

  dagre.layout(g);

  for (const n of nodes) {
    const pos = g.node(n.id);
    if (!pos) continue;
    n.position = { x: pos.x - pos.width / 2, y: pos.y - pos.height / 2 };
  }
}

/** Render the DTMF key as it would appear on the keypad. */
export function dtmfLabel(num: number): string {
  if (num === 10) return "*";
  if (num === 11) return "#";
  return String(num);
}

/** One-line human-readable description of an action's kind + target. */
export function actionSummary(a: Action): string {
  const k = actionKind(a);
  switch (k) {
    case "dst":
      return `→ menu ${a.dst}`;
    case "dispatcher":
      return `→ queue ${a.dispatcher}`;
    case "tts":
      return `tts "${truncate(a.tts.msg, 30)}"`;
    case "file":
      return `file ${a.file.src}`;
    case "randomfile":
      return `randomfile ${a.randomfile.folder}`;
    case "srv":
      return `srv ${a.srv.dst}`;
    case "transfer":
      return `transfer ${a.transfer}`;
    case "hangup":
      return "hangup";
    case "record":
      return `record ${a.record}`;
    case "dtmf":
      return `dtmf ${a.dtmf}`;
    case "livefeed":
      return `livefeed ${a.livefeed?.device || "default"}`;
    case "clear":
      return "clear";
    default:
      return "(empty)";
  }
}

function truncate(s: string, n: number): string {
  if (s.length <= n) return s;
  return s.slice(0, n - 1) + "…";
}
