// Pure helpers that turn the Fn[]/Queue[] portion of a Definition into the
// nodes/edges React Flow renders, plus a dagre-driven auto-layout pass.
//
// Model: every Fn is a header node; every Action is its own node connected
// to its parent fn by a DTMF-key-labelled edge; dst/dispatcher actions get
// outgoing edges to their target fn/queue node.

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

// width/height are stamped by runTreeLayout from the layout dimensions.
// React Flow needs explicit node geometry for the MiniMap to draw the node
// rectangles (custom nodes aren't always measured by the time it renders).
export type GraphNode =
  | { id: string; type: "fnNode"; position: { x: number; y: number }; width?: number; height?: number; data: FnNodeData }
  | { id: string; type: "actionNode"; position: { x: number; y: number }; width?: number; height?: number; data: ActionNodeData }
  | { id: string; type: "queueNode"; position: { x: number; y: number }; width?: number; height?: number; data: QueueNodeData };

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

  runTreeLayout(nodes, edges, entrypoint);
  return { nodes, edges };
}

// Phone-keypad order: 1,2,3,4,5,6,7,8,9,0,#,*. Used to feed actions to dagre
// in keypad order so its within-rank tie-breaks follow the keypad; dagre's
// crossing-minimization then aligns routing actions with their target menus
// (flow order) where that removes a crossing.
function keypadRank(num: number): number {
  if (num >= 1 && num <= 9) return num;
  if (num === 0) return 10;
  if (num === 11) return 11; // #
  if (num === 10) return 12; // *
  return 99;
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

const NODE_W: Record<GraphNode["type"], number> = {
  fnNode: FN_NODE_WIDTH,
  actionNode: ACTION_NODE_WIDTH,
  queueNode: QUEUE_NODE_WIDTH,
};
const NODE_H: Record<GraphNode["type"], number> = {
  fnNode: FN_NODE_HEIGHT,
  actionNode: ACTION_NODE_HEIGHT,
  queueNode: QUEUE_NODE_HEIGHT,
};

// runTreeLayout is a left-to-right "tidy tree" layout tailored to IVR flows.
// Unlike a generic crossing-minimizer, it treats the graph as a tree rooted at
// the entrypoint and:
//   - orders each node's children in KEYPAD order (1,2,…,0,#,*), so a menu's
//     actions always read top-to-bottom by DTMF key;
//   - centers every parent on the vertical span of its children, so a routing
//     action lands on the same row as the menu/queue it points at (the flow
//     stays aligned with the keypad row it came from);
//   - stacks leaves with real per-node heights, so a tall subtree (e.g. a menu
//     with three transfers) gets its own vertical room instead of overlapping
//     its siblings.
// Nodes reached by more than one edge are laid out on first visit; the extra
// edges are drawn as-is (they may cross — rare for menu trees). Orphan fns /
// queues become additional roots stacked below the main tree.
export function runTreeLayout(
  nodes: GraphNode[],
  edges: GraphEdge[],
  entrypoint: string,
) {
  const byId = new Map(nodes.map((n) => [n.id, n] as const));
  const COL_W = 320; // horizontal distance between ranks (> widest node)
  const ROW_GAP = 28; // vertical gap between stacked leaves

  const outEdges = new Map<string, GraphEdge[]>();
  for (const e of edges) {
    if (!byId.has(e.source) || !byId.has(e.target)) continue;
    const list = outEdges.get(e.source);
    if (list) list.push(e);
    else outEdges.set(e.source, [e]);
  }

  // Children in draw order: action children (via DTMF "key" edges) sort by
  // keypad rank; other targets keep their edge order.
  const childrenOf = (id: string): string[] =>
    (outEdges.get(id) ?? [])
      .map((e, i) => {
        const t = byId.get(e.target)!;
        const rank =
          t.type === "actionNode" && t.data.kind === "action"
            ? keypadRank(t.data.action.num)
            : Number.MAX_SAFE_INTEGER;
        return { id: e.target, rank, i };
      })
      .sort((a, b) => a.rank - b.rank || a.i - b.i)
      .map((c) => c.id);

  const visited = new Set<string>();
  let cursor = 0; // running y for the next leaf's top edge

  // Places the subtree rooted at id and returns the node's center-y.
  const place = (id: string, depth: number): number => {
    const n = byId.get(id)!;
    const h = NODE_H[n.type];
    if (visited.has(id)) return n.position.y + h / 2;
    visited.add(id);
    n.width = NODE_W[n.type];
    n.height = h;

    const kids = childrenOf(id).filter((c) => !visited.has(c));
    let centerY: number;
    if (kids.length === 0) {
      centerY = cursor + h / 2;
      cursor += h + ROW_GAP;
    } else {
      const centers = kids.map((c) => place(c, depth + 1));
      centerY = (centers[0] + centers[centers.length - 1]) / 2;
    }
    n.position = { x: depth * COL_W, y: centerY - h / 2 };
    return centerY;
  };

  // Roots: entrypoint first, then any unvisited fn/queue (orphans), then any
  // stray node — each starts a fresh tree at depth 0, stacked below.
  const rootOrder = [
    `fn_${entrypoint}`,
    ...nodes.filter((n) => n.type === "fnNode" || n.type === "queueNode").map((n) => n.id),
    ...nodes.map((n) => n.id),
  ];
  for (const id of rootOrder) {
    if (byId.has(id) && !visited.has(id)) place(id, 0);
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
