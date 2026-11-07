// Package vocablookup fills vocabulary cards from a dictionary or an OpenAI-compatible model.
package vocablookup

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const DefaultPrompt = `You help a Chinese graduate student quickly understand and remember an English word or phrase from academic papers.

Reply with JSON only, no markdown fences, using these keys:
- phonetic: American IPA, like /ɪˈnɔːɡjəreɪt/
- definition: senses ranked by how often the learner will meet them when reading papers, at most 4. Format exactly:
【常用】[pos] short English gloss
Chinese gloss
【也作】[pos] short English gloss
Chinese gloss
Use 【常用】 for the 1-2 senses they should learn first. Use 【也作】 for other useful senses. Each English gloss is one short clause.
- example: two sentences for the most common sense(s), academic when it fits. Format exactly:
EN: English sentence.
中: Natural Chinese translation.
EN: English sentence.
中: Natural Chinese translation.
- notes: Chinese only, easy to scan, to help them master the word quickly. Use these headers and "· " bullets; omit an empty section:
【用法】
· when and why writers choose this word
【搭配】
· key collocations, keep the English phrase and add a brief Chinese note
【易混】
· contrast with one close synonym if useful
【记忆】
· one short memory hook

Be accurate and concise. Do not invent unrelated meanings.`

// LLMTimeout is how long a model lookup may take, including slower compatible APIs.
const LLMTimeout = 90 * time.Second

var httpClient = &http.Client{Timeout: LLMTimeout}

// Result is the filled-in card content from a lookup.
type Result struct {
	Phonetic   string `json:"phonetic"`
	Definition string `json:"definition"`
	Example    string `json:"example"`
	Notes      string `json:"notes"`
	AudioURL   string `json:"audioUrl,omitempty"`
	Source     string `json:"source"`
}

// LLMConfig is an OpenAI-compatible chat completions endpoint.
type LLMConfig struct {
	BaseURL string
	APIKey  string
	Model   string
	Prompt  string
}

func NormalizeChatCompletionsURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return "", fmt.Errorf("invalid API base URL")
	}
	path := strings.TrimRight(u.Path, "/")
	switch {
	case strings.HasSuffix(path, "/chat/completions"):
		// already a full endpoint
	case path == "":
		path = "/v1/chat/completions"
	default:
		path += "/chat/completions"
	}
	u.Path = path
	u.Fragment = ""
	return u.String(), nil
}

func ParseLLMVocabJSON(content string) (*Result, error) {
	content = strings.TrimSpace(content)
	if i := strings.Index(content, "{"); i >= 0 {
		if j := strings.LastIndex(content, "}"); j > i {
			content = content[i : j+1]
		}
	}
	var raw struct {
		Phonetic   string `json:"phonetic"`
		Definition string `json:"definition"`
		Example    string `json:"example"`
		Notes      string `json:"notes"`
	}
	if err := json.Unmarshal([]byte(content), &raw); err != nil {
		return nil, fmt.Errorf("the model response was not valid JSON")
	}
	out := &Result{
		Phonetic:   strings.TrimSpace(raw.Phonetic),
		Definition: strings.TrimSpace(raw.Definition),
		Example:    strings.TrimSpace(raw.Example),
		Notes:      strings.TrimSpace(raw.Notes),
		Source:     "llm",
	}
	if out.Phonetic == "" && out.Definition == "" && out.Notes == "" {
		return nil, fmt.Errorf("the model response was missing vocabulary fields")
	}
	return out, nil
}

type flexContent struct{ Text string }

func (c *flexContent) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if len(b) > 0 && b[0] == '"' {
		return json.Unmarshal(b, &c.Text)
	}
	var parts []struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(b, &parts); err != nil {
		return err
	}
	var bld strings.Builder
	for _, part := range parts {
		bld.WriteString(part.Text)
	}
	c.Text = bld.String()
	return nil
}

func redact(s, secret string) string {
	if secret != "" && strings.Contains(s, secret) {
		return strings.ReplaceAll(s, secret, "****")
	}
	return s
}

func shouldDisableThinking(endpoint string) bool {
	host := strings.ToLower(endpoint)
	return strings.Contains(host, "aliyuncs.com") || strings.Contains(host, "dashscope")
}

func timeoutOrRequestError(err error, secret string) error {
	msg := redact(err.Error(), secret)
	lower := strings.ToLower(msg)
	if errors.Is(err, context.DeadlineExceeded) || strings.Contains(lower, "deadline exceeded") || strings.Contains(lower, "timeout") {
		return fmt.Errorf("the model request timed out after %s; try again, or switch to a faster model", LLMTimeout)
	}
	return fmt.Errorf("the model request failed: %s", msg)
}

