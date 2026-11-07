package vocablookup

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

func ensureJSONInstruction(prompt string) string {
	if strings.Contains(strings.ToLower(prompt), "json") {
		return prompt
	}
	return strings.TrimSpace(prompt) + "\n\nReply with JSON only, no markdown fences, using keys phonetic, definition, example, and notes."
}

func ParseLLMVocabJSON(content string) (*Result, error) {
	for _, candidate := range extractJSONCandidates(content) {
		if out, err := decodeVocabJSON(candidate); err == nil {
			return out, nil
		}
		if out, err := decodeVocabJSON(repairLLMJSON(candidate)); err == nil {
			return out, nil
		}
	}
	return nil, fmt.Errorf("the model response was not valid JSON")
}

func decodeVocabJSON(content string) (*Result, error) {
	var raw struct {
		Phonetic      flexText `json:"phonetic"`
		IPA           flexText `json:"ipa"`
		Pronunciation flexText `json:"pronunciation"`
		Definition    flexText `json:"definition"`
		Definitions   flexText `json:"definitions"`
		Meaning       flexText `json:"meaning"`
		Meanings      flexText `json:"meanings"`
		Example       flexText `json:"example"`
		Examples      flexText `json:"examples"`
		Notes         flexText `json:"notes"`
		Note          flexText `json:"note"`
	}
	if err := json.Unmarshal([]byte(content), &raw); err != nil {
		return nil, err
	}
	out := &Result{
		Phonetic:   firstNonEmpty(string(raw.Phonetic), string(raw.IPA), string(raw.Pronunciation)),
		Definition: firstNonEmpty(string(raw.Definition), string(raw.Definitions), string(raw.Meaning), string(raw.Meanings)),
		Example:    firstNonEmpty(string(raw.Example), string(raw.Examples)),
		Notes:      firstNonEmpty(string(raw.Notes), string(raw.Note)),
		Source:     "llm",
	}
	if out.Phonetic == "" && out.Definition == "" && out.Notes == "" {
		return nil, fmt.Errorf("the model response was missing vocabulary fields")
	}
	return out, nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if s := strings.TrimSpace(v); s != "" {
			return s
		}
	}
	return ""
}

type flexText string

func (f *flexText) UnmarshalJSON(b []byte) error {
	text, err := coerceFlexText(bytes.TrimSpace(b))
	if err != nil {
		return err
	}
	*f = flexText(text)
	return nil
}

func coerceFlexText(b []byte) (string, error) {
	b = bytes.TrimSpace(b)
	if len(b) == 0 || string(b) == "null" {
		return "", nil
	}
	switch b[0] {
	case '"':
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return "", err
		}
		return strings.TrimSpace(s), nil
	case '[':
		var raws []json.RawMessage
		if err := json.Unmarshal(b, &raws); err != nil {
			return "", err
		}
		defLike, exLike := 0, 0
		for _, raw := range raws {
			raw = bytes.TrimSpace(raw)
			if len(raw) > 0 && raw[0] == '{' {
				if looksLikeDefinitionObject(raw) {
					defLike++
				} else if looksLikeExampleObject(raw) {
					exLike++
				}
			}
		}
		if defLike > 0 && defLike >= exLike {
			return formatDefinitionArray(raws)
		}
		var parts []string
		for _, raw := range raws {
			s, err := coerceFlexText(raw)
			if err != nil {
				return "", err
			}
			if s != "" {
				parts = append(parts, s)
			}
		}
		return strings.Join(parts, "\n"), nil
	case '{':
		return formatFlexObject(b)
	default:
		return strings.TrimSpace(string(b)), nil
	}
}

func formatDefinitionArray(raws []json.RawMessage) (string, error) {
	marked := false
	for _, raw := range raws {
		if objectHasCommonFlag(raw) {
			marked = true
			break
		}
	}
	var parts []string
	for i, raw := range raws {
		s, err := coerceFlexText(raw)
		if err != nil {
			return "", err
		}
		if s == "" {
			continue
		}
		if !strings.Contains(s, "【常用】") && !strings.Contains(s, "【也作】") {
			isCommon := (!marked && i == 0) || objectHasCommonFlag(raw)
			if isCommon {
				s = "【常用】" + s
			} else {
				s = "【也作】" + s
			}
		}
		parts = append(parts, s)
	}
	return strings.Join(parts, "\n"), nil
}

