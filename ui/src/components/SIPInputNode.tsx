import { Handle, Position, type NodeProps } from "@xyflow/react";
import type { SIPInputNodeData } from "../lib/fn-graph";

export default function SIPInputNode({
  data,
  selected,
}: NodeProps & { data: SIPInputNodeData }) {
  return (
    <div
      className={`bg-gunmetal border-2 ${selected ? "border-white" : "border-success"} rounded-lg font-mono text-white px-4 py-3 cursor-pointer`}
      style={{ width: 220 }}
    >
      <div className="flex gap-2 text-xs">
        <span className="text-success uppercase">SIP in</span>
        <span className="truncate flex-1 text-right">{data.label}</span>
      </div>
      <div className="text-[10px] text-blue-slate text-right truncate">
        {data.detail}
      </div>
      <Handle
        type="source"
        position={Position.Right}
        className="!bg-success"
        isConnectable={false}
      />
    </div>
  );
}
