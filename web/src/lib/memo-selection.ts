import { useSyncExternalStore } from "react";

/** Multi-select state for batch export, shared by the filter row and memo cards. */
interface SelectionState {
  active: boolean;
  selected: ReadonlySet<string>;
  /** Names of the memos currently loaded in the list, for "select all". */
  visible: readonly string[];
}

let state: SelectionState = { active: false, selected: new Set(), visible: [] };
const listeners = new Set<() => void>();

function set(next: Partial<SelectionState>) {
  state = { ...state, ...next };
  for (const l of listeners) l();
}

export const memoSelection = {
  start(initial?: string) {
    set({ active: true, selected: new Set(initial ? [initial] : []) });
  },
  exit() {
    if (state.active || state.selected.size) set({ active: false, selected: new Set() });
  },
  toggle(name: string) {
    const selected = new Set(state.selected);
    if (selected.has(name)) selected.delete(name);
    else selected.add(name);
    set({ selected });
  },
  selectAll() {
    set({ selected: new Set(state.visible) });
  },
  clear() {
    set({ selected: new Set() });
  },
  setVisible(visible: readonly string[]) {
    if (visible.length === state.visible.length && visible.every((v, i) => v === state.visible[i])) return;
    set({ visible });
  },
};

const subscribe = (l: () => void) => {
  listeners.add(l);
  return () => listeners.delete(l);
};

export function useMemoSelection<T>(selector: (s: SelectionState) => T): T {
  return useSyncExternalStore(subscribe, () => selector(state));
}
