package accountpool

import (
	"context"
	crand "crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type tokenPayload struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	IDToken      string `json:"id_token"`
	ExpiresIn    int64  `json:"expires_in"`
}

func (pool *AccountPool) StartLogin(rawName string) (LoginFlow, error) {
	name, err := normalizeProfileName(rawName, false)
	if err != nil {
		return LoginFlow{}, err
	}
	if err := pool.ensureProfileScaffold(name); err != nil {
		return LoginFlow{}, err
	}

	verifierBytes := make([]byte, 32)
	if _, err := io.ReadFull(crand.Reader, verifierBytes); err != nil {
		return LoginFlow{}, err
	}
	verifier := base64.RawURLEncoding.EncodeToString(verifierBytes)
	hash := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(hash[:])

	stateBytes := make([]byte, 16)
	if _, err := io.ReadFull(crand.Reader, stateBytes); err != nil {
		return LoginFlow{}, err
	}
	state := fmt.Sprintf("%x", stateBytes)

	u, err := url.Parse(pool.authorizeURL)
	if err != nil {
		return LoginFlow{}, err
	}
	query := u.Query()
	query.Set("response_type", "code")
	query.Set("client_id", defaultOAuthClientID)
	query.Set("redirect_uri", pool.redirectURL)
	query.Set("scope", defaultOAuthScope)
	query.Set("code_challenge", challenge)
	query.Set("code_challenge_method", "S256")
	query.Set("state", state)
	query.Set("id_token_add_organizations", "true")
	query.Set("codex_cli_simplified_flow", "true")
	query.Set("originator", "pi")
	query.Set("prompt", "login")
	u.RawQuery = query.Encode()

	return LoginFlow{
		ProfileName: name,
		AuthURL:     u.String(),
		Verifier:    verifier,
		State:       state,
		RedirectURL: pool.redirectURL,
	}, nil
}

func (pool *AccountPool) CompleteLogin(profileName, code, verifier string) (OAuthTokens, error) {
	name, err := normalizeProfileName(profileName, false)
	if err != nil {
		return OAuthTokens{}, err
	}
	if strings.TrimSpace(code) == "" {
		return OAuthTokens{}, errors.New("缺少授权 code")
	}
	if strings.TrimSpace(verifier) == "" {
		return OAuthTokens{}, errors.New("缺少登录校验信息")
	}

	form := url.Values{
		"grant_type":    {"authorization_code"},
		"client_id":     {defaultOAuthClientID},
		"code":          {code},
		"code_verifier": {verifier},
		"redirect_uri":  {pool.redirectURL},
	}
	payload, err := pool.postTokenRequest(form)
	if err != nil {
		return OAuthTokens{}, err
	}
	tokens, err := pool.tokensFromPayload(payload, "")
	if err != nil {
		return OAuthTokens{}, err
	}
	if err := pool.PersistTokens(name, tokens); err != nil {
		return OAuthTokens{}, err
	}
	return tokens, nil
}

func (pool *AccountPool) RefreshTokens(refreshToken string) (OAuthTokens, error) {
	refreshToken = strings.TrimSpace(refreshToken)
	if refreshToken == "" {
		return OAuthTokens{}, errors.New("缺少 refresh token")
	}
	form := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
		"client_id":     {defaultOAuthClientID},
	}
	payload, err := pool.postTokenRequest(form)
	if err != nil {
		return OAuthTokens{}, err
	}
	return pool.tokensFromPayload(payload, refreshToken)
}

func (pool *AccountPool) postTokenRequest(form url.Values) (tokenPayload, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, pool.tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return tokenPayload{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := pool.httpClient.Do(req)
	if err != nil {
		return tokenPayload{}, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return tokenPayload{}, &remoteStatusError{
			Operation:  "token_exchange",
			StatusCode: resp.StatusCode,
			Body:       strings.TrimSpace(string(body)),
		}
	}
	var payload tokenPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return tokenPayload{}, err
	}
	return payload, nil
}

func (pool *AccountPool) tokensFromPayload(payload tokenPayload, fallbackRefreshToken string) (OAuthTokens, error) {
	refreshToken := firstNonEmpty(payload.RefreshToken, fallbackRefreshToken)
	if payload.AccessToken == "" || refreshToken == "" || payload.ExpiresIn == 0 {
		return OAuthTokens{}, errors.New("token_response_missing_fields")
	}
	accountID := extractAccountID(payload.AccessToken)
	if accountID == "" {
		return OAuthTokens{}, errors.New("token_account_id_missing")
	}
	return OAuthTokens{
		Access:    payload.AccessToken,
		Refresh:   refreshToken,
		IDToken:   payload.IDToken,
		Expires:   pool.now().Add(time.Duration(payload.ExpiresIn) * time.Second).UnixMilli(),
		AccountID: accountID,
		Email:     extractEmail(payload.AccessToken),
	}, nil
}
