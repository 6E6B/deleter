package discord

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func trimEndpointQuery(endpoint string) string {
	if idx := strings.Index(endpoint, "?"); idx >= 0 {
		return endpoint[:idx]
	}
	return endpoint
}

func isDiscordID(segment string) bool {
	if segment == "" {
		return false
	}
	for _, r := range segment {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func normalizeRouteKey(method, endpoint string) string {
	path := strings.Trim(trimEndpointQuery(endpoint), "/")
	if path == "" {
		return method + " /"
	}

	segments := strings.Split(path, "/")
	preserveNext := false
	for i, segment := range segments {
		if preserveNext {
			preserveNext = false
			continue
		}
		switch segment {
		case "channels", "guilds", "webhooks":
			preserveNext = true
		default:
			if isDiscordID(segment) {
				segments[i] = ":id"
			}
		}
	}

	return method + " /" + strings.Join(segments, "/")
}

func majorParameterKey(endpoint string) string {
	segments := strings.Split(strings.Trim(trimEndpointQuery(endpoint), "/"), "/")
	for i := 0; i+1 < len(segments); i++ {
		switch segments[i] {
		case "channels", "guilds", "webhooks":
			return segments[i] + "/" + segments[i+1]
		}
	}
	return "global"
}

func parseSeconds(value string) (time.Duration, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, false
	}
	f, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, false
	}
	return time.Duration(f * float64(time.Second)), true
}

func parseRetryAfter(headers http.Header, body []byte) time.Duration {
	for _, key := range []string{"Retry-After", "X-RateLimit-Reset-After"} {
		if wait, ok := parseSeconds(headers.Get(key)); ok {
			return wait + 100*time.Millisecond
		}
	}

	var rl struct {
		RetryAfter float64 `json:"retry_after"`
	}
	if err := json.Unmarshal(body, &rl); err == nil && rl.RetryAfter > 0 {
		return time.Duration(rl.RetryAfter*float64(time.Second)) + 100*time.Millisecond
	}

	return 5 * time.Second
}

func parseRemaining(headers http.Header) (int, bool) {
	value := strings.TrimSpace(headers.Get("X-RateLimit-Remaining"))
	if value == "" {
		return 0, false
	}
	remaining, err := strconv.Atoi(value)
	if err != nil {
		return 0, false
	}
	return remaining, true
}

func (s *Session) waitForRateLimit(method, endpoint string) {
	routeKey := normalizeRouteKey(method, endpoint)

	for {
		now := time.Now()
		var wait time.Duration
		label := ""

		if s.globalReset.After(now) {
			wait = time.Until(s.globalReset)
			label = "global"
		}

		for _, key := range s.rateLimitKeys(routeKey) {
			resetAt, ok := s.rateLimitResets[key]
			if !ok {
				continue
			}
			if !resetAt.After(now) {
				delete(s.rateLimitResets, key)
				continue
			}
			if routeWait := time.Until(resetAt); routeWait > wait {
				wait = routeWait
				label = "route"
			}
		}

		if wait <= 0 {
			s.consecutiveWaits = 0
			return
		}

		_ = label
		s.consecutiveWaits++
		if s.consecutiveWaits >= 3 {
			cooldown := 30 * time.Second
			fmt.Printf("Rate limited repeatedly, cooling down for %s...\n", cooldown)
			time.Sleep(cooldown)
			s.consecutiveWaits = 0
			continue
		}

		time.Sleep(wait)
	}
}

func (s *Session) rateLimitKeys(routeKey string) []string {
	keys := []string{routeKey}
	if bucketKey, ok := s.routeBuckets[routeKey]; ok {
		keys = append(keys, bucketKey)
	}
	return keys
}

func (s *Session) updateRateLimitState(method, endpoint string, headers http.Header, body []byte, statusCode int) {
	routeKey := normalizeRouteKey(method, endpoint)
	if bucket := strings.TrimSpace(headers.Get("X-RateLimit-Bucket")); bucket != "" {
		s.routeBuckets[routeKey] = bucket + "|" + majorParameterKey(endpoint)
	}

	if statusCode == 429 {
		wait := parseRetryAfter(headers, body)
		if strings.EqualFold(headers.Get("X-RateLimit-Global"), "true") {
			s.globalReset = time.Now().Add(wait)
			return
		}

		key := routeKey
		if bucketKey, ok := s.routeBuckets[routeKey]; ok {
			key = bucketKey
		}
		s.rateLimitResets[key] = time.Now().Add(wait)
		return
	}

	remaining, hasRemaining := parseRemaining(headers)
	resetAfter, hasResetAfter := parseSeconds(headers.Get("X-RateLimit-Reset-After"))
	if !hasRemaining || !hasResetAfter {
		return
	}

	for _, key := range s.rateLimitKeys(routeKey) {
		if remaining <= 0 {
			s.rateLimitResets[key] = time.Now().Add(resetAfter)
		} else {
			delete(s.rateLimitResets, key)
		}
	}
}
