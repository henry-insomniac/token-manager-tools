package accountpool

import (
	"encoding/base64"
	"encoding/json"
	"strings"
)

func decodeJWTPayload(token string) map[string]any {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil
	}
	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		return nil
	}
	return decoded
}

func extractAccountID(accessToken string) string {
	payload := decodeJWTPayload(accessToken)
	auth, ok := payload[openAIAuthClaim].(map[string]any)
	if !ok {
		return ""
	}
	return anyString(auth["chatgpt_account_id"])
}

func extractEmail(accessToken string) string {
	payload := decodeJWTPayload(accessToken)
	profile, ok := payload[openAIProfileClaim].(map[string]any)
	if !ok {
		return ""
	}
	return anyString(profile["email"])
}
