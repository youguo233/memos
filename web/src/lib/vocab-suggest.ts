import wordListText from "./english-words.txt?raw";

const ENGLISH_WORDS = wordListText
  .split(/\r?\n/)
  .map((word) => word.trim())
  .filter(Boolean);

export type VocabSuggestion = {
  term: string;
  existing?: boolean;
};

export function suggestVocabTerms(query: string, knownTerms: string[], limit = 8): VocabSuggestion[] {
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
