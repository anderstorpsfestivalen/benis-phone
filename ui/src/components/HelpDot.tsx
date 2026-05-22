import { useLayoutEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";

// Tiny `?` affordance: hover or press-and-hold to show a detailed tooltip.
// Tooltip is rendered in a portal so the side-panel's overflow:hidden never
// clips it, and positioned with getBoundingClientRect so it always stays in
// the viewport (flips to the left of the dot if it would overflow right).

const TOOLTIP_WIDTH = 280;
const GAP = 8;

export default function HelpDot({ help }: { help: string }) {
  const dotRef = useRef<HTMLSpanElement | null>(null);
  const [open, setOpen] = useState(false);
  const [pos, setPos] = useState<{ left: number; top: number } | null>(null);

  useLayoutEffect(() => {
    if (!open || !dotRef.current) return;
    const rect = dotRef.current.getBoundingClientRect();
    const vw = window.innerWidth;
    const vh = window.innerHeight;
    // Default: tooltip starts to the right of the dot.
    let left = rect.right + GAP;
    if (left + TOOLTIP_WIDTH > vw - GAP) {
      // Not enough room on the right — flip to the left.
      left = rect.left - TOOLTIP_WIDTH - GAP;
    }
    if (left < GAP) left = GAP;
    // Vertically centered on the dot, clamped to the viewport.
    let top = rect.top + rect.height / 2 - 20;
    if (top + 80 > vh - GAP) top = vh - 80 - GAP;
    if (top < GAP) top = GAP;
    setPos({ left, top });
  }, [open]);

  if (!help) return null;

  return (
    <>
      <span
        ref={dotRef}
        className="relative inline-flex items-center justify-center align-middle ml-1 cursor-help"
        onMouseEnter={() => setOpen(true)}
        onMouseLeave={() => setOpen(false)}
        onMouseDown={(e) => {
          e.preventDefault();
          setOpen(true);
        }}
        onMouseUp={() => setOpen(false)}
        onTouchStart={(e) => {
          e.preventDefault();
          setOpen(true);
        }}
        onTouchEnd={() => setOpen(false)}
      >
        <span
          className="inline-flex items-center justify-center rounded-full border border-blue-slate text-blue-slate text-[8px] leading-none"
          style={{ width: 12, height: 12 }}
        >
          ?
        </span>
      </span>
      {open && pos && createPortal(
        <span
          role="tooltip"
          className="fixed z-[1000] p-2 bg-shadow-grey border border-blue-slate rounded text-xs text-white shadow-lg normal-case tracking-normal"
          style={{
            left: pos.left,
            top: pos.top,
            width: TOOLTIP_WIDTH,
            whiteSpace: "pre-wrap",
            pointerEvents: "none",
          }}
        >
          {help}
        </span>,
        document.body,
      )}
    </>
  );
}
