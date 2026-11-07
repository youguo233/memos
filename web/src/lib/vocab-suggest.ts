import wordListText from "./english-words.txt?raw";

// Frequency-ordered English words for local prefix suggestions (no extra server calls).
// Source: hermitdave/FrequencyWords content/2018/en/en_50k.txt (OpenSubtitles-based).
const ENGLISH_WORDS = wordListText
  .split(/\r?\n/)
  .map((line) => line.trim())
  .filter((line) => line && !line.startsWith("#"))
  .map((line) => line.split(/\s+/)[0]?.toLowerCase() ?? "")
  .filter((word) => /^[a-z]+(?:[-'][a-z]+)*$/.test(word));

export type VocabSuggestion = {
  term: string;
  existing?: boolean;
};

export function suggestVocabTerms(query: string, knownTerms: string[], limit = 10): VocabSuggestion[] {
  const needle = query.trim().toLowerCase();
  if (needle.length < 2) return [];

  const suggestions: VocabSuggestion[] = [];
  const seen = new Set<string>();

  for (const term of knownTerms) {
    const normalized = term.toLowerCase();
    if (!normalized.startsWith(needle) || normalized === needle || seen.has(normalized)) continue;
    suggestions.push({ term, existing: true });
    seen.add(normalized);
    if (suggestions.length >= Math.min(3, limit)) break;
  }

  for (const term of ENGLISH_WORDS) {
    if (!term.startsWith(needle) || term === needle || seen.has(term)) continue;
    suggestions.push({ term });
    seen.add(term);
    if (suggestions.length >= limit) break;
  }

  return suggestions;
}
