package api

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/usememos/memos/internal/random"
	"github.com/usememos/memos/internal/vocabfsrs"
	"github.com/usememos/memos/internal/vocablookup"
	"github.com/usememos/memos/server/auth"
	"github.com/usememos/memos/store"
)

const vocabMarker = "mbe-vocab-v4"

type vocabCardJSON struct {
	UID          string                   `json:"uid"`
	Term         string                   `json:"term"`
	Phonetic     string                   `json:"phonetic"`
	Definition   string                   `json:"definition"`
	Example      string                   `json:"example"`
	Notes        string                   `json:"notes"`
	Tags         []string                 `json:"tags"`
	State        int32                    `json:"state"`
	DueTs        int64                    `json:"dueTs"`
	Reps         int32                    `json:"reps"`
	Lapses       int32                    `json:"lapses"`
	LastReviewTs int64                    `json:"lastReviewTs"`
	CreatedTs    int64                    `json:"createdTs"`
	UpdatedTs    int64                    `json:"updatedTs"`
	Previews     map[string]vocabPrevJSON `json:"previews,omitempty"`
	Logs         []vocabLogJSON           `json:"logs,omitempty"`
}

type vocabPrevJSON struct {
	DueTs         int64 `json:"dueTs"`
	ScheduledDays int64 `json:"scheduledDays"`
}

type vocabLogJSON struct {
	Rating   int32 `json:"rating"`
	State    int32 `json:"state"`
	DueTs    int64 `json:"dueTs"`
	ReviewTs int64 `json:"reviewTs"`
}

type vocabCardPayload struct {
	Term       string   `json:"term"`
	Phonetic   string   `json:"phonetic"`
	Definition string   `json:"definition"`
	Example    string   `json:"example"`
	Notes      string   `json:"notes"`
	Tags       []string `json:"tags"`
}

func decodeTags(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return []string{}
	}
	var tags []string
	if err := json.Unmarshal([]byte(raw), &tags); err == nil {
		return tags
	}
	return []string{}
}

func encodeTags(tags []string) string {
	if tags == nil {
		tags = []string{}
	}
	cleaned := make([]string, 0, len(tags))
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if tag != "" {
			cleaned = append(cleaned, tag)
		}
	}
	b, err := json.Marshal(cleaned)
	if err != nil {
		return "[]"
	}
	return string(b)
}

func toVocabJSON(c *store.VocabCard, withPreview bool) vocabCardJSON {
	out := vocabCardJSON{
		UID:          c.UID,
		Term:         c.Term,
		Phonetic:     c.Phonetic,
		Definition:   c.Definition,
		Example:      c.Example,
		Notes:        c.Notes,
		Tags:         decodeTags(c.Tags),
		State:        c.State,
		DueTs:        c.DueTs,
		Reps:         c.Reps,
		Lapses:       c.Lapses,
		LastReviewTs: c.LastReviewTs,
		CreatedTs:    c.CreatedTs,
		UpdatedTs:    c.UpdatedTs,
	}
	if withPreview {
		previews := vocabfsrs.PreviewRatings(c, time.Now())
		out.Previews = map[string]vocabPrevJSON{}
		for rating, preview := range previews {
			out.Previews[strconv.Itoa(rating)] = vocabPrevJSON{DueTs: preview.DueTs, ScheduledDays: preview.ScheduledDays}
		}
	}
	return out
}

func (s *APIService) vocabCurrentUser(r *http.Request) *store.User {
	authn := auth.NewAuthenticator(s.Store, s.Secret)
	u, err := authn.AuthenticateToUser(r.Context(), r.Header.Get("Authorization"), r.Header.Get("Cookie"))
	if err != nil || u == nil || u.RowStatus != store.Normal {
		return nil
	}
	return u
}

func (s *APIService) loadOwnedVocab(w http.ResponseWriter, r *http.Request, user *store.User) *store.VocabCard {
	uid := r.PathValue("uid")
	if uid == "" {
		jsonError(w, http.StatusBadRequest, "missing uid")
		return nil
	}
	card, err := s.Store.GetVocabCard(r.Context(), &store.FindVocabCard{UID: &uid, CreatorID: &user.ID})
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to load card")
		return nil
	}
	if card == nil {
		jsonError(w, http.StatusNotFound, "card not found")
		return nil
	}
	return card
}

