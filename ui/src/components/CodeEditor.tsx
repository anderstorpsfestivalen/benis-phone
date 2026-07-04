import { useEffect, useRef } from "react";
import { EditorView, keymap, lineNumbers, highlightActiveLine } from "@codemirror/view";
import { EditorState, type Extension } from "@codemirror/state";
import { defaultKeymap, history, historyKeymap, indentWithTab } from "@codemirror/commands";
import { javascript } from "@codemirror/lang-javascript";
import {
  syntaxTree,
  bracketMatching,
  indentOnInput,
  HighlightStyle,
  syntaxHighlighting,
} from "@codemirror/language";
import { linter, lintGutter, type Diagnostic } from "@codemirror/lint";
import { tags as t } from "@lezer/highlight";

// jsErrorLinter walks the Lezer tree and reports every error node as a
// diagnostic. Lezer parses error-tolerantly, so an incomplete/invalid script
// leaves ⚠ error nodes we can surface inline — a cheap, dependency-free syntax
// check that matches what goja would reject at compile time (close enough for
// authoring; the definitive check is Prepare() re-compiling on save).
const jsErrorLinter = linter((view: EditorView): Diagnostic[] => {
  const diagnostics: Diagnostic[] = [];
  syntaxTree(view.state)
    .cursor()
    .iterate((node) => {
      if (node.type.isError) {
        diagnostics.push({
          from: node.from,
          to: Math.max(node.to, node.from + 1),
          severity: "error",
          message: "Syntax error",
        });
      }
    });
  return diagnostics;
});

// Dark theme keyed to the 5-color palette (tailwind.config.ts). CodeMirror
// themes are plain JS (not Tailwind classes), so hex values are used directly.
const palette = {
  bg: "#02111b", // ink-black
  gutter: "#30292f", // shadow-grey
  text: "#fcfcfc", // white
  muted: "#5d737e", // blue-slate
};

// Dark-friendly syntax colors (basicSetup's defaultHighlightStyle is tuned for
// light backgrounds, so we define our own against the ink-black editor bg).
const highlightStyle = HighlightStyle.define([
  { tag: t.keyword, color: "#c792ea" },
  { tag: [t.controlKeyword, t.moduleKeyword], color: "#c792ea" },
  { tag: [t.string, t.special(t.string)], color: "#c3e88d" },
  { tag: [t.number, t.bool, t.null], color: "#f78c6c" },
  { tag: t.comment, color: "#5d737e", fontStyle: "italic" },
  { tag: [t.function(t.variableName), t.function(t.propertyName)], color: "#82aaff" },
  { tag: t.propertyName, color: "#82aaff" },
  { tag: [t.operator, t.operatorKeyword], color: "#89ddff" },
  { tag: t.punctuation, color: "#89ddff" },
  { tag: [t.definition(t.variableName), t.variableName], color: "#fcfcfc" },
  { tag: t.regexp, color: "#f78c6c" },
]);

const theme = EditorView.theme(
  {
    "&": {
      color: palette.text,
      backgroundColor: palette.bg,
      fontSize: "13px",
      border: "1px solid #30292f",
      borderRadius: "4px",
    },
    ".cm-content": { fontFamily: "ui-monospace, SFMono-Regular, Menlo, monospace" },
    ".cm-gutters": {
      backgroundColor: palette.gutter,
      color: palette.muted,
      border: "none",
    },
    ".cm-activeLine": { backgroundColor: "#3f404533" },
    ".cm-activeLineGutter": { backgroundColor: "#3f404533" },
    "&.cm-focused": { outline: "none", borderColor: "#5d737e" },
    ".cm-cursor": { borderLeftColor: palette.text },
    ".cm-selectionBackground, &.cm-focused .cm-selectionBackground": {
      backgroundColor: "#5d737e66",
    },
  },
  { dark: true },
);

type Props = {
  value: string;
  onChange: (v: string) => void;
  minHeight?: string;
};

// CodeEditor is a controlled CodeMirror 6 JavaScript editor with inline syntax
// linting. It keeps the external `value` in sync without tearing down the view
// on every keystroke (which would lose cursor/undo).
export default function CodeEditor({ value, onChange, minHeight = "220px" }: Props) {
  const host = useRef<HTMLDivElement | null>(null);
  const view = useRef<EditorView | null>(null);
  // Keep the latest onChange in a ref so the update listener doesn't need to
  // be rebuilt (which would require recreating the whole editor).
  const onChangeRef = useRef(onChange);
  onChangeRef.current = onChange;

  useEffect(() => {
    if (!host.current) return;
    const extensions: Extension[] = [
      lineNumbers(),
      highlightActiveLine(),
      history(),
      bracketMatching(),
      indentOnInput(),
      keymap.of([...defaultKeymap, ...historyKeymap, indentWithTab]),
      javascript(),
      syntaxHighlighting(highlightStyle),
      jsErrorLinter,
      lintGutter(),
      theme,
      EditorView.lineWrapping,
      EditorView.theme({ ".cm-content": { minHeight } }),
      EditorView.updateListener.of((u) => {
        if (u.docChanged) onChangeRef.current(u.state.doc.toString());
      }),
    ];
    const v = new EditorView({
      state: EditorState.create({ doc: value, extensions }),
      parent: host.current,
    });
    view.current = v;
    return () => {
      v.destroy();
      view.current = null;
    };
    // Create once on mount; value sync is handled by the effect below.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // Push external value changes (e.g. Format, node switch) into the editor.
  useEffect(() => {
    const v = view.current;
    if (!v) return;
    const current = v.state.doc.toString();
    if (current !== value) {
      v.dispatch({ changes: { from: 0, to: current.length, insert: value } });
    }
  }, [value]);

  return <div ref={host} className="w-full overflow-hidden rounded" />;
}
