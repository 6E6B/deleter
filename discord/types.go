package discord

import "fmt"

type User struct {
	ID         string `json:"id"`
	Username   string `json:"username"`
	GlobalName string `json:"global_name"`
}

type Channel struct {
	ID            string `json:"id"`
	Type          int    `json:"type"`
	Name          string `json:"name"`
	LastMessageID string `json:"last_message_id"`
	Recipients    []User `json:"recipients"`
}

type Message struct {
	ID        string `json:"id"`
	Content   string `json:"content"`
	Author    User   `json:"author"`
	Timestamp string `json:"timestamp"`
}

type LoginResponse struct {
	UserID             string         `json:"user_id"`
	Token              string         `json:"token,omitempty"`
	UserSettings       *LoginSettings `json:"user_settings,omitempty"`
	RequiredActions    []string       `json:"required_actions,omitempty"`
	Ticket             string         `json:"ticket,omitempty"`
	LoginInstanceID    string         `json:"login_instance_id,omitempty"`
	MFA                bool           `json:"mfa,omitempty"`
	TOTP               bool           `json:"totp,omitempty"`
	SMS                bool           `json:"sms,omitempty"`
	Backup             bool           `json:"backup,omitempty"`
	WebAuthn           *string        `json:"webauthn"`
	SuspendedUserToken string         `json:"suspended_user_token,omitempty"`
	CaptchaKey         []string       `json:"captcha_key,omitempty"`
	CaptchaService     string         `json:"captcha_service,omitempty"`
	CaptchaSitekey     string         `json:"captcha_sitekey,omitempty"`
}

type LoginSettings struct {
	Locale string `json:"locale"`
	Theme  string `json:"theme"`
}

type MFAVerifyResponse struct {
	Token              string         `json:"token"`
	UserSettings       *LoginSettings `json:"user_settings,omitempty"`
	SuspendedUserToken string         `json:"suspended_user_token,omitempty"`
}

type SMSSendResponse struct {
	Phone string `json:"phone"`
}

type DiscordError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *DiscordError) Error() string {
	return fmt.Sprintf("Discord error %d: %s", e.Code, e.Message)
}

type superProperties struct {
	OS                       string  `json:"os"`
	Browser                  string  `json:"browser"`
	ReleaseChannel           string  `json:"release_channel"`
	ClientVersion            string  `json:"client_version"`
	OSVersion                string  `json:"os_version"`
	OSArch                   string  `json:"os_arch"`
	AppArch                  string  `json:"app_arch"`
	SystemLocale             string  `json:"system_locale"`
	HasClientMods            bool    `json:"has_client_mods"`
	ClientLaunchID           string  `json:"client_launch_id"`
	LaunchSignature          string  `json:"launch_signature"`
	BrowserUserAgent         string  `json:"browser_user_agent"`
	BrowserVersion           string  `json:"browser_version"`
	WindowManager            string  `json:"window_manager,omitempty"`
	Distro                   string  `json:"distro,omitempty"`
	ClientBuildNumber        int     `json:"client_build_number"`
	NativeBuildNumber        *int    `json:"native_build_number"`
	ClientEventSource        *string `json:"client_event_source"`
	ClientHeartbeatSessionID string  `json:"client_heartbeat_session_id"`
}
