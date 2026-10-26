package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"google.golang.org/grpc/metadata"

	"github.com/usememos/memos/internal/random"
	"github.com/usememos/memos/server/auth"
	"github.com/usememos/memos/store"
)

// Passkey (WebAuthn) endpoints, plain JSON over HTTP:
//
//	POST   /api/v1/passkeys/register/begin    (auth) -> {sessionId, options}
//	POST   /api/v1/passkeys/register/finish   (auth) ?sessionId=&name=  body: credential JSON
//	GET    /api/v1/passkeys                   (auth) list
//	DELETE /api/v1/passkeys/{id}              (auth)
//	POST   /api/v1/passkeys/login/begin              -> {sessionId, options} (discoverable)
//	POST   /api/v1/passkeys/login/finish             ?sessionId=  body: assertion JSON
//
// RP ID / origins come from MEMOS_WEBAUTHN_RP_ID and MEMOS_WEBAUTHN_ORIGINS
// (comma separated) or, when unset, from X-Forwarded-Host/Proto or Host.

type passkeyUser struct {
	user  *store.User
	creds []webauthn.Credential
}

func (u *passkeyUser) WebAuthnID() []byte          { return []byte(strconv.Itoa(int(u.user.ID))) }
func (u *passkeyUser) WebAuthnName() string        { return u.user.Username }
func (u *passkeyUser) WebAuthnDisplayName() string { return firstNonEmpty(u.user.Nickname, u.user.Username) }
func (u *passkeyUser) WebAuthnCredentials() []webauthn.Credential {
	return u.creds
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

type passkeyCeremony struct {
	data    *webauthn.SessionData
	userID  int32
	expires time.Time
}

var passkeySessions = struct {
	sync.Mutex
	m map[string]passkeyCeremony
}{m: map[string]passkeyCeremony{}}

func putCeremony(c passkeyCeremony) string {
	id := random.UUID()
	passkeySessions.Lock()
	defer passkeySessions.Unlock()
	now := time.Now()
	for k, v := range passkeySessions.m {
		if now.After(v.expires) {
			delete(passkeySessions.m, k)
		}
	}
	c.expires = now.Add(5 * time.Minute)
	passkeySessions.m[id] = c
	return id
}

func takeCeremony(id string) (passkeyCeremony, bool) {
	passkeySessions.Lock()
	defer passkeySessions.Unlock()
	c, ok := passkeySessions.m[id]
	delete(passkeySessions.m, id)
	if !ok || time.Now().After(c.expires) {
		return passkeyCeremony{}, false
	}
	return c, true
}

func firstHeader(r *http.Request, key string) string {
	v := r.Header.Get(key)
	if i := strings.IndexByte(v, ','); i >= 0 {
		v = v[:i]
	}
	return strings.TrimSpace(v)
}

func webAuthnFor(r *http.Request) (*webauthn.WebAuthn, error) {
	rpID := strings.TrimSpace(os.Getenv("MEMOS_WEBAUTHN_RP_ID"))
	var origins []string
	for _, o := range strings.Split(os.Getenv("MEMOS_WEBAUTHN_ORIGINS"), ",") {
		if o = strings.TrimRight(strings.TrimSpace(o), "/"); o != "" {
			origins = append(origins, o)
		}
	}
	if len(origins) == 0 {
		host := firstNonEmpty(firstHeader(r, "X-Forwarded-Host"), r.Host)
		proto := firstHeader(r, "X-Forwarded-Proto")
		if proto == "" {
			proto = "http"
			if r.TLS != nil {
				proto = "https"
			}
		}
		origins = []string{proto + "://" + host}
	}
	if rpID == "" {
		u, err := url.Parse(origins[0])
		if err != nil {
			return nil, err
		}
		rpID = u.Hostname()
	}
	return webauthn.New(&webauthn.Config{
		RPID:          rpID,
		RPDisplayName: "Memos",
		RPOrigins:     origins,
		AuthenticatorSelection: protocol.AuthenticatorSelection{
			ResidentKey:      protocol.ResidentKeyRequirementRequired,
			UserVerification: protocol.VerificationPreferred,
		},
	})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func jsonError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"message": msg})
}

