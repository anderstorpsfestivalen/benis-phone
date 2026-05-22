import { Handle, Position, type NodeProps } from "@xyflow/react";
import type { QueueNodeData } from "../lib/fn-graph";

export default function QueueNode({ data, selected }: NodeProps & { data: QueueNodeData }) {
  const { name, queue } = data;
  const broken = !queue;
  return (
    <div
      className={`bg-shadow-grey border-2 ${
        selected ? "border-white" : broken ? "border-red-400" : "border-blue-slate/60"
      } rounded font-mono text-xs text-white px-3 py-2 cursor-pointer`}
      style={{ width: 280 }}
    >
      <Handle type="target" position={Position.Left} className="!bg-blue-slate" isConnectable={false} />
      <div className="flex items-center justify-between gap-2">
        <span className="truncate flex-1">{name}</span>
        <span className="text-[10px] text-blue-slate uppercase tracking-wide shrink-0">
          {broken ? "missing" : "queue"}
        </span>
      </div>
      {queue && (
        <div className="text-[10px] text-blue-slate mt-1">
          pos {queue.minpos}–{queue.maxpos}, speed {queue.speed}s
        </div>
      )}
    </div>
  );
}
