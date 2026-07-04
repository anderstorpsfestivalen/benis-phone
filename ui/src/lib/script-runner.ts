// script-runner runs a script node's JS in an isolated Web Worker so the
// editor can "Test it" without saving the config or ringing the IVR. The
// worker mirrors the Go runtime bindings (speak/readKey/http/vars/args/goto/
// log) with test doubles:
//   - speak(text)        → records a transcript line
//   - readKey()          → shifts the next key from a caller-supplied sequence
//                          (null when the sequence is exhausted)
//   - http.get/post      → synchronous XHR through /api/genericjson/preview
//                          (same-origin, behind Cloudflare Access), so the JS
//                          stays blocking just like goja
//   - vars.get/set       → an in-memory object
//   - goto(fn, param)    → records the exit target
//
// Caveat: this runs in the browser's V8, not goja (ES5.1+), so the newest JS
// APIs may behave differently at runtime. It's a faithful enough smoke test
// for flow logic and HTTP shape.

export type TranscriptEvent =
  | { type: "speak"; text: string }
  | { type: "readKey"; key: string | null }
  | { type: "http"; method: string; url: string; status: number }
  | { type: "goto"; fn: string; param: unknown }
  | { type: "log"; args: string[] };

export type ScriptRunResult = {
  ok: boolean;
  transcript: TranscriptEvent[];
  gotoTarget: string | null;
  vars: Record<string, unknown>;
  error?: string;
};

// The worker body. Kept as a string so it can be wrapped in a Blob URL — fully
// self-contained, no separate worker entry file to wire through Vite.
const WORKER_SRC = /* js */ `
self.onmessage = function (e) {
  var code = e.data.code;
  var args = e.data.args || {};
  var keys = (e.data.keys || []).slice();
  var origin = e.data.origin;
  var transcript = [];
  var vars = {};
  var gotoTarget = null;

  function proxyFetch(method, url, body, opts) {
    var xhr = new XMLHttpRequest();
    xhr.open("POST", origin + "/api/genericjson/preview", false); // sync, like goja
    xhr.setRequestHeader("Content-Type", "application/json");
    var payload = { url: url, method: method };
    if (body) payload.body = body;
    if (opts && opts.headers) payload.headers = opts.headers;
    xhr.send(JSON.stringify(payload));
    if (xhr.status < 200 || xhr.status >= 300) {
      throw new Error("preview proxy failed: HTTP " + xhr.status + " " + xhr.responseText);
    }
    var res = JSON.parse(xhr.responseText);
    if (res.error) throw new Error(res.error);
    var json = null;
    try { json = JSON.parse(res.body); } catch (_) { json = null; }
    transcript.push({ type: "http", method: method, url: url, status: res.status });
    return { status: res.status, json: json, text: res.body };
  }

  var http = {
    get: function (url, opts) { return proxyFetch("GET", url, undefined, opts); },
    post: function (url, body, opts) {
      var b = typeof body === "string" ? body : (body == null ? "" : JSON.stringify(body));
      return proxyFetch("POST", url, b, opts);
    },
  };
  function readKey() {
    var k = keys.length ? keys.shift() : null;
    transcript.push({ type: "readKey", key: k });
    return k;
  }
  function speak(text) { transcript.push({ type: "speak", text: String(text) }); }
  var varsApi = {
    get: function (n) { return vars[n]; },
    set: function (n, v) { vars[n] = v; },
  };
  function gotoFn(fn, param) {
    gotoTarget = String(fn);
    if (param !== undefined) vars["goto"] = param;
    transcript.push({ type: "goto", fn: gotoTarget, param: param === undefined ? null : param });
  }
  function log() {
    transcript.push({ type: "log", args: Array.prototype.slice.call(arguments).map(String) });
  }

  try {
    var fn = new Function("http", "readKey", "speak", "vars", "args", "goto", "log", code);
    fn(http, readKey, speak, varsApi, args, gotoFn, log);
    self.postMessage({ ok: true, transcript: transcript, gotoTarget: gotoTarget, vars: vars });
  } catch (err) {
    self.postMessage({
      ok: false,
      transcript: transcript,
      gotoTarget: gotoTarget,
      vars: vars,
      error: (err && err.message) ? err.message : String(err),
    });
  }
};
`;

const TIMEOUT_MS = 15_000;

export function runScriptTest(input: {
  code: string;
  args: Record<string, string>;
  keys: string[];
}): Promise<ScriptRunResult> {
  return new Promise((resolve) => {
    const blob = new Blob([WORKER_SRC], { type: "text/javascript" });
    const url = URL.createObjectURL(blob);
    const worker = new Worker(url);

    const cleanup = () => {
      worker.terminate();
      URL.revokeObjectURL(url);
    };

    const timer = setTimeout(() => {
      cleanup();
      resolve({
        ok: false,
        transcript: [],
        gotoTarget: null,
        vars: {},
        error: `script timed out after ${TIMEOUT_MS / 1000}s (infinite loop, or a slow/blocked HTTP call?)`,
      });
    }, TIMEOUT_MS);

    worker.onmessage = (e: MessageEvent<ScriptRunResult>) => {
      clearTimeout(timer);
      cleanup();
      resolve(e.data);
    };
    worker.onerror = (e) => {
      clearTimeout(timer);
      cleanup();
      resolve({
        ok: false,
        transcript: [],
        gotoTarget: null,
        vars: {},
        error: e.message || "worker error",
      });
    };

    worker.postMessage({
      code: input.code,
      args: input.args,
      keys: input.keys,
      origin: self.location.origin,
    });
  });
}
