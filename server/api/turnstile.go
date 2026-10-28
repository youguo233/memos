package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/pkg/errors"
	"google.golang.org/grpc/metadata"

	"github.com/usememos/memos/internal/clientip"
	"github.com/usememos/memos/internal/ratelimit"
)

const turnstileVerifyURL = "https://challenges.cloudflare.com/turnstile/v0/siteverify"

var turnstileHTTPClient = &http.Client{Timeout: 10 * time.Second}

// turnstileChallenge verifies Cloudflare Turnstile tokens with keys from the
// CAPTCHA instance setting.
type turnstileChallenge struct {
	siteKey   string
	secretKey string
}

func (*turnstileChallenge) Provider() string  { return "turnstile" }
func (t *turnstileChallenge) SiteKey() string { return t.siteKey }

func (t *turnstileChallenge) Verify(ctx context.Context, _ ratelimit.Scope, token string, remoteIP string) error {
	form := url.Values{"secret": {t.secretKey}, "response": {token}}
	if remoteIP != "" {
		form.Set("remoteip", remoteIP)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, turnstileVerifyURL, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := turnstileHTTPClient.Do(req)
	if err != nil {
		return errors.Wrap(err, "turnstile siteverify request failed")
	}
	defer resp.Body.Close()
	var result struct {
		Success    bool     `json:"success"`
		ErrorCodes []string `json:"error-codes"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return errors.Wrap(err, "invalid turnstile siteverify response")
	}
	if !result.Success {
		return errors.Errorf("turnstile rejected token: %v", result.ErrorCodes)
	}
	return nil
}

// activeChallenge returns the deployment-configured challenge, or a Turnstile
// challenge when the CAPTCHA instance setting is enabled with both keys.
func (s *APIService) activeChallenge(ctx context.Context) ratelimit.Challenge {
	if s.Challenge != nil {
		return s.Challenge
	}
	setting, err := s.Store.GetInstanceCaptchaSetting(ctx)
	if err != nil {
		slog.Warn("failed to load captcha setting", "error", err)
		return nil
	}
	if setting == nil || !setting.Enabled || setting.SiteKey == "" || setting.SecretKey == "" {
		return nil
	}
	return &turnstileChallenge{siteKey: setting.SiteKey, secretKey: setting.SecretKey}
}

// challengeRemoteIP prefers CF-Connecting-IP / X-Forwarded-For (the instance
// usually sits behind Cloudflare) and falls back to the resolved client IP.
// It is only a hint to siteverify.
func challengeRemoteIP(ctx context.Context) string {
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if v := md.Get("cf-connecting-ip"); len(v) > 0 && strings.TrimSpace(v[0]) != "" {
			return strings.TrimSpace(v[0])
		}
		if v := md.Get("x-forwarded-for"); len(v) > 0 {
			if ip := strings.TrimSpace(strings.Split(v[0], ",")[0]); ip != "" {
				return ip
			}
		}
	}
	return clientip.FromContext(ctx)
}
