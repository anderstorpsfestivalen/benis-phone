import { useCallback, useMemo, useState } from "react";
import {
  ReactFlow,
  Background,
  Controls,
  MiniMap,
  type Edge,
  type Node,
  type NodeMouseHandler,
} from "@xyflow/react";
import "@xyflow/react/dist/style.css";

import type { Action, Fn, Queue } from "../generated/config";
import {
  buildNodesAndEdges,
  type ActionNodeData,
  type FnNodeData,
  type QueueNodeData,
} from "../lib/fn-graph";
import { emptyFn, emptyQueue } from "../lib/empty";
import FnNode from "./FnNode";
import ActionNode from "./ActionNode";
import QueueNode from "./QueueNode";
import FnEditor from "./FnEditor";
import ActionEditor from "./ActionEditor";
import QueueEditor from "./QueueEditor";

const nodeTypes = { fnNode: FnNode, actionNode: ActionNode, queueNode: QueueNode };

type Selection =
  | { kind: "fn"; fnName: string }
  | { kind: "action"; fnName: string; actionIndex: number }
  | { kind: "queue"; queueName: string }
  | null;

export default function FnGraph({
  fns,
  queues,
  entrypoint,
  onFnsChange,
  onQueuesChange,
}: {
  fns: Fn[];
  queues: Queue[];
  entrypoint: string;
  onFnsChange: (fns: Fn[]) => void;
  onQueuesChange: (queues: Queue[]) => void;
}) {
  const [selection, setSelection] = useState<Selection>(null);

  const { nodes, edges } = useMemo(() => {
    const g = buildNodesAndEdges(fns, queues, entrypoint);
    const rfNodes: Node[] = g.nodes.map((n) => {
      let isSelected = false;
      if (selection) {
        if (selection.kind === "fn" && n.type === "fnNode" && n.data.fn.name === selection.fnName) {
          isSelected = true;
        } else if (
          selection.kind === "action" &&
          n.type === "actionNode" &&
          n.data.fnName === selection.fnName &&
          n.data.actionIndex === selection.actionIndex
        ) {
          isSelected = true;
        } else if (
          selection.kind === "queue" &&
          n.type === "queueNode" &&
          n.data.name === selection.queueName
        ) {
          isSelected = true;
        }
      }
      return {
        id: n.id,
        type: n.type,
        position: n.position,
        data: n.data,
        selected: isSelected,
      };
    });
    const rfEdges: Edge[] = g.edges.map((e) => ({
      id: e.id,
      source: e.source,
      target: e.target,
      label: e.label,
      animated: false,
      style: e.data.broken
        ? { stroke: "#f87171", strokeDasharray: "4 4" }
        : e.data.kind === "dispatcher"
          ? { stroke: "#5d737e", strokeDasharray: "6 4" }
          : e.data.kind === "key"
            ? { stroke: "#5d737e", strokeWidth: 1.5 }
            : { stroke: "#5d737e" },
      labelStyle: { fill: "#fcfcfc", fontFamily: "monospace", fontSize: 12 },
      labelBgStyle: { fill: "#02111b" },
      labelBgPadding: [4, 2],
    }));
    return { nodes: rfNodes, edges: rfEdges };
  }, [fns, queues, entrypoint, selection]);

  const onNodeClick = useCallback<NodeMouseHandler>((_, node) => {
    if (node.type === "fnNode") {
      const data = node.data as FnNodeData;
      setSelection({ kind: "fn", fnName: data.fn.name });
    } else if (node.type === "actionNode") {
      const data = node.data as ActionNodeData;
      setSelection({ kind: "action", fnName: data.fnName, actionIndex: data.actionIndex });
    } else if (node.type === "queueNode") {
      const data = node.data as QueueNodeData;
      setSelection({ kind: "queue", queueName: data.name });
    } else {
      setSelection(null);
    }
  }, []);

  const selectedFn = useMemo(() => {
    if (!selection || (selection.kind !== "fn" && selection.kind !== "action")) return null;
    return fns.find((f) => f.name === selection.fnName) ?? null;
  }, [fns, selection]);

  const selectedQueue = useMemo(() => {
    if (!selection || selection.kind !== "queue") return null;
    return queues.find((q) => q.name === selection.queueName) ?? null;
  }, [queues, selection]);

  function addFn() {
    const name = `fn${fns.length + 1}`;
    onFnsChange([...fns, emptyFn(name)]);
    setSelection({ kind: "fn", fnName: name });
  }

  function addQueue() {
    const name = `queue${queues.length + 1}`;
    onQueuesChange([...queues, emptyQueue(name)]);
    setSelection({ kind: "queue", queueName: name });
  }

  function updateSelectedFn(updated: Fn) {
    if (!selection || selection.kind === "queue" || !selectedFn) return;
    const i = fns.findIndex((f) => f.name === selectedFn.name);
    if (i < 0) return;
    const next = [...fns];
    next[i] = updated;
    onFnsChange(next);
    if (updated.name !== selectedFn.name) {
      setSelection(
        selection.kind === "fn"
          ? { kind: "fn", fnName: updated.name }
          : { kind: "action", fnName: updated.name, actionIndex: selection.actionIndex },
      );
    }
  }

  function updateSelectedAction(updated: Action) {
    if (!selection || selection.kind !== "action" || !selectedFn) return;
    const nextActions = [...selectedFn.actions];
    nextActions[selection.actionIndex] = updated;
    updateSelectedFn({ ...selectedFn, actions: nextActions });
  }

  function removeSelectedAction() {
    if (!selection || selection.kind !== "action" || !selectedFn) return;
    const nextActions = selectedFn.actions.filter((_, i) => i !== selection.actionIndex);
    updateSelectedFn({ ...selectedFn, actions: nextActions });
    setSelection({ kind: "fn", fnName: selectedFn.name });
  }

  function updateSelectedQueue(updated: Queue) {
    if (!selection || selection.kind !== "queue" || !selectedQueue) return;
    const i = queues.findIndex((q) => q.name === selectedQueue.name);
    if (i < 0) return;
    const next = [...queues];
    next[i] = updated;
    onQueuesChange(next);
    if (updated.name !== selectedQueue.name) {
      setSelection({ kind: "queue", queueName: updated.name });
    }
  }

  function removeSelected() {
    if (!selection) return;
    if (selection.kind === "fn") {
      if (!confirm(`Remove menu "${selection.fnName}"? Inbound references will become broken.`)) {
        return;
      }
      onFnsChange(fns.filter((f) => f.name !== selection.fnName));
      setSelection(null);
    } else if (selection.kind === "queue") {
      if (!confirm(`Remove queue "${selection.queueName}"? Dispatchers pointing here will break.`)) {
        return;
      }
      onQueuesChange(queues.filter((q) => q.name !== selection.queueName));
      setSelection(null);
    }
  }

  return (
    <div
      className="flex gap-3"
      style={{ height: "calc(100vh - 120px)", minHeight: 500 }}
    >
      <div className="flex-1 min-w-0 border border-shadow-grey rounded overflow-hidden bg-ink-black relative">
        <div className="absolute top-2 left-2 z-10 flex gap-2">
          <button
            onClick={addFn}
            className="px-3 py-1 text-xs font-mono border border-shadow-grey bg-gunmetal text-blue-slate hover:text-white rounded"
          >
            + add menu
          </button>
          <button
            onClick={addQueue}
            className="px-3 py-1 text-xs font-mono border border-shadow-grey bg-gunmetal text-blue-slate hover:text-white rounded"
          >
            + add queue
          </button>
          {(selection?.kind === "fn" || selection?.kind === "queue") && (
            <button
              onClick={removeSelected}
              className="px-3 py-1 text-xs font-mono border border-shadow-grey bg-gunmetal text-blue-slate hover:text-white rounded"
            >
              remove "{selection.kind === "fn" ? selection.fnName : selection.queueName}"
            </button>
          )}
        </div>
        <ReactFlow
          nodes={nodes}
          edges={edges}
          nodeTypes={nodeTypes}
          onNodeClick={onNodeClick}
          fitView
          nodesDraggable={false}
          nodesConnectable={false}
          edgesFocusable={false}
          colorMode="dark"
        >
          <Background color="#3f4045" gap={20} />
          <Controls className="!bg-gunmetal !border-shadow-grey" />
          <MiniMap
            pannable
            zoomable
            maskColor="rgba(2, 17, 27, 0.8)"
            nodeColor="#5d737e"
            style={{ background: "#02111b" }}
          />
        </ReactFlow>
      </div>

      <div className="w-[560px] shrink-0 border border-shadow-grey rounded bg-gunmetal/40 overflow-y-auto">
        {!selection && (
          <div className="p-6 text-sm text-blue-slate">
            <p>
              Click a node to edit. The {entrypoint || "main"} menu is the call entrypoint.
            </p>
            <p className="mt-3 text-xs leading-relaxed">
              Menu nodes branch on a DTMF key into action nodes. Each action node shows
              its kind; routing actions then connect to the next menu or queue.
              Solid lines: menu links. Dashed lines: dispatcher → queue. Red dashed: broken.
            </p>
          </div>
        )}

        {selection?.kind === "fn" && selectedFn && (
          <div className="p-3">
            <FnEditor
              value={selectedFn}
              onChange={updateSelectedFn}
              onSelectAction={(i) =>
                setSelection({ kind: "action", fnName: selectedFn.name, actionIndex: i })
              }
            />
          </div>
        )}

        {selection?.kind === "action" && selectedFn && selectedFn.actions[selection.actionIndex] && (
          <div className="p-3 flex flex-col gap-3">
            <div className="flex items-center justify-between text-xs">
              <button
                onClick={() => setSelection({ kind: "fn", fnName: selectedFn.name })}
                className="text-blue-slate hover:text-white font-mono"
              >
                ← back to menu "{selectedFn.name}"
              </button>
              <span className="text-blue-slate uppercase tracking-wider">
                action {selection.actionIndex + 1} / {selectedFn.actions.length}
              </span>
            </div>
            <ActionEditor
              value={selectedFn.actions[selection.actionIndex]}
              knownFnNames={fns.map((f) => f.name).filter(Boolean)}
              onChange={updateSelectedAction}
              onRemove={removeSelectedAction}
            />
          </div>
        )}

        {selection?.kind === "queue" && selectedQueue && (
          <div className="p-3">
            <QueueEditor value={selectedQueue} onChange={updateSelectedQueue} />
          </div>
        )}
      </div>
    </div>
  );
}