func normalizeTerm(term string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(term)), " ")
}

// RegisterVocabRoutes mounts the vocabulary notebook endpoints.
func (s *APIService) RegisterVocabRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/vocab/stats", s.handleVocabStats)
	mux.HandleFunc("GET /api/v1/vocab/settings", s.handleVocabGetSettings)
	mux.HandleFunc("PUT /api/v1/vocab/settings", s.handleVocabPutSettings)
	mux.HandleFunc("POST /api/v1/vocab/lookup", s.handleVocabLookup)
	mux.HandleFunc("GET /api/v1/vocab/export", s.handleVocabExport)
	mux.HandleFunc("POST /api/v1/vocab/import", s.handleVocabImport)
	mux.HandleFunc("GET /api/v1/vocab/session", s.handleVocabSession)
	mux.HandleFunc("GET /api/v1/vocab/cards", s.handleVocabList)
	mux.HandleFunc("POST /api/v1/vocab/cards", s.handleVocabCreate)
	mux.HandleFunc("GET /api/v1/vocab/cards/{uid}", s.handleVocabGet)
	mux.HandleFunc("PATCH /api/v1/vocab/cards/{uid}", s.handleVocabUpdate)
	mux.HandleFunc("DELETE /api/v1/vocab/cards/{uid}", s.handleVocabDelete)
	mux.HandleFunc("POST /api/v1/vocab/cards/{uid}/review", s.handleVocabReview)
}

func (s *APIService) handleVocabStats(w http.ResponseWriter, r *http.Request) {
	user := s.vocabCurrentUser(r)
	if user == nil {
		jsonError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	st, err := s.Store.GetVocabStats(r.Context(), user.ID, time.Now().Unix())
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to load stats")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"marker":   vocabMarker,
		"total":    st.Total,
		"new":      st.New,
		"learning": st.Learning,
		"review":   st.Review,
		"due":      st.Due,
	})
}

func (s *APIService) handleVocabList(w http.ResponseWriter, r *http.Request) {
	user := s.vocabCurrentUser(r)
	if user == nil {
		jsonError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	q := r.URL.Query()
	find := &store.FindVocabCard{CreatorID: &user.ID, Filter: q.Get("filter"), Sort: q.Get("sort")}
	if search := strings.TrimSpace(q.Get("q")); search != "" {
		find.Query = &search
	}
	limit, _ := strconv.Atoi(q.Get("limit"))
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	find.Limit = limit
	find.Offset, _ = strconv.Atoi(q.Get("offset"))
	if find.Sort == "" {
		find.Sort = "added"
	}
	cards, err := s.Store.ListVocabCards(r.Context(), find)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to list cards")
		return
	}
	out := make([]vocabCardJSON, 0, len(cards))
	for _, card := range cards {
		out = append(out, toVocabJSON(card, false))
	}
	writeJSON(w, http.StatusOK, map[string]any{"cards": out})
}

func (s *APIService) handleVocabSession(w http.ResponseWriter, r *http.Request) {
	user := s.vocabCurrentUser(r)
	if user == nil {
		jsonError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	due, err := s.Store.ListVocabCards(r.Context(), &store.FindVocabCard{CreatorID: &user.ID, Filter: "due_review", Sort: "due", Limit: 200})
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to load session")
		return
	}
	news, err := s.Store.ListVocabCards(r.Context(), &store.FindVocabCard{CreatorID: &user.ID, Filter: "new", Sort: "added", Limit: 20})
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to load session")
		return
	}
	cards := append(due, news...)
	out := make([]vocabCardJSON, 0, len(cards))
	for _, card := range cards {
		out = append(out, toVocabJSON(card, true))
	}
	writeJSON(w, http.StatusOK, map[string]any{"cards": out, "marker": vocabMarker})
}

