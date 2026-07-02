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
	sameSource,
	type ActionNodeData,
	type ActionSource,
	type FnNodeData,
	type QueueNodeData,
} from "../lib/fn-graph";
import { emptyAction, emptyFn, emptyQueue } from "../lib/empty";
import FnNode from "./FnNode";
import ActionNode from "./ActionNode";
import QueueNode from "./QueueNode";
import FnEditor from "./FnEditor";
import ActionEditor from "./ActionEditor";
import QueueEditor from "./QueueEditor";

const nodeTypes = {
	fnNode: FnNode,
	actionNode: ActionNode,
	queueNode: QueueNode,
};

type Selection =
	| { kind: "fn"; fnName: string }
	| { kind: "action"; source: ActionSource }
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
				if (
					selection.kind === "fn" &&
					n.type === "fnNode" &&
					n.data.fn.name === selection.fnName
				) {
					isSelected = true;
				} else if (
					selection.kind === "action" &&
					n.type === "actionNode" &&
					sameSource(n.data.source, selection.source)
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
				width: n.width,
				height: n.height,
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
			labelStyle: {
				fill: "#fcfcfc",
				fontFamily: "monospace",
				fontSize: 12,
			},
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
			setSelection({ kind: "action", source: data.source });
		} else if (node.type === "queueNode") {
			const data = node.data as QueueNodeData;
			setSelection({ kind: "queue", queueName: data.name });
		} else {
			setSelection(null);
		}
	}, []);

	// The action editor + the surrounding side-panel chrome both need the
	// currently-selected Action and the writer that puts it back. These
	// helpers reduce the "is it an fn action or queue.end" branching to a
	// single place.
	const actionForSelection = useMemo<{
		action: Action;
		parentLabel: string;
		onChange: (a: Action) => void;
		onRemove: (() => void) | null;
	} | null>(() => {
		if (!selection || selection.kind !== "action") return null;
		const src = selection.source;
		if (src.kind === "fn") {
			const fn = fns.find((f) => f.name === src.fnName);
			if (!fn || !fn.actions[src.actionIndex]) return null;
			return {
				action: fn.actions[src.actionIndex],
				parentLabel: `menu "${src.fnName}"`,
				onChange: (a) => {
					const next = [...fn.actions];
					next[src.actionIndex] = a;
					updateFnByName(src.fnName, { ...fn, actions: next });
				},
				onRemove: () => {
					const next = fn.actions.filter(
						(_, i) => i !== src.actionIndex,
					);
					updateFnByName(src.fnName, { ...fn, actions: next });
					setSelection({ kind: "fn", fnName: src.fnName });
				},
			};
		}
		// queue-end
		const q = queues.find((x) => x.name === src.queueName);
		if (!q) return null;
		return {
			action: q.end,
			parentLabel: `queue "${src.queueName}" (end)`,
			onChange: (a) => updateQueueByName(src.queueName, { ...q, end: a }),
			// Queue.end is structural — it always exists. "Remove" resets it
			// back to an empty action rather than deleting the slot.
			onRemove: () => {
				updateQueueByName(src.queueName, { ...q, end: emptyAction() });
			},
		};
		// eslint-disable-next-line react-hooks/exhaustive-deps
	}, [selection, fns, queues]);

	const selectedFn = useMemo(() => {
		if (!selection || selection.kind !== "fn") return null;
		return fns.find((f) => f.name === selection.fnName) ?? null;
	}, [fns, selection]);

	const selectedQueue = useMemo(() => {
		if (!selection || selection.kind !== "queue") return null;
		return queues.find((q) => q.name === selection.queueName) ?? null;
	}, [queues, selection]);

	function updateFnByName(name: string, next: Fn) {
		const i = fns.findIndex((f) => f.name === name);
		if (i < 0) return;
		const arr = [...fns];
		arr[i] = next;
		onFnsChange(arr);
		if (next.name !== name && selection) {
			// Rename: keep the selection pointed at the same logical thing.
			if (selection.kind === "fn" && selection.fnName === name) {
				setSelection({ kind: "fn", fnName: next.name });
			} else if (
				selection.kind === "action" &&
				selection.source.kind === "fn" &&
				selection.source.fnName === name
			) {
				setSelection({
					kind: "action",
					source: { ...selection.source, fnName: next.name },
				});
			}
		}
	}

	function updateQueueByName(name: string, next: Queue) {
		const i = queues.findIndex((q) => q.name === name);
		if (i < 0) return;
		const arr = [...queues];
		arr[i] = next;
		onQueuesChange(arr);
		if (next.name !== name && selection) {
			if (selection.kind === "queue" && selection.queueName === name) {
				setSelection({ kind: "queue", queueName: next.name });
			} else if (
				selection.kind === "action" &&
				selection.source.kind === "queue-end" &&
				selection.source.queueName === name
			) {
				setSelection({
					kind: "action",
					source: { kind: "queue-end", queueName: next.name },
				});
			}
		}
	}

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

	function removeSelected() {
		if (!selection) return;
		if (selection.kind === "fn") {
			if (
				!confirm(
					`Remove menu "${selection.fnName}"? Inbound references will become broken.`,
				)
			) {
				return;
			}
			onFnsChange(fns.filter((f) => f.name !== selection.fnName));
			setSelection(null);
		} else if (selection.kind === "queue") {
			if (
				!confirm(
					`Remove queue "${selection.queueName}"? Dispatchers pointing here will break.`,
				)
			) {
				return;
			}
			onQueuesChange(
				queues.filter((q) => q.name !== selection.queueName),
			);
			setSelection(null);
		}
	}

	const removeLabel =
		selection?.kind === "fn"
			? selection.fnName
			: selection?.kind === "queue"
				? selection.queueName
				: "";

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
					{(selection?.kind === "fn" ||
						selection?.kind === "queue") && (
						<button
							onClick={removeSelected}
							className="px-3 py-1 text-xs font-mono border border-shadow-grey bg-gunmetal text-blue-slate hover:text-white rounded"
						>
							remove "{removeLabel}"
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
							Click a node to edit. The {entrypoint || "main"}{" "}
							menu is the call entrypoint.
						</p>
						<p className="mt-3 text-xs leading-relaxed">
							Menus branch on a DTMF key into action nodes. Queues
							also have an action node — labelled <em>end</em> —
							that runs when the queue terminates; configure it
							like any other action. Routing actions connect on to
							the next menu or queue. Solid lines: menu links.
							Dashed lines: dispatcher → queue. Red dashed:
							broken.
						</p>
					</div>
				)}

				{selection?.kind === "fn" && selectedFn && (
					<div className="p-3">
						<FnEditor
							value={selectedFn}
							onChange={(next) =>
								updateFnByName(selectedFn.name, next)
							}
							onSelectAction={(i) =>
								setSelection({
									kind: "action",
									source: {
										kind: "fn",
										fnName: selectedFn.name,
										actionIndex: i,
									},
								})
							}
						/>
					</div>
				)}

				{selection?.kind === "action" && actionForSelection && (
					<div className="p-3 flex flex-col gap-3">
						<div className="flex items-center justify-between text-xs">
							<button
								onClick={() => {
									if (selection.source.kind === "fn") {
										setSelection({
											kind: "fn",
											fnName: selection.source.fnName,
										});
									} else {
										setSelection({
											kind: "queue",
											queueName:
												selection.source.queueName,
										});
									}
								}}
								className="text-blue-slate hover:text-white font-mono"
							>
								← back to {actionForSelection.parentLabel}
							</button>
						</div>
						<ActionEditor
							value={actionForSelection.action}
							knownFnNames={fns
								.map((f) => f.name)
								.filter(Boolean)}
							onChange={actionForSelection.onChange}
							onRemove={actionForSelection.onRemove ?? (() => {})}
						/>
					</div>
				)}

				{selection?.kind === "queue" && selectedQueue && (
					<div className="p-3">
						<QueueEditor
							value={selectedQueue}
							onChange={(next) =>
								updateQueueByName(selectedQueue.name, next)
							}
							onSelectEnd={() =>
								setSelection({
									kind: "action",
									source: {
										kind: "queue-end",
										queueName: selectedQueue.name,
									},
								})
							}
						/>
					</div>
				)}
			</div>
		</div>
	);
}
