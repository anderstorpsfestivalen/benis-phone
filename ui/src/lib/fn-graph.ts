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

// Actions live in two places in the config: as elements of `Fn.actions`
// and as the singleton `Queue.end`. We unify their representation in the
// graph so both look and edit identically.
export type ActionSource =
  | { kind: "fn"; fnName: string; actionIndex: number }
  | { kind: "queue-end"; queueName: string };

export type ActionNodeData = {
  kind: "action";
  source: ActionSource;
  action: Action;
  actionKind: ActionKind | null;
};

export function actionNodeId(source: ActionSource): string {
  return source.kind === "fn"
    ? `act_fn_${source.fnName}_${source.actionIndex}`
    : `act_queue_${source.queueName}_end`;
}

export function sameSource(a: ActionSource, b: ActionSource): boolean {
  if (a.kind !== b.kind) return false;
  if (a.kind === "fn" && b.kind === "fn") {
    return a.fnName === b.fnName && a.actionIndex === b.actionIndex;
  }
  if (a.kind === "queue-end" && b.kind === "queue-end") {
    return a.queueName === b.queueName;
  }
  return false;
}

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
const QUEUE_NODE_HEIGHT = 80;

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
      const source: ActionSource = { kind: "fn", fnName: fn.name, actionIndex: j };
      const actionId = actionNodeId(source);
      nodes.push({
        id: actionId,
        type: "actionNode",
        position: { x: 0, y: 0 },
        data: {
          kind: "action",
          source,
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
      appendActionTargetEdge(edges, actionId, `e_${fn.name}_${j}_to`, action, fnNames, queueNames, danglingQueueRefs);
    });
  });

  // Real queues — always rendered so newly-added unreferenced queues are
  // visible/clickable. Each queue's `end` action also becomes a node so the
  // post-queue flow is visible end-to-end.
  queues.forEach((q) => {
    if (!q.name) return;
    const qId = `queue_${q.name}`;
    nodes.push({
      id: qId,
      type: "queueNode",
      position: { x: 0, y: 0 },
      data: { kind: "queue", queue: q, name: q.name },
    });

    // queue.end is just an Action. Treat it identically to fn actions so
    // the existing ActionEditor and ActionNode pick it up unchanged.
    const endSource: ActionSource = { kind: "queue-end", queueName: q.name };
    const endId = actionNodeId(endSource);
    nodes.push({
      id: endId,
      type: "actionNode",
      position: { x: 0, y: 0 },
      data: {
        kind: "action",
        source: endSource,
        action: q.end,
        actionKind: actionKind(q.end),
      },
    });
    edges.push({
      id: `e_q_${q.name}_end`,
      source: qId,
      target: endId,
      label: "end",
      data: { kind: "key", broken: false },
    });
    appendActionTargetEdge(edges, endId, `e_q_${q.name}_end_to`, q.end, fnNames, queueNames, danglingQueueRefs);
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
  enforceKeypadOrder(nodes, fns);
  return { nodes, edges };
}

// Phone-keypad order: 1,2,3,4,5,6,7,8,9,0,#,*. Dagre's barycentric
// within-rank ordering doesn't know about DTMF semantics, so its layout
// of an fn's action children comes out arbitrary. We keep dagre's
// computed y-slots (they're correctly spaced and centered) but
// re-assign which action occupies which slot so they read top-to-bottom
// in keypad order.
function keypadRank(num: number): number {
  if (num >= 1 && num <= 9) return num;
  if (num === 0) return 10;
  if (num === 11) return 11; // #
  if (num === 10) return 12; // *
  return 99;
}

function enforceKeypadOrder(nodes: GraphNode[], fns: Fn[]) {
  const byId = new Map(nodes.map((n) => [n.id, n] as const));
  for (const fn of fns) {
    if (!fn.name || fn.actions.length < 2) continue;
    const entries = fn.actions
      .map((action, j) => {
        const id = actionNodeId({ kind: "fn", fnName: fn.name, actionIndex: j });
        const node = byId.get(id);
        return node ? { node, rank: keypadRank(action.num) } : null;
      })
      .filter((e): e is { node: GraphNode; rank: number } => e !== null);
    if (entries.length < 2) continue;
    const slotsY = entries.map((e) => e.node.position.y).sort((a, b) => a - b);
    entries.sort((a, b) => a.rank - b.rank);
    entries.forEach((e, i) => {
      e.node.position = { ...e.node.position, y: slotsY[i] };
    });
  }
}

function dispatcherOrDst(a: Action):
  | { kind: "dst"; target: string }
  | { kind: "dispatcher"; target: string }
  | null {
  if (a.dst) return { kind: "dst", target: a.dst };
  if (a.dispatcher) return { kind: "dispatcher", target: a.dispatcher };
  return null;
}

// appendActionTargetEdge wires an action node to whatever fn/queue it
// routes to (if any). Shared between fn-action edges and queue-end edges
// so both follow the same routing-line semantics.
function appendActionTargetEdge(
  edges: GraphEdge[],
  sourceId: string,
  edgeId: string,
  action: Action,
  fnNames: Set<string>,
  queueNames: Set<string>,
  danglingQueueRefs: Set<string>,
) {
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
      id: edgeId,
      source: sourceId,
      target:
        target.kind === "dst" ? `fn_${target.target}` : `queue_${target.target}`,
      label: "",
      data: { kind: target.kind, broken },
    });
  }

  // A listmenu routes to its dst fn after the caller selects an option.
  if (action.listmenu && action.listmenu.dst) {
    edges.push({
      id: `${edgeId}_list`,
      source: sourceId,
      target: `fn_${action.listmenu.dst}`,
      label: "",
      data: { kind: "dst", broken: !fnNames.has(action.listmenu.dst) },
    });
  }

  // `then` auto-advances to another fn once the action's audio finishes.
  if (action.then) {
    edges.push({
      id: `${edgeId}_then`,
      source: sourceId,
      target: `fn_${action.then}`,
      label: "auto",
      data: { kind: "dst", broken: !fnNames.has(action.then) },
    });
  }
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
    case "genericjson":
      return a.genericjson.url;
    case "interactive":
      return `▶ ${a.interactive.dst}`;
    case "listmenu":
      return `▤ list → ${a.listmenu.dst || "?"}`;
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
    case "genericjson":
    case "interactive":
    case "listmenu":
      return "service";
    default:
      return "control";
  }
}