func (s *APIService) handleVocabGet(w http.ResponseWriter, r *http.Request) {
	user := s.vocabCurrentUser(r)
	if user == nil {
		jsonError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	card := s.loadOwnedVocab(w, r, user)
	if card == nil {
		return
	}
	out := toVocabJSON(card, true)
	logs, err := s.Store.ListVocabReviewLogs(r.Context(), card.ID, 20)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to load history")
		return
	}
	out.Logs = make([]vocabLogJSON, 0, len(logs))
	for _, log := range logs {
		out.Logs = append(out.Logs, vocabLogJSON{Rating: log.Rating, State: log.State, DueTs: log.DueTs, ReviewTs: log.ReviewTs})
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *APIService) handleVocabCreate(w http.ResponseWriter, r *http.Request) {
	user := s.vocabCurrentUser(r)
	if user == nil {
		jsonError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var payload vocabCardPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid json")
		return
	}
	term := normalizeTerm(payload.Term)
	definition := strings.TrimSpace(payload.Definition)
	if term == "" || definition == "" {
		jsonError(w, http.StatusBadRequest, "term and definition are required")
		return
	}
	existing, err := s.Store.GetVocabCard(r.Context(), &store.FindVocabCard{CreatorID: &user.ID, TermEqualFold: &term})
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to check existing term")
		return
	}
	if existing != nil {
		writeJSON(w, http.StatusConflict, map[string]any{"message": "term already exists", "uid": existing.UID})
		return
	}
	now := time.Now().Unix()
	card, err := s.Store.CreateVocabCard(r.Context(), &store.VocabCard{
		UID:        random.UUID(),
		CreatorID:  user.ID,
		Term:       term,
		Phonetic:   strings.TrimSpace(payload.Phonetic),
		Definition: definition,
		Example:    strings.TrimSpace(payload.Example),
		Notes:      strings.TrimSpace(payload.Notes),
		Tags:       encodeTags(payload.Tags),
		DueTs:      now,
	})
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to create card")
		return
	}
	writeJSON(w, http.StatusOK, toVocabJSON(card, true))
}

func (s *APIService) handleVocabUpdate(w http.ResponseWriter, r *http.Request) {
	user := s.vocabCurrentUser(r)
	if user == nil {
		jsonError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	card := s.loadOwnedVocab(w, r, user)
	if card == nil {
		return
	}
	var payload vocabCardPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid json")
		return
	}
	term := normalizeTerm(payload.Term)
	definition := strings.TrimSpace(payload.Definition)
	if term == "" || definition == "" {
		jsonError(w, http.StatusBadRequest, "term and definition are required")
		return
	}
	existing, err := s.Store.GetVocabCard(r.Context(), &store.FindVocabCard{CreatorID: &user.ID, TermEqualFold: &term})
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to check existing term")
		return
	}
	if existing != nil && existing.ID != card.ID {
		writeJSON(w, http.StatusConflict, map[string]any{"message": "term already exists", "uid": existing.UID})
		return
	}
	card.Term = term
	card.Phonetic = strings.TrimSpace(payload.Phonetic)
	card.Definition = definition
	card.Example = strings.TrimSpace(payload.Example)
	card.Notes = strings.TrimSpace(payload.Notes)
	card.Tags = encodeTags(payload.Tags)
	if err := s.Store.UpdateVocabCardContent(r.Context(), card); err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to update card")
		return
	}
	writeJSON(w, http.StatusOK, toVocabJSON(card, true))
}

