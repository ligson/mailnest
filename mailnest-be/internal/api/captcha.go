package api

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"html"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"mailnest-be/internal/response"
)

const captchaTTL = 5 * time.Minute

type captchaStore struct {
	mu      sync.Mutex
	entries map[string]captchaEntry
}

type captchaEntry struct {
	answer    string
	expiresAt time.Time
}

func newCaptchaStore() *captchaStore {
	return &captchaStore{entries: map[string]captchaEntry{}}
}

func (s *captchaStore) newChallenge() (string, string, time.Time, error) {
	id, err := randomCaptchaID()
	if err != nil {
		return "", "", time.Time{}, err
	}
	answer, err := randomCaptchaText(4)
	if err != nil {
		return "", "", time.Time{}, err
	}
	expiresAt := time.Now().Add(captchaTTL)

	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanupLocked(time.Now())
	s.entries[id] = captchaEntry{
		answer:    strings.ToLower(answer),
		expiresAt: expiresAt,
	}
	return id, answer, expiresAt, nil
}

func (s *captchaStore) verify(id, answer string) bool {
	id = strings.TrimSpace(id)
	answer = strings.ToLower(strings.TrimSpace(answer))
	if id == "" || answer == "" {
		return false
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.entries[id]
	delete(s.entries, id)
	if !ok || time.Now().After(entry.expiresAt) {
		return false
	}
	return answer == entry.answer
}

func (s *captchaStore) cleanupLocked(now time.Time) {
	for id, entry := range s.entries {
		if now.After(entry.expiresAt) {
			delete(s.entries, id)
		}
	}
}

func (a *App) handleCaptcha(w http.ResponseWriter, r *http.Request) {
	id, text, expiresAt, err := a.captchas.newChallenge()
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "验证码生成失败")
		return
	}
	response.OK(w, "获取成功", map[string]any{
		"id":            id,
		"imageData":     captchaSVGDataURL(text, id),
		"expireSeconds": int(time.Until(expiresAt).Seconds()),
	})
}

func randomCaptchaID() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func randomCaptchaText(length int) (string, error) {
	const alphabet = "23456789ABCDEFGHJKLMNPQRSTUVWXYZ"
	var builder strings.Builder
	for i := 0; i < length; i++ {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(alphabet))))
		if err != nil {
			return "", err
		}
		builder.WriteByte(alphabet[n.Int64()])
	}
	return builder.String(), nil
}

func captchaSVGDataURL(text, seed string) string {
	colors := []string{"#0f766e", "#2563eb", "#7c3aed", "#b45309", "#be123c"}
	var lines strings.Builder
	for i := 0; i < 6; i++ {
		x1 := 10 + i*18
		y1 := 8 + (int(seed[i%len(seed)]) % 34)
		x2 := 142 - i*11
		y2 := 8 + (int(seed[(i+5)%len(seed)]) % 34)
		lines.WriteString(fmt.Sprintf(`<line x1="%d" y1="%d" x2="%d" y2="%d" stroke="%s" stroke-width="1.2" opacity="0.28"/>`, x1, y1, x2, y2, colors[i%len(colors)]))
	}

	var chars strings.Builder
	for index, r := range text {
		x := 24 + index*31
		y := 34 + (int(seed[(index+2)%len(seed)]) % 7) - 3
		rotate := (int(seed[(index+8)%len(seed)]) % 18) - 9
		chars.WriteString(fmt.Sprintf(`<text x="%d" y="%d" transform="rotate(%d %d %d)" fill="%s">%s</text>`, x, y, rotate, x, y, colors[index%len(colors)], html.EscapeString(string(r))))
	}

	svg := fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="150" height="48" viewBox="0 0 150 48"><rect width="150" height="48" rx="8" fill="#f8fafc" stroke="#cbd5e1"/><g font-family="Arial, sans-serif" font-size="25" font-weight="700" letter-spacing="3">%s</g>%s</svg>`, chars.String(), lines.String())
	return "data:image/svg+xml;charset=utf-8," + url.PathEscape(svg)
}
