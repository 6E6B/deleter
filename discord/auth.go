package discord

import (
	"encoding/json"
	"fmt"
)

func (s *Session) GetFingerprint() (string, error) {
	body, code, _, err := s.doRequest("GET", "/experiments")
	if err != nil {
		return "", err
	}
	if code != 200 {
		return "", fmt.Errorf("HTTP %d", code)
	}
	var resp struct {
		Fingerprint string `json:"fingerprint"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", err
	}
	if resp.Fingerprint == "" {
		return "", fmt.Errorf("empty fingerprint")
	}
	s.mu.Lock()
	s.fingerprint = resp.Fingerprint
	s.mu.Unlock()
	return resp.Fingerprint, nil
}

func (s *Session) Login(login, password string, undelete bool) (*LoginResponse, error) {
	payload := map[string]interface{}{
		"login":            login,
		"password":         password,
		"undelete":         undelete,
		"login_source":     nil,
		"gift_code_sku_id": nil,
	}

	body, code, _, err := s.doRequestWithBody("POST", "/auth/login", payload)
	if err != nil {
		return nil, err
	}

	var resp LoginResponse
	if err := json.Unmarshal(body, &resp); err != nil && code == 200 {
		return nil, err
	}

	if DebugMode {
		fmt.Printf("\n    [debug] login HTTP %d: %s\n", code, string(body))
	}

	if code == 403 && resp.SuspendedUserToken != "" {
		return &resp, nil
	}

	if len(resp.CaptchaKey) > 0 {
		return nil, fmt.Errorf("CAPTCHA required (%s) — use a token instead", resp.CaptchaService)
	}

	if code != 200 {
		var discErr DiscordError
		if json.Unmarshal(body, &discErr) == nil && discErr.Code != 0 {
			return nil, &discErr
		}
		return nil, fmt.Errorf("HTTP %d: %s", code, string(body))
	}

	return &resp, nil
}

func (s *Session) VerifyMFA(authenticatorType, ticket, loginInstanceID, code string) (*MFAVerifyResponse, error) {
	payload := map[string]interface{}{
		"code":             code,
		"ticket":           ticket,
		"login_source":     nil,
		"gift_code_sku_id": nil,
	}
	if loginInstanceID != "" {
		payload["login_instance_id"] = loginInstanceID
	}

	endpoint := "/auth/mfa/" + authenticatorType

	if DebugMode {
		raw, _ := json.MarshalIndent(payload, "    ", "  ")
		fmt.Printf("\n    [debug] POST %s\n    %s\n", apiBase+endpoint, raw)
	}

	body, statusCode, _, err := s.doRequestWithBody("POST", endpoint, payload)
	if err != nil {
		return nil, err
	}

	if DebugMode {
		fmt.Printf("    [debug] HTTP %d: %s\n", statusCode, string(body))
	}

	var resp MFAVerifyResponse
	if err := json.Unmarshal(body, &resp); err != nil && statusCode == 200 {
		return nil, err
	}

	if statusCode == 403 && resp.SuspendedUserToken != "" {
		return &resp, nil
	}

	if statusCode != 200 {
		var discErr DiscordError
		if json.Unmarshal(body, &discErr) == nil && discErr.Code != 0 {
			return nil, &discErr
		}
		return nil, fmt.Errorf("HTTP %d: %s", statusCode, string(body))
	}

	return &resp, nil
}

func (s *Session) SendMFASMS(ticket string) (*SMSSendResponse, error) {
	payload := map[string]interface{}{
		"ticket": ticket,
	}

	body, code, _, err := s.doRequestWithBody("POST", "/auth/mfa/sms/send", payload)
	if err != nil {
		return nil, err
	}
	if code != 200 {
		return nil, fmt.Errorf("HTTP %d: %s", code, string(body))
	}

	var resp SMSSendResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (s *Session) AuthorizeIP(verificationToken string) error {
	payload := map[string]interface{}{
		"token": verificationToken,
	}
	body, code, _, err := s.doRequestWithBody("POST", "/auth/authorize-ip", payload)
	if err != nil {
		return err
	}
	if code != 204 {
		return fmt.Errorf("HTTP %d: %s", code, string(body))
	}
	return nil
}
