package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	g "github.com/AllenDang/giu"

	"deleter/discord"
)

func (a *App) targetUI() []g.Widget {
	a.mu.Lock()
	busy := a.targetBusy
	errMsg := a.targetErr
	username := a.user.Username
	userID := a.user.ID
	modeDM := a.modeDM
	dmLoading := a.dmLoading
	dmLabels := append([]string(nil), a.dmLabels...)
	selectedDMIndex := a.selectedDMIndex
	a.mu.Unlock()

	widgets := []g.Widget{
		g.Labelf("Logged in as %s (%s)", username, userID),
		g.Separator(),
		g.Dummy(0, 5),
		g.Label("Mode"),
		g.RadioButton("DM Channel", modeDM).OnChange(func() {
			a.modeDM = true
			a.ensureDMChannelsLoaded()
		}),
		g.RadioButton("Channel ID", !modeDM).OnChange(func() {
			a.modeDM = false
		}),
		g.Dummy(0, 5),
	}

	if modeDM {
		a.ensureDMChannelsLoaded()
		widgets = append(widgets,
			g.Label("DM Channel"),
		)
		if dmLoading {
			widgets = append(widgets, g.Label("Loading DM channels..."))
		} else if len(dmLabels) == 0 {
			widgets = append(widgets, g.Label("No DM channels found."))
		} else {
			widgets = append(widgets,
				g.ListBox(dmLabels).
					SelectedIndex(&a.selectedDMIndex).
					Size(-1, 220).
					OnDClick(func(selectedIndex int) {
						a.mu.Lock()
						if selectedIndex >= 0 && selectedIndex < len(a.dmChannels) {
							a.selectedDMIndex = int32(selectedIndex)
							a.mu.Unlock()
							a.onStartDelete()
							return
						}
						a.mu.Unlock()
					}),
			)
		}
		widgets = append(widgets,
			g.Dummy(0, 5),
			g.Row(
				g.Button("Refresh DMs").Size(120, 30).OnClick(a.loadDMChannels).Disabled(dmLoading || busy),
				g.Labelf("Selected: %s", selectedDMLabel(dmLabels, selectedDMIndex)),
			),
			g.Dummy(0, 5),
		)
	} else {
		widgets = append(widgets,
			g.Label("Channel ID"),
			g.InputText(&a.targetID).Label("##targetid").Size(-1),
			g.Dummy(0, 5),
		)
	}

	if busy {
		widgets = append(widgets, g.Label("Resolving..."))
	} else {
		startDisabled := modeDM && (dmLoading || len(dmLabels) == 0 || selectedDMIndex < 0 || int(selectedDMIndex) >= len(dmLabels))
		widgets = append(widgets, g.Row(
			g.Button("Start Deleting").Size(140, 30).OnClick(a.onStartDelete).Disabled(startDisabled),
			g.Button("Logout").Size(100, 30).OnClick(a.onLogout),
		))
	}

	if errMsg != "" {
		widgets = append(widgets, g.Dummy(0, 3), errorLabel(errMsg))
	}

	return widgets
}

func selectedDMLabel(labels []string, selected int32) string {
	if selected < 0 || int(selected) >= len(labels) {
		return "none"
	}
	return labels[selected]
}

func (a *App) ensureDMChannelsLoaded() {
	a.mu.Lock()
	shouldLoad := a.modeDM && !a.dmLoading && len(a.dmChannels) == 0 && a.targetErr == ""
	a.mu.Unlock()
	if shouldLoad {
		a.loadDMChannels()
	}
}

