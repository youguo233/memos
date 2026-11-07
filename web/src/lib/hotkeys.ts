/** True when a keypress is meant to type into the focused field rather than run a shortcut. */
export function isTypingTarget(target: EventTarget | null): boolean {
  if (!(target instanceof HTMLElement)) return false;
  if (target.isContentEditable) return true;
  if (target.closest("[contenteditable='true']")) return true;
  if (target instanceof HTMLTextAreaElement || target instanceof HTMLSelectElement) return true;
  if (target instanceof HTMLInputElement) {
    const type = target.type;
    return type !== "button" && type !== "submit" && type !== "reset" && type !== "checkbox" && type !== "radio" && type !== "file";
  }
  return false;
}
