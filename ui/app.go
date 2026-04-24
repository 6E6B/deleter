package ui

import (
	"context"
	"sync"
	"time"

	g "github.com/AllenDang/giu"

	"deleter/discord"
)

type appState int

const (
	stateLogin appState = iota
	stateMFA
	stateTarget
	stateDeleting
	stateDone
)

const maxDeletionLogs = 1000

type mfaOption struct {
	label string
	kind  string
}

type App struct {
	mu    sync.Mutex
	state appState

	// Login
	email     string
	password  string
	token     string
	useToken  bool
	loginBusy bool
	loginErr  string

	// MFA
	mfaTicket          string
	mfaLoginInstanceID string
	mfaHasTOTP         bool
	mfaHasSMS          bool
	mfaHasBackup       bool
	mfaMethod          int32
	mfaCode            string
	mfaBusy            bool
	mfaErr             string

	// Session
	session *discord.Session
	user    *discord.User

	// Target
	modeDM          bool
	targetID        string
	targetBusy      bool
	targetErr       string
	dmChannels      []discord.Channel
	dmLabels        []string
	selectedDMIndex int32
	dmLoading       bool

	// Deletion
	channelID    string
	deleted      int
	logs         []string
	logText      string
	deleting     bool
	deleteCancel context.CancelFunc
	deleteErr    string
	startTime    time.Time
	elapsed      time.Duration
	rate         string

	// Done
	doneMsg string
}

func Run() {
	app := &App{
		modeDM: true,
	}
	wnd := g.NewMasterWindow("Deleter", 550, 450, 0)
	wnd.Run(app.loop)
}

func (a *App) loop() {
	a.mu.Lock()
	state := a.state
	a.mu.Unlock()

	g.SingleWindow().Layout(a.buildUI(state)...)
}

func (a *App) buildUI(state appState) []g.Widget {
	switch state {
	case stateLogin:
		return a.loginUI()
	case stateMFA:
		return a.mfaUI()
	case stateTarget:
		return a.targetUI()
	case stateDeleting:
		return a.deletingUI()
	case stateDone:
		return a.doneUI()
	}
	return nil
}