func (a *App) loadDMChannels() {
	a.mu.Lock()
	if a.dmLoading || a.session == nil {
		a.mu.Unlock()
		return
	}
	a.dmLoading = true
	a.targetErr = ""
	a.mu.Unlock()

	go func() {
		channels, err := a.session.GetPrivateChannels()
		labels := make([]string, 0, len(channels))
		for _, ch := range channels {
			labels = append(labels, formatDMChannelLabel(ch))
		}

		a.mu.Lock()
		a.dmLoading = false
		if err != nil {
			a.targetErr = fmt.Sprintf("Failed to load DM channels: %v", err)
			a.dmChannels = nil
			a.dmLabels = nil
			a.selectedDMIndex = 0
			a.mu.Unlock()
			g.Update()
			return
		}

		a.dmChannels = channels
		a.dmLabels = labels
		if len(channels) == 0 {
			a.selectedDMIndex = 0
		} else if a.selectedDMIndex < 0 || int(a.selectedDMIndex) >= len(channels) {
			a.selectedDMIndex = 0
		}
		a.mu.Unlock()
		g.Update()
	}()
}

func formatDMChannelLabel(ch discord.Channel) string {
	switch ch.Type {
	case 1:
		if len(ch.Recipients) == 0 {
			return fmt.Sprintf("DM %s", ch.ID)
		}
		user := ch.Recipients[0]
		name := displayUserName(user)
		if name == "" {
			name = user.ID
		}
		return fmt.Sprintf("%s (%s)", name, user.ID)
	case 3:
		if ch.Name != "" {
			return fmt.Sprintf("%s (Group DM, %s)", ch.Name, ch.ID)
		}
		names := make([]string, 0, len(ch.Recipients))
		for _, user := range ch.Recipients {
			name := displayUserName(user)
			if name == "" {
				name = user.ID
			}
			names = append(names, name)
		}
		if len(names) == 0 {
			return fmt.Sprintf("Group DM %s", ch.ID)
		}
		return fmt.Sprintf("%s (Group DM, %s)", strings.Join(names, ", "), ch.ID)
	default:
		return ch.ID
	}
}

func displayUserName(user discord.User) string {
	if user.GlobalName != "" {
		return user.GlobalName
	}
	return user.Username
}

func (a *App) onLogout() {
	a.mu.Lock()
	a.session = nil
	a.user = nil
	a.email = ""
	a.password = ""
	a.token = ""
	a.loginErr = ""
	a.targetID = ""
	a.targetErr = ""
	a.dmChannels = nil
	a.dmLabels = nil
	a.selectedDMIndex = 0
	a.dmLoading = false
	a.state = stateLogin
	a.mu.Unlock()
}

func (a *App) onStartDelete() {
	a.mu.Lock()
	if a.targetBusy {
		a.mu.Unlock()
		return
	}
	isDM := a.modeDM
	targetID := strings.TrimSpace(a.targetID)
	channelID := targetID
	if isDM {
		selectedIndex := int(a.selectedDMIndex)
		if a.dmLoading {
			a.targetErr = "DM channels are still loading"
			a.mu.Unlock()
			return
		}
		if selectedIndex < 0 || selectedIndex >= len(a.dmChannels) {
			a.targetErr = "Select a DM channel"
			a.mu.Unlock()
			return
		}
		channelID = a.dmChannels[selectedIndex].ID
	} else if targetID == "" {
		a.targetErr = "Channel ID is required"
		a.mu.Unlock()
		return
	}

	a.targetBusy = true
	a.targetErr = ""
	a.mu.Unlock()

	go func() {
		a.session.SetReferer("https://discord.com/channels/@me/" + channelID)

		ctx, cancel := context.WithCancel(context.Background())

		a.mu.Lock()
		a.channelID = channelID
		a.deleted = 0
		a.logs = nil
		a.logText = ""
		a.deleteErr = ""
		a.deleting = true
		a.deleteCancel = cancel
		a.startTime = time.Now()
		a.elapsed = 0
		a.rate = ""
		a.targetBusy = false
		a.state = stateDeleting
		a.mu.Unlock()
		g.Update()

		go func() {
			ticker := time.NewTicker(time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					a.mu.Lock()
					a.elapsed = time.Since(a.startTime)
					a.rate = formatDeletionRate(a.deleted, a.elapsed)
					a.mu.Unlock()
					g.Update()
				}
			}
		}()

		a.runDeletion(ctx)
	}()
}
