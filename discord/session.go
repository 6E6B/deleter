package discord

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	utls "github.com/refraction-networking/utls"
)

const (
	apiBase          = "https://discord.com/api/v9"
	clientVersion    = "0.0.670"
	electronVer      = "35.1.5"
	chromeVer        = "134.0.6998.179"
	buildNumber      = 397417
	releaseChannel   = "stable"
	windowsOSVersion = "10.0.22631"
)

var DebugMode = os.Getenv("DEBUG") == "1"

type Session struct {
	mu sync.Mutex

	token            string
	fingerprint      string
	userAgent        string
	superPropsB64    string
	props            superProperties
	client           *http.Client
	heartbeatCreated time.Time
	locale           string
	timezone         string
	referer          string
	rateLimitResets  map[string]time.Time
	routeBuckets     map[string]string
	globalReset      time.Time
	consecutiveWaits int
}

func windowsUserAgent() string {
	return fmt.Sprintf(
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) discord/%s Chrome/%s Electron/%s Safari/537.36",
		clientVersion, chromeVer, electronVer,
	)
}

func newChromeTransport() http.RoundTripper {
	dialer := &net.Dialer{Timeout: 30 * time.Second}
	return &http.Transport{
		DialTLSContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			conn, err := dialer.DialContext(ctx, network, addr)
			if err != nil {
				return nil, err
			}
			host, _, _ := net.SplitHostPort(addr)

			spec, err := utls.UTLSIdToSpec(utls.HelloChrome_Auto)
			if err != nil {
				conn.Close()
				return nil, err
			}
			for _, ext := range spec.Extensions {
				if alpn, ok := ext.(*utls.ALPNExtension); ok {
					alpn.AlpnProtocols = []string{"http/1.1"}
				}
			}

			uconn := utls.UClient(conn, &utls.Config{ServerName: host}, utls.HelloCustom)
			if err := uconn.ApplyPreset(&spec); err != nil {
				conn.Close()
				return nil, err
			}
			if err := uconn.HandshakeContext(ctx); err != nil {
				conn.Close()
				return nil, err
			}
			return uconn, nil
		},
	}
}

func NewSession(token string) *Session {
	jar, _ := cookiejar.New(nil)
	locale := detectLocale()
	discordURL, _ := url.Parse("https://discord.com")
	jar.SetCookies(discordURL, []*http.Cookie{
		{Name: "locale", Value: locale},
	})

	ua := windowsUserAgent()

	ref := "https://discord.com/channels/@me"
	if token == "" {
		ref = "https://discord.com/login"
	}

	s := &Session{
		token:     token,
		userAgent: ua,
		client: &http.Client{
			Timeout:   30 * time.Second,
			Jar:       jar,
			Transport: newChromeTransport(),
		},
		locale:          locale,
		timezone:        detectTimezone(),
		referer:         ref,
		rateLimitResets: make(map[string]time.Time),
		routeBuckets:    make(map[string]string),
	}

	s.props = superProperties{
		OS:                       "Windows",
		Browser:                  "Discord Client",
		ReleaseChannel:           releaseChannel,
		ClientVersion:            clientVersion,
		OSVersion:                windowsOSVersion,
		OSArch:                   "x64",
		AppArch:                  "x64",
		SystemLocale:             locale,
		HasClientMods:            false,
		ClientLaunchID:           generateUUID(),
		LaunchSignature:          generateLaunchSignature(),
		BrowserUserAgent:         ua,
		BrowserVersion:           electronVer,
		ClientBuildNumber:        buildNumber,
		NativeBuildNumber:        nil,
		ClientEventSource:        nil,
		ClientHeartbeatSessionID: generateUUID(),
	}
	s.heartbeatCreated = time.Now()
	s.superPropsB64 = encodeSuperProperties(s.props)
	return s
}

func (s *Session) SetToken(token string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.token = token
	s.fingerprint = ""
	s.referer = "https://discord.com/channels/@me"
}

func (s *Session) SetReferer(ref string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.referer = ref
}

func (s *Session) refreshHeartbeat() {
	if time.Since(s.heartbeatCreated) > 30*time.Minute {
		s.props.ClientHeartbeatSessionID = generateUUID()
		s.heartbeatCreated = time.Now()
		s.superPropsB64 = encodeSuperProperties(s.props)
	}
}

