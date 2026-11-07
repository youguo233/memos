/**
 * iOS IME helpers for the memo editor.
 *
 * - `installIosSelectionGuard` (always on iOS): after typed or composed input, iOS Safari can leave the
 *   committed text selected on an empty line. If the selection is a range inside the editor right after
 *   input and the user has not touched or used a selection key since, collapse it to its end.
 * - `installImeDebug` (only with `?imedebug=1`, remembered for the tab; `?imedebug=0` turns it off): an
 *   on-screen log of input, composition and selection events, and of every script call that moves the
 *   selection, with a short stack, so the event order can be screen-recorded on a real device.
 */

const EDITOR_ROOT = ".mbe-root";
const DEBUG_KEY = "memos.imedebug";

type Logger = (message: string) => void;

declare global {
  interface Window {
    __memosImeDebug?: Logger;
  }
}

export const isIOS = (): boolean => {
  if (typeof navigator === "undefined") return false;
  const ua = navigator.userAgent;
  return /AppleWebKit/.test(ua) && (/iPhone|iPad|iPod/.test(ua) || (/Macintosh/.test(ua) && navigator.maxTouchPoints > 1));
};

const log: Logger = (message) => window.__memosImeDebug?.(message);

const editorRootOf = (node: Node | null): Element | null => {
  const el = node instanceof Element ? node : (node?.parentElement ?? null);
  return el?.closest(EDITOR_ROOT) ?? null;
};

const describeNode = (node: Node | null): string => {
  if (!node) return "null";
  if (node.nodeType === Node.TEXT_NODE) return `#text(${JSON.stringify((node as Text).data.slice(0, 12))})`;
  const el = node as Element;
  const cls = typeof el.className === "string" && el.className ? `.${el.className.split(/\s+/).slice(0, 3).join(".")}` : "";
  return `${el.nodeName.toLowerCase()}${cls}`;
};

const describeSelection = (): string => {
  const sel = document.getSelection();
  if (!sel || !sel.anchorNode) return "sel=none";
  return `sel ${describeNode(sel.anchorNode)}:${sel.anchorOffset} -> ${describeNode(sel.focusNode)}:${sel.focusOffset}${sel.isCollapsed ? " (caret)" : ` [${JSON.stringify(sel.toString().slice(0, 20))}]`}`;
};

export function installIosSelectionGuard(): void {
  if (typeof window === "undefined" || !isIOS()) return;
  let composing = false;
  let userGestureAt = 0;

  const markGesture = () => {
    userGestureAt = performance.now();
  };
  document.addEventListener("touchstart", markGesture, { capture: true, passive: true });
  document.addEventListener("pointerdown", markGesture, { capture: true, passive: true });
  document.addEventListener(
    "keydown",
    (e) => {
      if (e.shiftKey || e.metaKey || e.ctrlKey || e.altKey) markGesture();
    },
    true,
  );
  document.addEventListener("compositionstart", () => (composing = true), true);

  const check = (when: string, since: number) => {
    if (composing || userGestureAt > since) return;
    const sel = document.getSelection();
    if (!sel || sel.isCollapsed || !sel.anchorNode || !sel.focusNode) return;
    const root = editorRootOf(sel.anchorNode);
    if (!root || root !== editorRootOf(sel.focusNode)) return;
    log(`guard(${when}): collapsing unexpected range ${describeSelection()}`);
    sel.collapseToEnd();
  };
  const schedule = (target: EventTarget | null) => {
    if (!editorRootOf(target as Node | null)) return;
    const since = performance.now();
    setTimeout(() => check("timeout", since), 0);
    requestAnimationFrame(() => check("raf", since));
    setTimeout(() => check("80ms", since), 80);
  };
  document.addEventListener(
    "compositionend",
    (e) => {
      composing = false;
      schedule(e.target);
    },
    true,
  );
  document.addEventListener(
    "input",
    (e) => {
      if (!composing && !(e as InputEvent).isComposing) schedule(e.target);
    },
    true,
  );
}

