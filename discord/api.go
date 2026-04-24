package discord

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
)

func (s *Session) GetCurrentUser() (*User, error) {
	body, code, _, err := s.doRequest("GET", "/users/@me")
	if err != nil {
		return nil, err
	}
	if code != 200 {
		return nil, fmt.Errorf("HTTP %d: %s", code, string(body))
	}
	var user User
	return &user, json.Unmarshal(body, &user)
}

func (s *Session) GetPrivateChannels() ([]Channel, error) {
	body, code, _, err := s.doRequest("GET", "/users/@me/channels")
	if err != nil {
		return nil, err
	}
	if code != 200 {
		return nil, fmt.Errorf("HTTP %d: %s", code, string(body))
	}

	var channels []Channel
	if err := json.Unmarshal(body, &channels); err != nil {
		return nil, err
	}

	privateChannels := channels[:0]
	for _, ch := range channels {
		if ch.Type == 1 || ch.Type == 3 {
			privateChannels = append(privateChannels, ch)
		}
	}

	return privateChannels, nil
}

func (s *Session) FindDMChannel(targetUserID string) (string, error) {
	channels, err := s.GetPrivateChannels()
	if err != nil {
		return "", err
	}

	for _, ch := range channels {
		if ch.Type == 1 {
			for _, r := range ch.Recipients {
				if r.ID == targetUserID {
					return ch.ID, nil
				}
			}
		}
	}

	return "", fmt.Errorf("no DM channel found with user %s", targetUserID)
}

func (s *Session) GetMessages(channelID, beforeID string, limit int) ([]Message, error) {
	params := url.Values{}
	params.Set("limit", strconv.Itoa(limit))
	if beforeID != "" {
		params.Set("before", beforeID)
	}

	endpoint := fmt.Sprintf("/channels/%s/messages?%s", channelID, params.Encode())

	for {
		body, code, _, err := s.doRequest("GET", endpoint)
		if err != nil {
			return nil, err
		}

		if code == 429 {
			continue
		}

		if code != 200 {
			return nil, fmt.Errorf("HTTP %d: %s", code, string(body))
		}

		var msgs []Message
		return msgs, json.Unmarshal(body, &msgs)
	}
}

func (s *Session) DeleteMessage(channelID, messageID string) error {
	endpoint := fmt.Sprintf("/channels/%s/messages/%s", channelID, messageID)

	for {
		body, code, _, err := s.doRequest("DELETE", endpoint)
		if err != nil {
			return err
		}

		if code == 429 {
			continue
		}

		if code != 204 {
			return fmt.Errorf("HTTP %d: %s", code, string(body))
		}

		return nil
	}
}
