package vocablookup

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeChatCompletionsURL(t *testing.T) {
	cases := map[string]string{
		"https://api.deepseek.com":                          "https://api.deepseek.com/v1/chat/completions",
		"https://api.deepseek.com/":                         "https://api.deepseek.com/v1/chat/completions",
		"https://api.deepseek.com/v1":                       "https://api.deepseek.com/v1/chat/completions",
		"https://api.deepseek.com/v1/":                      "https://api.deepseek.com/v1/chat/completions",
		"https://api.openai.com/v1/chat/completions":        "https://api.openai.com/v1/chat/completions",
		"https://api.openai.com/v1/chat/completions/":       "https://api.openai.com/v1/chat/completions",
		"https://dashscope.aliyuncs.com/compatible-mode/v1": "https://dashscope.aliyuncs.com/compatible-mode/v1/chat/completions",
		"http://127.0.0.1:11434":                            "http://127.0.0.1:11434/v1/chat/completions",
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