func extractJSONCandidates(content string) []string {
	content = stripThinkTags(content)
	content = strings.TrimSpace(content)
	var out []string
	seen := map[string]bool{}
	add := func(s string) {
		s = strings.TrimSpace(s)
		if s == "" || seen[s] {
			return
		}
		seen[s] = true
		out = append(out, s)
	}
	add(content)
	if start := strings.Index(content, "{"); start >= 0 {
		if end := strings.LastIndex(content, "}"); end > start {
			add(content[start : end+1])
		}
		for i := 0; i < len(content); i++ {
			if content[i] != '{' {
				continue
			}
			if obj, ok := balancedJSONObject(content[i:]); ok {
				add(obj)
			}
		}
	}
	return out
}

func stripThinkTags(s string) string {
	pairs := [][2]string{
		{"<think>", "</think>"},
		{"<thinking>", "</thinking>"},
	}
	for _, pair := range pairs {
		for {
			lower := strings.ToLower(s)
			start := strings.Index(lower, pair[0])
			if start < 0 {
				break
			}
			rest := lower[start+len(pair[0]):]
			endRel := strings.Index(rest, pair[1])
			if endRel < 0 {
				s = s[:start] + s[start+len(pair[0]):]
				continue
			}
			end := start + len(pair[0]) + endRel + len(pair[1])
			s = s[:start] + "\n" + s[end:]
		}
	}
	return strings.TrimSpace(s)
}

func balancedJSONObject(s string) (string, bool) {
	if s == "" || s[0] != '{' {
		return "", false
	}
	depth := 0
	inString := false
	escaped := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if escaped {
			escaped = false
			continue
		}
		if inString {
			if c == '\\' {
				escaped = true
				continue
			}
			if c == '"' {
				inString = false
			}
			continue
		}
		switch c {
		case '"':
			inString = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return s[:i+1], true
			}
		}
	}
	return "", false
}

func repairLLMJSON(s string) string {
	return removeTrailingCommas(escapeRawControlsInStrings(s))
}

func escapeRawControlsInStrings(s string) string {
	var b strings.Builder
	b.Grow(len(s) + 16)
	inString := false
	escaped := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if escaped {
			b.WriteByte(c)
			escaped = false
			continue
		}
		if inString && c == '\\' {
			b.WriteByte(c)
			escaped = true
			continue
		}
		if c == '"' {
			inString = !inString
			b.WriteByte(c)
			continue
		}
		if inString {
			switch c {
			case '\n':
				b.WriteString(`\n`)
			case '\r':
				b.WriteString(`\r`)
			case '\t':
				b.WriteString(`\t`)
			default:
				if c < 0x20 {
					fmt.Fprintf(&b, `\u%04x`, c)
				} else {
					b.WriteByte(c)
				}
			}
			continue
		}
		b.WriteByte(c)
	}
	return b.String()
}

func removeTrailingCommas(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	inString := false
	escaped := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if escaped {
			b.WriteByte(c)
			escaped = false
			continue
		}
		if inString {
			if c == '\\' {
				escaped = true
			} else if c == '"' {
				inString = false
			}
			b.WriteByte(c)
			continue
		}
		if c == '"' {
			inString = true
			b.WriteByte(c)
			continue
		}
		if c == ',' {
			j := i + 1
			for j < len(s) && (s[j] == ' ' || s[j] == '\n' || s[j] == '\r' || s[j] == '\t') {
				j++
			}
			if j < len(s) && (s[j] == '}' || s[j] == ']') {
				continue
			}
		}
		b.WriteByte(c)
	}
	return b.String()
}

func looksLikeDefinitionObject(raw json.RawMessage) bool {
	obj, ok := asObject(raw)
	if !ok {
		return false
	}
	return hasAnyKey(obj, "definition", "gloss", "meaning", "sense", "pos", "partOfSpeech", "part_of_speech")
}

func looksLikeExampleObject(raw json.RawMessage) bool {
	obj, ok := asObject(raw)
	if !ok {
		return false
	}
	if hasAnyKey(obj, "example", "sentence") {
		return true
	}
	return hasAnyKey(obj, "en", "english") && hasAnyKey(obj, "zh", "chinese", "translation", "cn", "中")
}

func asObject(raw json.RawMessage) (map[string]json.RawMessage, bool) {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil || obj == nil {
		return nil, false
	}
	return obj, true
}

func hasAnyKey(obj map[string]json.RawMessage, keys ...string) bool {
	for _, want := range keys {
		for got := range obj {
			if strings.EqualFold(got, want) {
				return true
			}
		}
	}
	return false
}