func (s *APIService) loadPasskeyUser(ctx context.Context, user *store.User) (*passkeyUser, []*store.UserPasskey, error) {
	rows, err := s.Store.ListUserPasskeys(ctx, &store.FindUserPasskey{UserID: &user.ID})
	if err != nil {
		return nil, nil, err
	}
	pu := &passkeyUser{user: user}
	for _, row := range rows {
		var c webauthn.Credential
		if json.Unmarshal([]byte(row.Credential), &c) == nil {
			pu.creds = append(pu.creds, c)
		}
	}
	return pu, rows, nil
}

// RegisterPasskeyRoutes mounts the passkey endpoints on mux.
func (s *APIService) RegisterPasskeyRoutes(mux *http.ServeMux) {
	authn := auth.NewAuthenticator(s.Store, s.Secret)
	current := func(r *http.Request) *store.User {
		u, err := authn.AuthenticateToUser(r.Context(), r.Header.Get("Authorization"), r.Header.Get("Cookie"))
		if err != nil || u == nil || u.RowStatus != store.Normal {
			return nil
		}
		return u
	}

	mux.HandleFunc("GET /api/v1/passkeys", func(w http.ResponseWriter, r *http.Request) {
		user := current(r)
		if user == nil {
			jsonError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		rows, err := s.Store.ListUserPasskeys(r.Context(), &store.FindUserPasskey{UserID: &user.ID})
		if err != nil {
			jsonError(w, http.StatusInternalServerError, "failed to list passkeys")
			return
		}
		out := []map[string]any{}
		for _, p := range rows {
			out = append(out, map[string]any{"id": p.ID, "name": p.Name, "createdTs": p.CreatedTs, "lastUsedTs": p.LastUsedTs})
		}
		writeJSON(w, http.StatusOK, map[string]any{"passkeys": out})
	})

	mux.HandleFunc("DELETE /api/v1/passkeys/{id}", func(w http.ResponseWriter, r *http.Request) {
		user := current(r)
		if user == nil {
			jsonError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		id, err := strconv.Atoi(r.PathValue("id"))
		if err != nil {
			jsonError(w, http.StatusBadRequest, "invalid id")
			return
		}
		ok, err := s.Store.DeleteUserPasskey(r.Context(), user.ID, int32(id))
		if err != nil {
			jsonError(w, http.StatusInternalServerError, "failed to delete passkey")
			return
		}
		if !ok {
			jsonError(w, http.StatusNotFound, "passkey not found")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{})
	})

	mux.HandleFunc("POST /api/v1/passkeys/register/begin", func(w http.ResponseWriter, r *http.Request) {
		user := current(r)
		if user == nil {
			jsonError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		wa, err := webAuthnFor(r)
		if err != nil {
			jsonError(w, http.StatusInternalServerError, "invalid WebAuthn configuration")
			return
		}
		pu, _, err := s.loadPasskeyUser(r.Context(), user)
		if err != nil {
			jsonError(w, http.StatusInternalServerError, "failed to load passkeys")
			return
		}
		exclude := make([]protocol.CredentialDescriptor, 0, len(pu.creds))
		for _, c := range pu.creds {
			exclude = append(exclude, c.Descriptor())
		}
		options, data, err := wa.BeginRegistration(pu, webauthn.WithExclusions(exclude))
		if err != nil {
			jsonError(w, http.StatusInternalServerError, "failed to begin registration")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"sessionId": putCeremony(passkeyCeremony{data: data, userID: user.ID}), "options": options})
	})

	mux.HandleFunc("POST /api/v1/passkeys/register/finish", func(w http.ResponseWriter, r *http.Request) {
		user := current(r)
		if user == nil {
			jsonError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		c, ok := takeCeremony(r.URL.Query().Get("sessionId"))
		if !ok || c.userID != user.ID {
			jsonError(w, http.StatusBadRequest, "registration session expired")
			return
		}
		wa, err := webAuthnFor(r)
		if err != nil {
			jsonError(w, http.StatusInternalServerError, "invalid WebAuthn configuration")
			return
		}
		pu, _, err := s.loadPasskeyUser(r.Context(), user)
		if err != nil {
			jsonError(w, http.StatusInternalServerError, "failed to load passkeys")
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
		cred, err := wa.FinishRegistration(pu, *c.data, r)
		if err != nil {
			jsonError(w, http.StatusBadRequest, "passkey verification failed")
			return
		}
		raw, _ := json.Marshal(cred)
		name := strings.TrimSpace(r.URL.Query().Get("name"))
		if len(name) > 100 {
			name = name[:100]
		}
		if name == "" {
			name = "Passkey " + time.Now().UTC().Format("2006-01-02")
		}
		p, err := s.Store.CreateUserPasskey(r.Context(), &store.UserPasskey{
			UserID: user.ID, CredentialID: base64.RawURLEncoding.EncodeToString(cred.ID), Name: name, Credential: string(raw),
		})
		if err != nil {
			jsonError(w, http.StatusInternalServerError, "failed to save passkey")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"id": p.ID, "name": p.Name, "createdTs": p.CreatedTs, "lastUsedTs": 0})
	})

	mux.HandleFunc("POST /api/v1/passkeys/login/begin", func(w http.ResponseWriter, r *http.Request) {
		wa, err := webAuthnFor(r)
		if err != nil {
			jsonError(w, http.StatusInternalServerError, "invalid WebAuthn configuration")
			return
		}
		options, data, err := wa.BeginDiscoverableLogin()
		if err != nil {
			jsonError(w, http.StatusInternalServerError, "failed to begin login")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"sessionId": putCeremony(passkeyCeremony{data: data}), "options": options})
	})

	mux.HandleFunc("POST /api/v1/passkeys/login/finish", func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		c, ok := takeCeremony(r.URL.Query().Get("sessionId"))
		if !ok {
			jsonError(w, http.StatusBadRequest, "login session expired")
			return
		}
		wa, err := webAuthnFor(r)
		if err != nil {
			jsonError(w, http.StatusInternalServerError, "invalid WebAuthn configuration")
			return
		}
		var matched *store.UserPasskey
		var matchedUser *store.User
		handler := func(rawID, userHandle []byte) (webauthn.User, error) {
			credID := base64.RawURLEncoding.EncodeToString(rawID)
			p, err := s.Store.GetUserPasskey(ctx, &store.FindUserPasskey{CredentialID: &credID})
			if err != nil || p == nil {
				return nil, protocol.ErrBadRequest.WithDetails("unknown credential")
			}
			if string(userHandle) != strconv.Itoa(int(p.UserID)) {
				return nil, protocol.ErrBadRequest.WithDetails("user handle mismatch")
			}
			u, err := s.Store.GetUser(ctx, &store.FindUser{ID: &p.UserID})
			if err != nil || u == nil {
				return nil, protocol.ErrBadRequest.WithDetails("unknown user")
			}
			pu, _, err := s.loadPasskeyUser(ctx, u)
			if err != nil {
				return nil, err
			}
			matched, matchedUser = p, u
			return pu, nil
		}
		r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
		cred, err := wa.FinishDiscoverableLogin(handler, *c.data, r)
		if err != nil || matched == nil {
			jsonError(w, http.StatusUnauthorized, "passkey sign-in failed")
			return
		}
		if matchedUser.RowStatus != store.Normal {
			jsonError(w, http.StatusForbidden, "user has been archived")
			return
		}
		if raw, err := json.Marshal(cred); err == nil {
			_ = s.Store.UpdateUserPasskeyUsage(ctx, matched.ID, string(raw))
		}
		// Reuse the regular sign-in session issuance (refresh cookie + access token).
		md := metadata.MD{}
		for k, v := range r.Header {
			md.Append(strings.ToLower(k), v...)
		}
		sctx := WithHeaderCarrier(metadata.NewIncomingContext(ctx, md))
		accessToken, expiresAt, err := s.doSignIn(sctx, matchedUser)
		if err != nil {
			jsonError(w, http.StatusInternalServerError, "failed to sign in")
			return
		}
		for k, v := range GetHeaderCarrier(sctx).All() {
			w.Header().Add(k, v)
		}
		writeJSON(w, http.StatusOK, map[string]any{"accessToken": accessToken, "accessTokenExpireTime": expiresAt.UTC().Format(time.RFC3339)})
	})
}
