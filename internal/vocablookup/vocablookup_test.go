package vocablookup

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeChatCompletionsURL(t *testing.T) {
	cases := map[string]string{
		"https://api.deepseek.com":                                         "https://api.deepseek.com/v1/chat/completions",
		"https://api.deepseek.com/":                                        "https://api.deepseek.com/v1/chat/completions",
		"https://api.deepseek.com/v1":                                      "https://api.deepseek.com/v1/chat/completions",
		"https://api.deepseek.com/v1/":                                     "https://api.deepseek.com/v1/chat/completions",
		"https://api.openai.com/v1/chat/completions":                       "https://api.openai.com/v1/chat/completions",
		"https://api.openai.com/v1/chat/completions/":                      "https://api.openai.com/v1/chat/completions",
		"https://dashscope.aliyuncs.com/compatible-mode/v1":                "https://dashscope.aliyuncs.com/compatible-mode/v1/chat/completions",
		"https://llm-demo.cn-beijing.maas.aliyuncs.com/compatible-mode/v1": "https://llm-demo.cn-beijing.maas.aliyuncs.com/compatible-mode/v1/chat/completions",
		"http://127.0.0.1:11434":                                           "http://127.0.0.1:11434/v1/chat/completions",
	}
	for raw, want := range cases {
		got, err := NormalizeChatCompletionsURL(raw)
		require.NoError(t, err, raw)
		require.Equal(t, want, got, raw)
	}
	_, err := NormalizeChatCompletionsURL("ftp://example.com")
	require.Error(t, err)
	_, err = NormalizeChatCompletionsURL("not a url")
	require.Error(t, err)
}

func TestParseLLMVocabJSON(t *testing.T) {
	raw, err := ParseLLMVocabJSON("```json\n{\"phonetic\":\"/test/\",\"definition\":\"d\",\"example\":\"e\",\"notes\":\"n\"}\n```")
	require.NoError(t, err)
	require.Equal(t, "/test/", raw.Phonetic)
	require.Equal(t, "d", raw.Definition)
	require.Equal(t, "e", raw.Example)
	require.Equal(t, "n", raw.Notes)
	require.Equal(t, "llm", raw.Source)

	_, err = ParseLLMVocabJSON("sorry")
	require.Error(t, err)
}

func TestParseDictionaryJSON(t *testing.T) {
	fixture := `[
	  {
	    "phonetic": "/həˈloʊ/",
	    "phonetics": [
	      {"text": "/həˈləʊ/", "audio": "https://example.com/hello-uk.mp3"},
	      {"text": "/həˈloʊ/", "audio": "https://example.com/hello-us.mp3"}
	    ],
	    "meanings": [
	      {
	        "partOfSpeech": "exclamation",
	        "definitions": [
	          {"definition": "used as a greeting", "example": "hello there!"}
	        ]
	      },
	      {
	        "partOfSpeech": "noun",
	        "definitions": [
	          {"definition": "an utterance of “hello”", "example": "she was greeted by a cheery hello"}
	        ]
	      }
	    ]
	  }
	]`
	got, err := ParseDictionaryJSON([]byte(fixture))
	require.NoError(t, err)
	require.Equal(t, "/həˈloʊ/", got.Phonetic)
	require.Contains(t, got.Definition, "[exclamation] used as a greeting")
	require.Contains(t, got.Definition, "[noun] an utterance")
	require.Contains(t, got.Example, "hello there!")
	require.Equal(t, "https://example.com/hello-us.mp3", got.AudioURL)
	require.Equal(t, "dictionary", got.Source)
}

func TestLookupLLM(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/chat/completions", r.URL.Path)
		require.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))
		var body struct {
			Model    string `json:"model"`
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		require.Equal(t, "demo-model", body.Model)
		require.Equal(t, "hello", body.Messages[1].Content)
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{\"phonetic\":\"/həˈloʊ/\",\"definition\":\"used as a greeting\\n用作招呼语\",\"example\":\"Hello, everyone.\",\"notes\":\"口语问候\"}"}}]}`))
	}))
	defer srv.Close()

	got, err := LookupLLM(context.Background(), LLMConfig{
		BaseURL: srv.URL,
		APIKey:  "test-key",
		Model:   "demo-model",
	}, "hello")
	require.NoError(t, err)
	require.Equal(t, "/həˈloʊ/", got.Phonetic)
	require.Contains(t, got.Definition, "用作招呼语")
	require.Equal(t, "Hello, everyone.", got.Example)
	require.Equal(t, "口语问候", got.Notes)
	require.Equal(t, "llm", got.Source)
}

func TestLookupLLMErrorIsRedacted(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"message":"invalid key secret-key-value"}}`))
	}))
	defer srv.Close()

	_, err := LookupLLM(context.Background(), LLMConfig{
		BaseURL: srv.URL,
		APIKey:  "secret-key-value",
		Model:   "demo-model",
	}, "hello")
	require.Error(t, err)
	require.NotContains(t, err.Error(), "secret-key-value")
	require.Contains(t, err.Error(), "****")
}

func TestShouldDisableThinking(t *testing.T) {
	require.True(t, shouldDisableThinking("https://llm-demo.cn-beijing.maas.aliyuncs.com/compatible-mode/v1/chat/completions"))
	require.True(t, shouldDisableThinking("https://dashscope.aliyuncs.com/compatible-mode/v1/chat/completions"))
	require.False(t, shouldDisableThinking("https://api.deepseek.com/v1/chat/completions"))
	require.False(t, shouldDisableThinking("http://127.0.0.1:11434/v1/chat/completions"))
}