func objectValue(obj map[string]json.RawMessage, keys ...string) string {
	for _, want := range keys {
		for got, raw := range obj {
			if !strings.EqualFold(got, want) {
				continue
			}
			s, err := coerceFlexText(raw)
			if err == nil && strings.TrimSpace(s) != "" {
				return strings.TrimSpace(s)
			}
		}
	}
	return ""
}

func objectHasCommonFlag(raw json.RawMessage) bool {
	obj, ok := asObject(raw)
	if !ok {
		return false
	}
	s := objectValue(obj, "common", "primary", "frequent", "is_common", "tag", "level")
	if s == "" {
		return false
	}
	lower := strings.ToLower(s)
	return lower == "true" || lower == "1" || lower == "yes" || lower == "common" || lower == "primary" || strings.Contains(s, "常用")
}

func formatFlexObject(b []byte) (string, error) {
	obj, ok := asObject(b)
	if !ok {
		return "", fmt.Errorf("invalid object")
	}
	if notes := formatNotesObject(obj); notes != "" {
		return notes, nil
	}
	pos := objectValue(obj, "pos", "partOfSpeech", "part_of_speech")
	gloss := objectValue(obj, "definition", "gloss", "meaning", "sense")
	zhGloss := objectValue(obj, "zh", "chinese", "translation", "cn", "中文", "中")
	if looksLikeDefinitionObject(b) {
		if gloss == "" {
			gloss = objectValue(obj, "en", "english")
		}
		var line strings.Builder
		if objectHasCommonFlag(b) || strings.Contains(objectValue(obj, "tag", "level"), "常用") {
			line.WriteString("【常用】")
		} else if strings.Contains(objectValue(obj, "tag", "level"), "也作") {
			line.WriteString("【也作】")
		}
		if pos != "" {
			line.WriteString("[" + pos + "] ")
		}
		line.WriteString(gloss)
		if zhGloss != "" && zhGloss != gloss {
			if gloss != "" {
				line.WriteByte('\n')
			}
			line.WriteString(zhGloss)
		}
		return strings.TrimSpace(line.String()), nil
	}
	en := objectValue(obj, "en", "english", "example", "sentence")
	zh := objectValue(obj, "zh", "chinese", "translation", "cn", "中")
	if en != "" || zh != "" {
		var bld strings.Builder
		if en != "" {
			if !strings.HasPrefix(en, "EN:") {
				bld.WriteString("EN: ")
			}
			bld.WriteString(en)
		}
		if zh != "" {
			if bld.Len() > 0 {
				bld.WriteByte('\n')
			}
			if !strings.HasPrefix(zh, "中:") {
				bld.WriteString("中: ")
			}
			bld.WriteString(zh)
		}
		return bld.String(), nil
	}
	keys := make([]string, 0, len(obj))
	for k := range obj {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var parts []string
	for _, k := range keys {
		s, err := coerceFlexText(obj[k])
		if err != nil || strings.TrimSpace(s) == "" {
			continue
		}
		parts = append(parts, strings.TrimSpace(s))
	}
	return strings.Join(parts, "\n"), nil
}

func formatNotesObject(obj map[string]json.RawMessage) string {
	sections := []struct {
		header string
		keys   []string
	}{
		{"【用法】", []string{"usage", "用法", "use"}},
		{"【搭配】", []string{"collocation", "collocations", "搭配", "phrases"}},
		{"【易混】", []string{"confusable", "confusion", "易混", "synonym", "synonyms"}},
		{"【记忆】", []string{"memory", "mnemonic", "记忆", "tip"}},
	}
	var out []string
	used := map[string]bool{}
	for _, sec := range sections {
		for _, key := range sec.keys {
			for got, raw := range obj {
				if !strings.EqualFold(got, key) || used[strings.ToLower(got)] {
					continue
				}
				used[strings.ToLower(got)] = true
				body, err := coerceFlexText(raw)
				if err != nil || strings.TrimSpace(body) == "" {
					continue
				}
				out = append(out, formatNotesSection(sec.header, body))
			}
		}
	}
	return strings.Join(out, "\n")
}

func formatNotesSection(header, body string) string {
	body = strings.TrimSpace(body)
	if body == "" {
		return ""
	}
	var b strings.Builder
	b.WriteString(header)
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "【") {
			b.WriteByte('\n')
			b.WriteString(line)
			continue
		}
		line = strings.TrimLeft(line, "·•-—* ")
		b.WriteString("\n· ")
		b.WriteString(line)
	}
	return b.String()
}