func (s *APIService) handleVocabDelete(w http.ResponseWriter, r *http.Request) {
	user := s.vocabCurrentUser(r)
	if user == nil {
		jsonError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	uid := r.PathValue("uid")
	ok, err := s.Store.DeleteVocabCard(r.Context(), user.ID, uid)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to delete card")
		return
	}
	if !ok {
		jsonError(w, http.StatusNotFound, "card not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{})
}

func (s *APIService) handleVocabReview(w http.ResponseWriter, r *http.Request) {
	user := s.vocabCurrentUser(r)
	if user == nil {
		jsonError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	card := s.loadOwnedVocab(w, r, user)
	if card == nil {
		return
	}
	var payload struct {
		Rating int `json:"rating"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid json")
		return
	}
	log, err := vocabfsrs.ApplyRating(card, payload.Rating, time.Now())
	if err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.Store.UpdateVocabCardScheduler(r.Context(), card); err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to save review")
		return
	}
	if _, err := s.Store.CreateVocabReviewLog(r.Context(), log); err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to save review log")
		return
	}
	writeJSON(w, http.StatusOK, toVocabJSON(card, true))
}

func (s *APIService) handleVocabExport(w http.ResponseWriter, r *http.Request) {
	user := s.vocabCurrentUser(r)
	if user == nil {
		jsonError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	cards, err := s.Store.ListVocabCards(r.Context(), &store.FindVocabCard{CreatorID: &user.ID, Sort: "term", Limit: 10000})
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to export")
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="vocab.csv"`)
	w.WriteHeader(http.StatusOK)
	cw := csv.NewWriter(w)
	_ = cw.Write([]string{"term", "phonetic", "definition", "example", "notes", "tags", "state", "dueTs"})
	for _, card := range cards {
		_ = cw.Write([]string{
			card.Term, card.Phonetic, card.Definition, card.Example, card.Notes,
			strings.Join(decodeTags(card.Tags), "|"),
			strconv.Itoa(int(card.State)),
			strconv.FormatInt(card.DueTs, 10),
		})
	}
	cw.Flush()
}

func (s *APIService) handleVocabImport(w http.ResponseWriter, r *http.Request) {
	user := s.vocabCurrentUser(r)
	if user == nil {
		jsonError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 4<<20))
	if err != nil {
		jsonError(w, http.StatusBadRequest, "failed to read csv")
		return
	}
	cr := csv.NewReader(strings.NewReader(string(body)))
	cr.FieldsPerRecord = -1
	cr.LazyQuotes = true
	rows, err := cr.ReadAll()
	if err != nil || len(rows) == 0 {
		jsonError(w, http.StatusBadRequest, "invalid csv")
		return
	}
	start := 0
	if len(rows[0]) > 0 && strings.EqualFold(strings.TrimPrefix(rows[0][0], "\ufeff"), "term") {
		start = 1
	}
	imported := 0
	for _, row := range rows[start:] {
		if len(row) < 3 {
			continue
		}
		term := normalizeTerm(row[0])
		definition := strings.TrimSpace(row[2])
		if term == "" || definition == "" {
			continue
		}
		payload := vocabCardPayload{
			Term:       term,
			Phonetic:   cell(row, 1),
			Definition: definition,
			Example:    cell(row, 3),
			Notes:      cell(row, 4),
		}
		if rawTags := cell(row, 5); rawTags != "" {
			payload.Tags = splitTags(rawTags)
		}
		existing, err := s.Store.GetVocabCard(r.Context(), &store.FindVocabCard{CreatorID: &user.ID, TermEqualFold: &term})
		if err != nil {
			jsonError(w, http.StatusInternalServerError, "failed to import")
			return
		}
		if existing != nil {
			existing.Term = term
			existing.Phonetic = payload.Phonetic
			existing.Definition = payload.Definition
			existing.Example = payload.Example
			existing.Notes = payload.Notes
			existing.Tags = encodeTags(payload.Tags)
			if err := s.Store.UpdateVocabCardContent(r.Context(), existing); err != nil {
				jsonError(w, http.StatusInternalServerError, "failed to import")
				return
			}
		} else {
			if _, err := s.Store.CreateVocabCard(r.Context(), &store.VocabCard{
				UID:        random.UUID(),
				CreatorID:  user.ID,
				Term:       term,
				Phonetic:   payload.Phonetic,
				Definition: payload.Definition,
				Example:    payload.Example,
				Notes:      payload.Notes,
				Tags:       encodeTags(payload.Tags),
				DueTs:      time.Now().Unix(),
			}); err != nil {
				jsonError(w, http.StatusInternalServerError, "failed to import")
				return
			}
		}
		imported++
	}
	writeJSON(w, http.StatusOK, map[string]any{"imported": imported})
}

func cell(row []string, i int) string {
	if i >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[i])
}

func splitTags(raw string) []string {
	parts := strings.FieldsFunc(raw, func(r rune) bool { return r == '|' || r == ',' })
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

type vocabSettingsPayload struct {
	Enabled     bool   `json:"enabled"`
	BaseURL     string `json:"baseUrl"`
	Model       string `json:"model"`
	Prompt      string `json:"prompt"`
	APIKey      string `json:"apiKey"`
	ClearAPIKey bool   `json:"clearApiKey"`
}

func vocabSettingsJSON(st *store.VocabSetting) map[string]any {
	if st == nil {
		st = &store.VocabSetting{}
	}
	configured := strings.TrimSpace(st.APIKey) != "" && strings.TrimSpace(st.BaseURL) != "" && strings.TrimSpace(st.Model) != ""
	return map[string]any{
		"enabled":      st.Enabled,
		"baseUrl":      st.BaseURL,
		"model":        st.Model,
		"prompt":       st.Prompt,
		"apiKeyMasked": maskAPIKey(strings.TrimSpace(st.APIKey)),
		"configured":   configured,
	}
}

func (s *APIService) handleVocabGetSettings(w http.ResponseWriter, r *http.Request) {
	user := s.vocabCurrentUser(r)
	if user == nil {
		jsonError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	st, err := s.Store.GetVocabSetting(r.Context(), user.ID)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to load settings")
		return
	}
	writeJSON(w, http.StatusOK, vocabSettingsJSON(st))
}

func (s *APIService) handleVocabPutSettings(w http.ResponseWriter, r *http.Request) {
	user := s.vocabCurrentUser(r)
	if user == nil {
		jsonError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var payload vocabSettingsPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid json")
		return
	}
	st, err := s.Store.GetVocabSetting(r.Context(), user.ID)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to load settings")
		return
	}
	if st == nil {
		st = &store.VocabSetting{CreatorID: user.ID}
	}
	baseURL := strings.TrimSpace(payload.BaseURL)
	if baseURL != "" {
		if _, err := vocablookup.NormalizeChatCompletionsURL(baseURL); err != nil {
			jsonError(w, http.StatusBadRequest, "invalid API base URL")
			return
		}
	}
	if len(baseURL) > 1024 || len(payload.Model) > 256 || len(payload.Prompt) > 8000 || len(payload.APIKey) > 512 {
		jsonError(w, http.StatusBadRequest, "settings fields are too long")
		return
	}
	st.Enabled = payload.Enabled
	st.BaseURL = baseURL
	st.Model = strings.TrimSpace(payload.Model)
	st.Prompt = strings.TrimSpace(payload.Prompt)
	if payload.ClearAPIKey {
		st.APIKey = ""
	} else if strings.TrimSpace(payload.APIKey) != "" {
		st.APIKey = strings.TrimSpace(payload.APIKey)
	}
	if st.Enabled && (st.APIKey == "" || st.BaseURL == "" || st.Model == "") {
		jsonError(w, http.StatusBadRequest, "API key, base URL, and model are required to enable model lookup")
		return
	}
	if err := s.Store.UpsertVocabSetting(r.Context(), st); err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to save settings")
		return
	}
	writeJSON(w, http.StatusOK, vocabSettingsJSON(st))
}