func (s *Session) doRequestCore(method, endpoint string, reqBody io.Reader) ([]byte, int, http.Header, error) {
	s.mu.Lock()
	s.refreshHeartbeat()
	s.waitForRateLimit(method, endpoint)
	token := s.token
	fingerprint := s.fingerprint
	userAgent := s.userAgent
	superPropsB64 := s.superPropsB64
	locale := s.locale
	timezone := s.timezone
	referer := s.referer
	s.mu.Unlock()

	req, err := http.NewRequest(method, apiBase+endpoint, reqBody)
	if err != nil {
		return nil, 0, nil, err
	}

	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Language", locale)
	if token != "" {
		req.Header.Set("Authorization", token)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "https://discord.com")
	req.Header.Set("Referer", referer)
	req.Header.Set("Sec-Ch-Ua", `"Chromium";v="134", "Not:A-Brand";v="24"`)
	req.Header.Set("Sec-Ch-Ua-Mobile", "?0")
	req.Header.Set("Sec-Ch-Ua-Platform", `"Windows"`)
	req.Header.Set("Sec-Fetch-Dest", "empty")
	req.Header.Set("Sec-Fetch-Mode", "cors")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("X-Debug-Options", "bugReporterEnabled")
	req.Header.Set("X-Discord-Locale", locale)
	req.Header.Set("X-Discord-Timezone", timezone)
	req.Header.Set("X-Super-Properties", superPropsB64)
	if fingerprint != "" {
		req.Header.Set("X-Fingerprint", fingerprint)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, 0, nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, resp.Header, err
	}

	s.mu.Lock()
	s.updateRateLimitState(method, endpoint, resp.Header, body, resp.StatusCode)
	s.mu.Unlock()

	return body, resp.StatusCode, resp.Header, nil
}

func (s *Session) doRequest(method, endpoint string) ([]byte, int, http.Header, error) {
	return s.doRequestCore(method, endpoint, nil)
}

func (s *Session) doRequestWithBody(method, endpoint string, payload interface{}) ([]byte, int, http.Header, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, 0, nil, err
	}
	return s.doRequestCore(method, endpoint, bytes.NewReader(data))
}

func (s *Session) SeedCookies() error {
	req, err := http.NewRequest("GET", "https://discord.com/login", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", s.locale)
	req.Header.Set("Sec-Fetch-Dest", "document")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.Header.Set("Sec-Fetch-Site", "none")
	req.Header.Set("Sec-Fetch-User", "?1")
	req.Header.Set("User-Agent", s.userAgent)
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(io.Discard, resp.Body)
	closeErr := resp.Body.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	if DebugMode {
		u, _ := url.Parse("https://discord.com")
		fmt.Printf("    [debug] seeded cookies: ")
		for i, c := range s.client.Jar.Cookies(u) {
			if i > 0 {
				fmt.Print(", ")
			}
			fmt.Print(c.Name)
		}
		fmt.Println()
	}
	return nil
}

func encodeSuperProperties(props superProperties) string {
	b, _ := json.Marshal(props)
	return base64.StdEncoding.EncodeToString(b)
}

func generateUUID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "00000000-0000-4000-8000-000000000000"
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	h := hex.EncodeToString(b)
	return h[0:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:32]
}

func generateLaunchSignature() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "00000000-0000-4000-8000-000000000000"
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80

	detectionBits, _ := new(big.Int).SetString(
		"00000000100000000001000000010000000010000001000000001000000000000010000010000001000000000100000000000001000000000000100000000000", 2)

	allOnes := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 128), big.NewInt(1))
	mask := new(big.Int).AndNot(allOnes, detectionBits)

	uuidInt := new(big.Int).SetBytes(b)
	uuidInt.And(uuidInt, mask)

	result := make([]byte, 16)
	raw := uuidInt.Bytes()
	copy(result[16-len(raw):], raw)

	h := hex.EncodeToString(result)
	return h[0:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:32]
}

func detectLocale() string {
	for _, env := range []string{"LC_ALL", "LC_MESSAGES", "LANG", "LANGUAGE"} {
		if v := os.Getenv(env); v != "" {
			v = strings.Split(v, ".")[0]
			v = strings.Replace(v, "_", "-", 1)
			if v != "" && v != "C" && v != "POSIX" {
				return v
			}
		}
	}
	return "en-US"
}

func detectTimezone() string {
	if tz := os.Getenv("TZ"); tz != "" {
		return tz
	}
	if location := time.Now().Location(); location != nil {
		if name := strings.TrimSpace(location.String()); name != "" && name != "Local" {
			return name
		}
	}
	return "America/New_York"
}

func StripToDigits(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func FormatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	sec := int(d.Seconds()) % 60
	if h > 0 {
		return fmt.Sprintf("%dh %dm %ds", h, m, sec)
	}
	if m > 0 {
		return fmt.Sprintf("%dm %ds", m, sec)
	}
	return fmt.Sprintf("%ds", sec)
}
