// Package vocablookup fills vocabulary cards from a dictionary or an OpenAI-compatible model.
package vocablookup

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const DefaultPrompt = `You help a Chinese graduate student build vocabulary cards while reading English academic papers.
The user message is an English word or phrase (it may be a technical term).
Reply with JSON only, no markdown, using these keys:
- phonetic: IPA transcription, American English preferred, like /.../
- definition: a concise English definition, then a Chinese translation, separated by a newline
- example: one or two natural example sentences (academic register when it fits), one sentence per line
- notes: short Chinese notes on collocations, usage, register, and confusions that matter when reading papers
Keep the content practical and accurate. Do not invent an unrelated meaning.`

var httpClient = &http.Client{Timeout: 45 * time.Second}

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

func LookupLLM(ctx context.Context, cfg LLMConfig, term string) (*Result, error) {
	endpoint, err := NormalizeChatCompletionsURL(cfg.BaseURL)
	if err != nil {
		return nil, err
	}
	prompt := strings.TrimSpace(cfg.Prompt)
	if prompt == "" {
		prompt = DefaultPrompt
	}
	payload, err := json.Marshal(map[string]any{
		"model":       cfg.Model,
		"temperature": 0.3,
		"messages": []map[string]string{
			{"role": "system", "content": prompt},
			{"role": "user", "content": term},
		},
	})
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
		return nil, fmt.Errorf("the model request failed: %s", redact(err.Error(), cfg.APIKey))
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