func (s *APIService) handleVocabLookup(w http.ResponseWriter, r *http.Request) {
	user := s.vocabCurrentUser(r)
	if user == nil {
		jsonError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var payload struct {
		Term string `json:"term"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid json")
		return
	}
	term := normalizeTerm(payload.Term)
	if term == "" {
		jsonError(w, http.StatusBadRequest, "term is required")
		return
	}

	st, err := s.Store.GetVocabSetting(r.Context(), user.ID)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to load settings")
		return
	}

	type dictOutcome struct {
		result *vocablookup.Result
		err    error
	}
	dictCh := make(chan dictOutcome, 1)
	go func() {
		ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
		defer cancel()
		res, lookupErr := vocablookup.LookupDictionary(ctx, term)
		dictCh <- dictOutcome{res, lookupErr}
	}()

	var llmResult *vocablookup.Result
	var llmErr error
	useLLM := st != nil && st.Enabled && strings.TrimSpace(st.APIKey) != "" && strings.TrimSpace(st.BaseURL) != "" && strings.TrimSpace(st.Model) != ""
	if useLLM {
		ctx, cancel := context.WithTimeout(r.Context(), vocablookup.LLMTimeout)
		defer cancel()
		llmResult, llmErr = vocablookup.LookupLLM(ctx, vocablookup.LLMConfig{
			BaseURL: st.BaseURL,
			APIKey:  st.APIKey,
			Model:   st.Model,
			Prompt:  st.Prompt,
		}, term)
	}
	dict := <-dictCh

	if useLLM {
		if llmErr != nil {
			jsonError(w, http.StatusBadGateway, llmErr.Error())
			return
		}
		if llmResult != nil {
			writeJSON(w, http.StatusOK, vocablookup.MergeAudio(llmResult, dict.result))
			return
		}
	}

	if dict.err != nil {
		jsonError(w, http.StatusBadGateway, dict.err.Error())
		return
	}
	if dict.result == nil {
		jsonError(w, http.StatusNotFound, "no dictionary entry found")
		return
	}
	writeJSON(w, http.StatusOK, dict.result)
}
