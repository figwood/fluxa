package security

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type Claims struct {
	UserID      int64    `json:"user_id"`
	UserName    string   `json:"user_name"`
	UserEmail   string   `json:"user_email"`
	GlobalRoles []string `json:"global_roles"`
	AuthVersion int      `json:"auth_version"`
	ExpiresAt   int64    `json:"exp"`
	IssuedAt    int64    `json:"iat"`
}

func GenerateAccessToken(secret string, ttl time.Duration, claims Claims) (string, error) {
	if strings.TrimSpace(secret) == "" {
		return "", errors.New("missing_jwt_secret")
	}
	now := time.Now()
	claims.IssuedAt = now.Unix()
	claims.ExpiresAt = now.Add(ttl).Unix()
	header := map[string]string{"alg": "HS256", "typ": "JWT"}
	headerJSON, err := json.Marshal(header)
	if err != nil {
		return "", err
	}
	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	unsigned := base64.RawURLEncoding.EncodeToString(headerJSON) + "." + base64.RawURLEncoding.EncodeToString(claimsJSON)
	return unsigned + "." + signJWT(secret, unsigned), nil
}

func ParseAccessToken(secret, token string) (Claims, error) {
	if strings.TrimSpace(secret) == "" {
		return Claims{}, errors.New("missing_jwt_secret")
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return Claims{}, errors.New("invalid_token")
	}
	unsigned := parts[0] + "." + parts[1]
	if !hmac.Equal([]byte(parts[2]), []byte(signJWT(secret, unsigned))) {
		return Claims{}, errors.New("invalid_signature")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return Claims{}, err
	}
	var claims Claims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return Claims{}, err
	}
	if claims.ExpiresAt <= time.Now().Unix() {
		return Claims{}, errors.New("token_expired")
	}
	return claims, nil
}

func signJWT(secret, unsigned string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(unsigned))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func TokenDebugString(claims Claims) string {
	return fmt.Sprintf("%s:%s:%s", strconv.FormatInt(claims.UserID, 10), claims.UserName, claims.UserEmail)
}