func LookupLLM(ctx context.Context, cfg LLMConfig, term string) (*Result, error) {
	endpoint, err := NormalizeChatCompletionsURL(cfg.BaseURL)
	if err != nil {
		return nil, err
	}
	prompt := strings.TrimSpace(cfg.Prompt)
	if prompt == "" {
		prompt = DefaultPrompt
	}
	payloadBody := map[string]any{
		"model":       cfg.Model,
		"temperature": 0.3,
		"max_tokens":  1400,
		"messages": []map[string]string{
			{"role": "system", "content": prompt},
			{"role": "user", "content": term},
		},
	}
	if shouldDisableThinking(endpoint) {
		payloadBody["enable_thinking"] = false
		payloadBody["chat_template_kwargs"] = map[string]any{"enable_thinking": false}
	}
	payload, err := json.Marshal(payloadBody)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, timeoutOrRequestError(err, cfg.APIKey)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("failed to read the model response")
	}
	var parsed struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
		Choices []struct {
			Message struct {
				Content flexContent `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, fmt.Errorf("the model lookup failed (%d)", resp.StatusCode)
		}
		return nil, fmt.Errorf("the model response was not valid JSON")
	}
	if parsed.Error.Message != "" {
		return nil, fmt.Errorf("the model lookup failed: %s", redact(parsed.Error.Message, cfg.APIKey))
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("the model lookup failed (%d)", resp.StatusCode)
	}
	if len(parsed.Choices) == 0 || strings.TrimSpace(parsed.Choices[0].Message.Content.Text) == "" {
		return nil, fmt.Errorf("the model returned an empty response")
	}
	return ParseLLMVocabJSON(parsed.Choices[0].Message.Content.Text)
}

func ParseDictionaryJSON(data []byte) (*Result, error) {
	var entries []struct {
		Phonetic  string `json:"phonetic"`
		Phonetics []struct {
			Text  string `json:"text"`
			Audio string `json:"audio"`
		} `json:"phonetics"`
		Meanings []struct {
			PartOfSpeech string `json:"partOfSpeech"`
			Definitions  []struct {
				Definition string `json:"definition"`
				Example    string `json:"example"`
			} `json:"definitions"`
		} `json:"meanings"`
	}
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, err
	}
	if len(entries) == 0 {
		return nil, nil
	}

	phonetic := ""
	audioUS, audioOther := "", ""
	var definitions []string
	var examples []string

	for _, entry := range entries {
		if phonetic == "" {
			phonetic = strings.TrimSpace(entry.Phonetic)
		}
		for _, item := range entry.Phonetics {
			if phonetic == "" {
				phonetic = strings.TrimSpace(item.Text)
			}
			audio := strings.TrimSpace(item.Audio)
			if audio == "" {
				continue
			}
			if strings.Contains(strings.ToLower(audio), "-us") || strings.Contains(strings.ToLower(audio), "us.mp3") {
				if audioUS == "" {
					audioUS = audio
				}
			} else if audioOther == "" {
				audioOther = audio
			}
		}
		for _, meaning := range entry.Meanings {
			pos := strings.TrimSpace(meaning.PartOfSpeech)
			for _, def := range meaning.Definitions {
				text := strings.TrimSpace(def.Definition)
				if text == "" {
					continue
				}
				if pos != "" {
					text = "[" + pos + "] " + text
				}
				if len(definitions) < 6 {
					definitions = append(definitions, text)
				}
				if ex := strings.TrimSpace(def.Example); ex != "" && len(examples) < 3 {
					examples = append(examples, ex)
				}
			}
		}
	}

	out := &Result{
		Phonetic:   phonetic,
		Definition: strings.Join(definitions, "\n"),
		Example:    strings.Join(examples, "\n"),
		AudioURL:   audioUS,
		Source:     "dictionary",
	}
	if out.AudioURL == "" {
		out.AudioURL = audioOther
	}
	if out.Phonetic == "" && out.Definition == "" {
		return nil, nil
	}
	return out, nil
}

func LookupDictionary(ctx context.Context, term string) (*Result, error) {
	u := "https://api.dictionaryapi.dev/api/v2/entries/en/" + url.PathEscape(term)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	client := &http.Client{Timeout: 12 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("the dictionary request failed")
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("failed to read the dictionary response")
	}
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("the dictionary lookup failed (%d)", resp.StatusCode)
	}
	return ParseDictionaryJSON(body)
}

func MergeAudio(primary, fallback *Result) *Result {
	if primary == nil {
		return fallback
	}
	if fallback == nil {
		return primary
	}
	if primary.Phonetic == "" {
		primary.Phonetic = fallback.Phonetic
	}
	if primary.AudioURL == "" {
		primary.AudioURL = fallback.AudioURL
	}
	return primary
}
