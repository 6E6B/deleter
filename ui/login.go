package ui

import (
	"fmt"
	"strings"

	g "github.com/AllenDang/giu"

	"deleter/discord"
)

func (a *App) loginUI() []g.Widget {
	a.mu.Lock()
	busy := a.loginBusy
	errMsg := a.loginErr
	a.mu.Unlock()

	widgets := []g.Widget{
		g.TabBar().TabItems(
			g.TabItem("Email / Password").Layout(
				g.Dummy(0, 3),
				g.Label("Email or Phone"),
				g.InputText(&a.email).Label("##email").Size(-1),
				g.Dummy(0, 3),
				g.Label("Password"),
				g.InputText(&a.password).Label("##password").Size(-1).Flags(g.InputTextFlagsPassword),
			),
			g.TabItem("Token").Layout(
				g.Dummy(0, 3),
				g.Label("Paste your Discord token"),
				g.InputText(&a.token).Label("##token").Size(-1).Flags(g.InputTextFlagsPassword),
				g.Dummy(0, 3),
				g.Label("Use this if email/password login requires CAPTCHA.").Wrapped(true),
			),
		),
		g.Dummy(0, 5),
	}

	if busy {
		widgets = append(widgets, g.Label("Logging in..."))
	} else {
		widgets = append(widgets,
			g.Button("Login").Size(120, 30).OnClick(a.onLogin),
		)
	}

	if errMsg != "" {
		widgets = append(widgets, g.Dummy(0, 3), errorLabel(errMsg))
	}

	return widgets
}

func (a *App) onLogin() {
	a.mu.Lock()
	email := strings.TrimSpace(a.email)
	password := a.password
	token := strings.TrimSpace(a.token)
	if a.loginBusy {
		a.mu.Unlock()
		return
	}
	a.loginBusy = true
	a.loginErr = ""
	a.mu.Unlock()

	useToken := token != "" && email == "" && password == ""

	go func() {
		if useToken {
			sess := discord.NewSession(token)
			user, err := sess.GetCurrentUser()
			if err != nil {
				a.mu.Lock()
				a.loginBusy = false
				a.loginErr = fmt.Sprintf("Invalid token: %v", err)
				a.mu.Unlock()
				g.Update()
				return
			}
			a.mu.Lock()
			a.session = sess
			a.user = user
			a.loginBusy = false
			a.state = stateTarget
			a.mu.Unlock()
			g.Update()
			return
		}

		if email == "" || password == "" {
			a.mu.Lock()
			a.loginBusy = false
			a.loginErr = "Email and password are required"
			a.mu.Unlock()
			g.Update()
			return
		}

		sess := discord.NewSession("")
		if err := sess.SeedCookies(); err != nil {
			a.mu.Lock()
			a.loginBusy = false
			a.loginErr = fmt.Sprintf("Failed to initialize Discord session: %v", err)
			a.mu.Unlock()
			g.Update()
			return
		}
		if _, err := sess.GetFingerprint(); err != nil {
			a.mu.Lock()
			a.loginBusy = false
			a.loginErr = fmt.Sprintf("Failed to initialize Discord session: %v", err)
			a.mu.Unlock()
			g.Update()
			return
		}

		resp, err := sess.Login(email, password, false)
		if err != nil {
			a.mu.Lock()
			a.loginBusy = false
			a.loginErr = err.Error()
			a.mu.Unlock()
			g.Update()
			return
		}

		if resp.SuspendedUserToken != "" {
			a.mu.Lock()
			a.loginBusy = false
			a.loginErr = "Account is suspended"
			a.mu.Unlock()
			g.Update()
			return
		}

		if resp.Token != "" {
			sess.SetToken(resp.Token)
			user, err := sess.GetCurrentUser()
			if err != nil {
				a.mu.Lock()
				a.loginBusy = false
				a.loginErr = fmt.Sprintf("Login succeeded but verification failed: %v", err)
				a.mu.Unlock()
				g.Update()
				return
			}
			a.mu.Lock()
			a.session = sess
			a.user = user
			a.loginBusy = false
			a.state = stateTarget
			a.mu.Unlock()
			g.Update()
			return
		}

		if resp.MFA {
			a.mu.Lock()
			a.session = sess
			a.mfaTicket = resp.Ticket
			a.mfaLoginInstanceID = resp.LoginInstanceID
			a.mfaHasTOTP = resp.TOTP
			a.mfaHasSMS = resp.SMS
			a.mfaHasBackup = resp.Backup
			a.mfaMethod = 0
			a.mfaCode = ""
			a.mfaErr = ""
			a.loginBusy = false
			a.state = stateMFA
			a.mu.Unlock()
			g.Update()
			return
		}

		a.mu.Lock()
		a.loginBusy = false
		a.loginErr = "Unexpected login response"
		a.mu.Unlock()
		g.Update()
	}()
}
