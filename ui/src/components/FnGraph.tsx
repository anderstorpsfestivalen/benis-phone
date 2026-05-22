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

import type { Fn, Queue } from "../generated/config";
import { buildNodesAndEdges } from "../lib/fn-graph";
import { emptyFn } from "../lib/empty";
import FnNode from "./FnNode";
import QueueNode from "./QueueNode";
import FnEditor from "./FnEditor";

const nodeTypes = { fnNode: FnNode, queueNode: QueueNode };

export default function FnGraph({
  fns,
  queues,
  entrypoint,
  onChange,
}: {
  fns: Fn[];
  queues: Queue[];
  entrypoint: string;
  onChange: (fns: Fn[]) => void;
}) {
  const [selectedName, setSelectedName] = useState<string | null>(null);

  const { nodes, edges } = useMemo(() => {
    const g = buildNodesAndEdges(fns, queues, entrypoint);
    const rfNodes: Node[] = g.nodes.map((n) => ({
      id: n.id,
      type: n.type,
      position: n.position,
      data: n.data,
      selected: n.type === "fnNode" && n.data.fn.name === selectedName,
    }));
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
          : { stroke: "#5d737e" },
      labelStyle: { fill: "#fcfcfc", fontFamily: "monospace", fontSize: 12 },
      labelBgStyle: { fill: "#02111b" },
      labelBgPadding: [4, 2],
    }));
    return { nodes: rfNodes, edges: rfEdges };
  }, [fns, queues, entrypoint, selectedName]);

  const onNodeClick = useCallback<NodeMouseHandler>((_, node) => {
    if (node.type === "fnNode") {
      const data = node.data as { fn: Fn };
      setSelectedName(data.fn.name);
    } else {
      setSelectedName(null);
    }
  }, []);

  const selected = useMemo(
    () => fns.find((f) => f.name === selectedName) ?? null,
    [fns, selectedName],
  );
  const knownFnNames = useMemo(
    () => fns.map((f) => f.name).filter(Boolean),
    [fns],
  );

  function addFn() {
    const name = `fn${fns.length + 1}`;
    const next = [...fns, emptyFn(name)];
    onChange(next);
    setSelectedName(name);
  }

  function updateSelected(updated: Fn) {
    const i = fns.findIndex((f) => f.name === selectedName);
    if (i < 0) return;
    const next = [...fns];
    next[i] = updated;
    onChange(next);
    // If the user renamed, follow the selection.
    if (updated.name !== selectedName) setSelectedName(updated.name);
  }

  function removeSelected() {
    if (!selectedName) return;
    if (!confirm(`Remove fn "${selectedName}"? Inbound references will become broken.`)) {
      return;
    }
    onChange(fns.filter((f) => f.name !== selectedName));
    setSelectedName(null);
  }

  return (
    <div className="flex gap-3" style={{ height: "calc(100vh - 220px)", minHeight: 500 }}>
      <div className="flex-1 border border-shadow-grey rounded overflow-hidden bg-ink-black relative">
        <div className="absolute top-2 left-2 z-10 flex gap-2">
          <button
            onClick={addFn}
            className="px-3 py-1 text-xs font-mono border border-shadow-grey bg-gunmetal text-blue-slate hover:text-white rounded"
          >
            + add fn
          </button>
          {selected && (
            <button
              onClick={removeSelected}
              className="px-3 py-1 text-xs font-mono border border-shadow-grey bg-gunmetal text-blue-slate hover:text-white rounded"
            >
              remove "{selected.name}"
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

      <div className="w-[480px] border border-shadow-grey rounded bg-gunmetal/40 overflow-y-auto">
        {selected ? (
          <div className="p-3">
            <FnEditor
              value={selected}
              knownFnNames={knownFnNames}
              onChange={updateSelected}
            />
          </div>
        ) : (
          <div className="p-6 text-sm text-blue-slate">
            <p>Click a node to edit. The {entrypoint || "main"} fn is the call entrypoint.</p>
            <p className="mt-3 text-xs">
              Edges show DTMF routing. Solid lines link menus (dst); dashed lines link
              dispatchers to queues. Red dashed lines indicate a missing target.
            </p>
          </div>
        )}
      </div>
    </div>
  );
}
