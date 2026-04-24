package ui

import (
	"fmt"
	"strings"

	g "github.com/AllenDang/giu"

	"deleter/discord"
)

func (a *App) getMFAOptions() []mfaOption {
	var opts []mfaOption
	a.mu.Lock()
	if a.mfaHasTOTP {
		opts = append(opts, mfaOption{"TOTP (Authenticator App)", "totp"})
	}
	if a.mfaHasSMS {
		opts = append(opts, mfaOption{"SMS", "sms"})
	}
	if a.mfaHasBackup {
		opts = append(opts, mfaOption{"Backup Code", "backup"})
	}
	a.mu.Unlock()
	return opts
}

func (a *App) mfaUI() []g.Widget {
	a.mu.Lock()
	busy := a.mfaBusy
	errMsg := a.mfaErr
	a.mu.Unlock()

	opts := a.getMFAOptions()

	widgets := []g.Widget{
		g.Label("Multi-Factor Authentication"),
		g.Separator(),
		g.Dummy(0, 5),
	}

	for i, opt := range opts {
		idx := int32(i)
		label := opt.label
		widgets = append(widgets,
			g.RadioButton(label, a.mfaMethod == idx).OnChange(func() {
				a.mfaMethod = idx
			}),
		)
	}

	widgets = append(widgets,
		g.Dummy(0, 5),
		g.Label("Code"),
		g.InputText(&a.mfaCode).Label("##mfacode").Size(-1),
		g.Dummy(0, 5),
	)

	if busy {
		widgets = append(widgets, g.Label("Verifying..."))
	} else {
		buttons := []g.Widget{
			g.Button("Verify").Size(120, 30).OnClick(func() { a.onVerifyMFA(opts) }),
		}
		if len(opts) > int(a.mfaMethod) && opts[a.mfaMethod].kind == "sms" {
			buttons = append(buttons,
				g.Button("Send SMS").Size(120, 30).OnClick(a.onSendSMS),
			)
		}
		widgets = append(widgets, g.Row(buttons...))
	}

	if errMsg != "" {
		widgets = append(widgets, g.Dummy(0, 3), errorLabel(errMsg))
	}

	return widgets
}

func (a *App) onVerifyMFA(opts []mfaOption) {
	a.mu.Lock()
	if a.mfaBusy {
		a.mu.Unlock()
		return
	}
	a.mfaBusy = true
	a.mfaErr = ""
	ticket := a.mfaTicket
	loginInstanceID := a.mfaLoginInstanceID
	method := a.mfaMethod
	code := strings.TrimSpace(a.mfaCode)
	a.mu.Unlock()

	if int(method) >= len(opts) {
		a.mu.Lock()
		a.mfaBusy = false
		a.mfaErr = "No MFA method selected"
		a.mu.Unlock()
		return
	}
	authType := opts[method].kind

	if authType == "totp" || authType == "sms" {
		code = discord.StripToDigits(code)
	}

	go func() {
		resp, err := a.session.VerifyMFA(authType, ticket, loginInstanceID, code)
		if err != nil {
			a.mu.Lock()
			a.mfaBusy = false
			a.mfaErr = err.Error()
			a.mu.Unlock()
			g.Update()
			return
		}

		if resp.SuspendedUserToken != "" {
			a.mu.Lock()
			a.mfaBusy = false
			a.mfaErr = "Account is suspended"
			a.mu.Unlock()
			g.Update()
			return
		}

		if resp.Token == "" {
			a.mu.Lock()
			a.mfaBusy = false
			a.mfaErr = "MFA verification did not return a token"
			a.mu.Unlock()
			g.Update()
			return
		}

		a.session.SetToken(resp.Token)
		user, err := a.session.GetCurrentUser()
		if err != nil {
			a.mu.Lock()
			a.mfaBusy = false
			a.mfaErr = fmt.Sprintf("Token received but user fetch failed: %v", err)
			a.mu.Unlock()
			g.Update()
			return
		}

		a.mu.Lock()
		a.user = user
		a.mfaBusy = false
		a.state = stateTarget
		a.mu.Unlock()
		g.Update()
	}()
}

func (a *App) onSendSMS() {
	a.mu.Lock()
	ticket := a.mfaTicket
	a.mu.Unlock()

	go func() {
		smsResp, err := a.session.SendMFASMS(ticket)
		if err != nil {
			a.mu.Lock()
			a.mfaErr = fmt.Sprintf("Failed to send SMS: %v", err)
			a.mu.Unlock()
			g.Update()
			return
		}
		a.mu.Lock()
		a.mfaErr = fmt.Sprintf("SMS sent to %s", smsResp.Phone)
		a.mu.Unlock()
		g.Update()
	}()
}