func TestTimeoutOrRequestError(t *testing.T) {
	err := timeoutOrRequestError(context.DeadlineExceeded, "secret-key")
	require.Error(t, err)
	require.Contains(t, err.Error(), "timed out")
	require.NotContains(t, err.Error(), "secret-key")

	err = timeoutOrRequestError(assertError("Post \"https://example.com\": context deadline exceeded"), "secret-key")
	require.Contains(t, err.Error(), "timed out")
}

type assertError string

func (e assertError) Error() string { return string(e) }

func TestParseLLMVocabJSONThinkTags(t *testing.T) {
	raw, err := ParseLLMVocabJSON("<think>\nI will output {\"phonetic\":\"/no/\"}\n</think>\n{\"phonetic\":\"/yes/\",\"definition\":\"d\",\"example\":\"e\",\"notes\":\"n\"}")
	require.NoError(t, err)
	require.Equal(t, "/yes/", raw.Phonetic)
}

func TestParseLLMVocabJSONRawNewlines(t *testing.T) {
	raw, err := ParseLLMVocabJSON("{\n  \"phonetic\": \"/test/\",\n  \"definition\": \"【常用】[verb] begin\n开始\",\n  \"example\": \"EN: Hello.\n中: 你好。\",\n  \"notes\": \"【用法】\n· formal\"\n}")
	require.NoError(t, err)
	require.Equal(t, "/test/", raw.Phonetic)
	require.Contains(t, raw.Definition, "开始")
	require.Contains(t, raw.Example, "你好")
	require.Contains(t, raw.Notes, "formal")
}

func TestParseLLMVocabJSONStructured(t *testing.T) {
	raw, err := ParseLLMVocabJSON(`{
		"phonetic": "/test/",
		"definition": [
			{"pos": "verb", "en": "begin", "zh": "开始", "common": true},
			{"pos": "verb", "en": "initiate", "zh": "开创"}
		],
		"example": [
			{"en": "They inaugurated the lab.", "zh": "他们为实验室举行了落成典礼。"}
		],
		"notes": {"usage": "偏正式", "memory": "in + augur"}
	}`)
	require.NoError(t, err)
	require.Contains(t, raw.Definition, "【常用】")
	require.Contains(t, raw.Definition, "开始")
	require.Contains(t, raw.Definition, "【也作】")
	require.Contains(t, raw.Example, "EN: They inaugurated the lab.")
	require.Contains(t, raw.Example, "中: 他们为实验室举行了落成典礼。")
	require.Contains(t, raw.Notes, "【用法】")
	require.Contains(t, raw.Notes, "偏正式")
	require.Contains(t, raw.Notes, "【记忆】")
}

func TestParseLLMVocabJSONTrailingComma(t *testing.T) {
	raw, err := ParseLLMVocabJSON(`{"phonetic":"/t/","definition":"d","example":"e","notes":"n",}`)
	require.NoError(t, err)
	require.Equal(t, "/t/", raw.Phonetic)
}

func TestParseLLMVocabJSONAlternateKeys(t *testing.T) {
	raw, err := ParseLLMVocabJSON(`{"ipa":"/t/","definitions":"d","examples":"e","note":"n"}`)
	require.NoError(t, err)
	require.Equal(t, "/t/", raw.Phonetic)
	require.Equal(t, "d", raw.Definition)
	require.Equal(t, "e", raw.Example)
	require.Equal(t, "n", raw.Notes)
}

func TestLookupLLMNestedObjectContent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":{"phonetic":"/x/","definition":"d","example":"e","notes":"n"}}}]}`))
	}))
	defer srv.Close()
	got, err := LookupLLM(context.Background(), LLMConfig{BaseURL: srv.URL, APIKey: "k", Model: "m"}, "hello")
	require.NoError(t, err)
	require.Equal(t, "/x/", got.Phonetic)
	require.Equal(t, "d", got.Definition)
}

func TestLookupLLMReasoningContent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"","reasoning_content":"{\"phonetic\":\"/x/\",\"definition\":\"d\",\"example\":\"e\",\"notes\":\"n\"}"}}]}`))
	}))
	defer srv.Close()
	got, err := LookupLLM(context.Background(), LLMConfig{BaseURL: srv.URL, APIKey: "k", Model: "m"}, "hello")
	require.NoError(t, err)
	require.Equal(t, "/x/", got.Phonetic)
}

func TestLookupLLMTruncatedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"choices":[{"finish_reason":"length","message":{"content":"{\"phonetic\":\"/x/\",\"definition\":\"partial"}}]}`))
	}))
	defer srv.Close()
	_, err := LookupLLM(context.Background(), LLMConfig{BaseURL: srv.URL, APIKey: "k", Model: "m"}, "hello")
	require.Error(t, err)
	require.Contains(t, err.Error(), "cut off")
}

func TestEnsureJSONInstruction(t *testing.T) {
	require.Equal(t, "Reply with JSON only", ensureJSONInstruction("Reply with JSON only"))
	got := ensureJSONInstruction("Explain the word.")
	require.Contains(t, strings.ToLower(got), "json")
	require.Contains(t, got, "Explain the word.")
}