const shortStack = (): string =>
  (new Error().stack ?? "")
    .split("\n")
    .slice(2, 6)
    .map((line) =>
      line
        .trim()
        .replace(/^at /, "")
        .replace(/\(?https?:\/\/[^/]+\//, "(")
        .slice(0, 90),
    )
    .join(" < ");

export function installImeDebug(): void {
  if (typeof window === "undefined") return;
  const param = new URLSearchParams(window.location.search).get("imedebug");
  if (param === "0") sessionStorage.removeItem(DEBUG_KEY);
  if (param === "1") sessionStorage.setItem(DEBUG_KEY, "1");
  if (sessionStorage.getItem(DEBUG_KEY) !== "1") return;

  const panel = document.createElement("div");
  panel.setAttribute(
    "style",
    "position:fixed;top:env(safe-area-inset-top,0);left:0;right:0;z-index:2147483647;max-height:38vh;overflow:hidden;" +
      "background:rgba(0,0,0,.78);color:#0f0;font:10px/1.25 ui-monospace,Menlo,monospace;padding:2px 4px;pointer-events:none;white-space:pre-wrap;word-break:break-all",
  );
  const lines: string[] = [];
  let t0 = performance.now();
  const write: Logger = (message) => {
    const t = (performance.now() - t0).toFixed(0).padStart(6);
    lines.push(`${t} ${message}`);
    if (lines.length > 60) lines.shift();
    panel.textContent = lines.slice(-28).join("\n");
    console.debug("[imedebug]", message);
  };
  window.__memosImeDebug = write;
  const mount = () => document.body && !panel.isConnected && document.body.appendChild(panel);
  if (document.body) mount();
  else document.addEventListener("DOMContentLoaded", mount);
  write(`imedebug on · ${navigator.userAgent.slice(0, 80)}`);

  // Events.
  const inEditor = (e: Event) => !!editorRootOf(e.target as Node | null);
  for (const type of ["beforeinput", "input"] as const) {
    document.addEventListener(
      type,
      (e) => {
        if (!inEditor(e)) return;
        const ie = e as InputEvent;
        const ranges = type === "beforeinput" ? ie.getTargetRanges?.() : undefined;
        const target = ranges?.[0] ? ` tr=${describeNode(ranges[0].startContainer)}:${ranges[0].startOffset}-${ranges[0].endOffset}` : "";
        const data = ie.data ?? ie.dataTransfer?.getData("text/plain") ?? "";
        // Read defaultPrevented after the editor's handler ran.
        setTimeout(() =>
          write(
            `${type} ${ie.inputType} ${JSON.stringify(data)} comp=${ie.isComposing} prevented=${ie.defaultPrevented}${target} | ${describeSelection()}`,
          ),
        );
      },
      true,
    );
  }
  for (const type of ["compositionstart", "compositionupdate", "compositionend"] as const) {
    document.addEventListener(
      type,
      (e) => inEditor(e) && write(`${type} ${JSON.stringify((e as CompositionEvent).data)} | ${describeSelection()}`),
      true,
    );
  }
  document.addEventListener("keydown", (e) => inEditor(e) && write(`keydown ${e.key} code=${e.keyCode} comp=${e.isComposing}`), true);
  document.addEventListener("touchstart", (e) => inEditor(e) && write("touchstart"), { capture: true, passive: true });
  document.addEventListener("selectionchange", () => {
    const sel = document.getSelection();
    if (sel?.anchorNode && editorRootOf(sel.anchorNode)) write(`selectionchange ${describeSelection()}`);
  });
  panel.addEventListener("dblclick", () => {
    lines.length = 0;
    t0 = performance.now();
  });

  // Script calls that move the selection.
  const wrap = <T extends object>(proto: T, name: keyof T & string, label: string) => {
    const original = proto[name] as unknown as (...args: unknown[]) => unknown;
    if (typeof original !== "function") return;
    (proto as Record<string, unknown>)[name] = function (this: unknown, ...args: unknown[]) {
      const touches =
        args.some((a) => a instanceof Node && editorRootOf(a)) || (this instanceof Range && editorRootOf(this.startContainer));
      if (touches || (label === "Selection" && editorRootOf(document.getSelection()?.anchorNode ?? null))) {
        const argText = args.map((a) => (a instanceof Node ? describeNode(a) : a instanceof Range ? "Range" : String(a))).join(",");
        write(`CALL ${label}.${name}(${argText}) @ ${shortStack()}`);
      }
      return original.apply(this, args);
    };
  };
  for (const name of [
    "setBaseAndExtent",
    "addRange",
    "removeAllRanges",
    "collapse",
    "collapseToEnd",
    "collapseToStart",
    "extend",
    "selectAllChildren",
  ] as const) {
    wrap(Selection.prototype, name, "Selection");
  }
  for (const name of ["selectNodeContents", "selectNode"] as const) wrap(Range.prototype, name, "Range");
  const focus = HTMLElement.prototype.focus;
  HTMLElement.prototype.focus = function (this: HTMLElement, options?: FocusOptions) {
    if (editorRootOf(this) || this.matches(EDITOR_ROOT)) write(`CALL focus(${describeNode(this)}) @ ${shortStack()}`);
    return focus.call(this, options);
  };
}
