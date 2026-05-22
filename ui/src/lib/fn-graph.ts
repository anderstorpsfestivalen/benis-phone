// Pure helpers that turn the Fn[]/Queue[] portion of a Definition into the
// nodes/edges React Flow renders, plus a dagre-driven auto-layout pass.
//
// Model: every Fn is a header node; every Action is its own node connected
// to its parent fn by a DTMF-key-labelled edge; dst/dispatcher actions get
// outgoing edges to their target fn/queue node.

import dagre from "dagre";
import type { Action, Fn, Queue } from "../generated/config";
import { actionKind, type ActionKind } from "../generated/config";

export type FnNodeData = {
  kind: "fn";
  fn: Fn;
  index: number;
  isEntry: boolean;
};

export type ActionNodeData = {
  kind: "action";
  fnName: string;
  actionIndex: number;
  action: Action;
  actionKind: ActionKind | null;
};

export type QueueNodeData = {
  kind: "queue";
  queue: Queue | null;
  name: string;
};

export type GraphNode =
  | { id: string; type: "fnNode"; position: { x: number; y: number }; data: FnNodeData }
  | { id: string; type: "actionNode"; position: { x: number; y: number }; data: ActionNodeData }
  | { id: string; type: "queueNode"; position: { x: number; y: number }; data: QueueNodeData };

export type GraphEdge = {
  id: string;
  source: string;
  target: string;
  label: string;
  data: { kind: "key" | "dst" | "dispatcher"; broken: boolean };
};

const FN_NODE_WIDTH = 220;
const FN_NODE_HEIGHT = 60;
const ACTION_NODE_WIDTH = 240;
const ACTION_NODE_HEIGHT = 88;
const QUEUE_NODE_WIDTH = 220;
const QUEUE_NODE_HEIGHT = 56;

export function buildNodesAndEdges(
  fns: Fn[],
  queues: Queue[],
  entrypoint: string,
): { nodes: GraphNode[]; edges: GraphEdge[] } {
  const fnNames = new Set(fns.map((f) => f.name).filter(Boolean));
  const queueNames = new Set(queues.map((q) => q.name).filter(Boolean));
  // Queues are always rendered as their own nodes (so newly-added queues
  // are reachable in the editor). Dispatchers create edges; if they point
  // to a name that isn't a registered queue, we still create a placeholder
  // node so the broken reference is visible.
  const danglingQueueRefs = new Set<string>();

  const nodes: GraphNode[] = [];
  const edges: GraphEdge[] = [];

  fns.forEach((fn, i) => {
    if (!fn.name) return;
    const fnId = `fn_${fn.name}`;
    nodes.push({
      id: fnId,
      type: "fnNode",
      position: { x: 0, y: 0 },
      data: { kind: "fn", fn, index: i, isEntry: fn.name === entrypoint },
    });

    fn.actions.forEach((action, j) => {
      const actionId = `act_${fn.name}_${j}`;
      nodes.push({
        id: actionId,
        type: "actionNode",
        position: { x: 0, y: 0 },
        data: {
          kind: "action",
          fnName: fn.name,
          actionIndex: j,
          action,
          actionKind: actionKind(action),
        },
      });

      // fn → action edge, labelled with the DTMF key
      edges.push({
        id: `e_${fn.name}_${j}_key`,
        source: fnId,
        target: actionId,
        label: dtmfLabel(action.num),
        data: { kind: "key", broken: false },
      });

      // action → target edge, only for dst / dispatcher
      const target = dispatcherOrDst(action);
      if (target) {
        if (target.kind === "dispatcher" && !queueNames.has(target.target)) {
          danglingQueueRefs.add(target.target);
        }
        const broken =
          target.kind === "dst"
            ? !fnNames.has(target.target)
            : !queueNames.has(target.target);
        edges.push({
          id: `e_${fn.name}_${j}_to`,
          source: actionId,
          target:
            target.kind === "dst"
              ? `fn_${target.target}`
              : `queue_${target.target}`,
          label: "",
          data: { kind: target.kind, broken },
        });
      }
    });
  });

  // Real queues — always rendered so newly-added unreferenced queues are
  // visible/clickable.
  queues.forEach((q) => {
    if (!q.name) return;
    nodes.push({
      id: `queue_${q.name}`,
      type: "queueNode",
      position: { x: 0, y: 0 },
      data: { kind: "queue", queue: q, name: q.name },
    });
  });
  // Broken dispatcher targets — placeholder nodes so the red edge has
  // something to terminate on.
  for (const name of danglingQueueRefs) {
    nodes.push({
      id: `queue_${name}`,
      type: "queueNode",
      position: { x: 0, y: 0 },
      data: { kind: "queue", queue: null, name },
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

export function runDagreLayout(nodes: GraphNode[], edges: GraphEdge[]) {
  const g = new dagre.graphlib.Graph();
  g.setGraph({ rankdir: "LR", nodesep: 24, ranksep: 80, marginx: 20, marginy: 20 });
  g.setDefaultEdgeLabel(() => ({}));

  for (const n of nodes) {
    const dims =
      n.type === "fnNode"
        ? { width: FN_NODE_WIDTH, height: FN_NODE_HEIGHT }
        : n.type === "actionNode"
          ? { width: ACTION_NODE_WIDTH, height: ACTION_NODE_HEIGHT }
          : { width: QUEUE_NODE_WIDTH, height: QUEUE_NODE_HEIGHT };
    g.setNode(n.id, dims);
  }
  for (const e of edges) g.setEdge(e.source, e.target);

  dagre.layout(g);

  for (const n of nodes) {
    const pos = g.node(n.id);
    if (!pos) continue;
    n.position = { x: pos.x - pos.width / 2, y: pos.y - pos.height / 2 };
  }
}

export function dtmfLabel(num: number): string {
  if (num === 10) return "*";
  if (num === 11) return "#";
  return String(num);
}

/** One-line description of an action — used inside the action node body. */
export function actionDetail(a: Action): string {
  const k = actionKind(a);
  switch (k) {
    case "dst":
      return `→ ${a.dst}`;
    case "dispatcher":
      return `→ ${a.dispatcher}`;
    case "tts":
      return truncate(a.tts.msg, 40);
    case "file":
      return a.file.src;
    case "randomfile":
      return a.randomfile.folder;
    case "srv":
      return a.srv.dst;
    case "transfer":
      return a.transfer;
    case "record":
      return a.record;
    case "dtmf":
      return a.dtmf;
    case "livefeed":
      return a.livefeed?.device || "default device";
    case "hangup":
    case "clear":
      return "";
    default:
      return "(empty)";
  }
}

function truncate(s: string, n: number): string {
  if (s.length <= n) return s;
  return s.slice(0, n - 1) + "…";
}

/** Visual classification of action kinds for node styling. */
export type ActionCategory = "route" | "speak" | "media" | "control" | "service";

export function categoryFor(kind: ActionKind | null): ActionCategory {
  switch (kind) {
    case "dst":
    case "dispatcher":
      return "route";
    case "tts":
      return "speak";
    case "file":
    case "randomfile":
    case "livefeed":
      return "media";
    case "srv":
      return "service";
    default:
      return "control";
  }
}
