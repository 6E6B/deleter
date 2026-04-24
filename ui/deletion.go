package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	g "github.com/AllenDang/giu"

	"deleter/discord"
)

func (a *App) deletingUI() []g.Widget {
	a.mu.Lock()
	deleted := a.deleted
	elapsed := a.elapsed
	rate := a.rate
	errMsg := a.deleteErr
	logs := make([]string, len(a.logs))
	copy(logs, a.logs)
	a.mu.Unlock()

	a.logText = strings.Join(logs, "\n")

	statsLine := fmt.Sprintf("Deleted: %d  |  Elapsed: %s", deleted, discord.FormatDuration(elapsed))
	if rate != "" {
		statsLine += "  |  " + rate
	}

	widgets := []g.Widget{
		g.Label("Deleting Messages"),
		g.Separator(),
		g.Dummy(0, 3),
		g.Label(statsLine),
		g.Dummy(0, 3),
		g.InputTextMultiline(&a.logText).
			Label("##deletionlog").
			Size(-1, -40).
			Flags(g.InputTextFlagsReadOnly).
			AutoScrollToBottom(true),
		g.Button("Stop").Size(100, 30).OnClick(a.onStopDelete),
	}

	if errMsg != "" {
		widgets = append(widgets, errorLabel(errMsg))
	}

	return widgets
}

func (a *App) onStopDelete() {
	a.mu.Lock()
	if a.deleteCancel != nil {
		a.deleteCancel()
	}
	a.mu.Unlock()
}

func (a *App) runDeletion(ctx context.Context) {
	beforeID := ""

	a.mu.Lock()
	channelID := a.channelID
	userID := a.user.ID
	a.mu.Unlock()

	for {
		select {
		case <-ctx.Done():
			a.finishDeletion("Stopped.")
			return
		default:
		}

		msgs, err := a.session.GetMessages(channelID, beforeID, 100)
		if err != nil {
			a.finishDeletion(fmt.Sprintf("Fetch error: %v", err))
			return
		}

		if len(msgs) == 0 {
			a.finishDeletion("All messages deleted.")
			return
		}

		for _, msg := range msgs {
			select {
			case <-ctx.Done():
				a.finishDeletion("Stopped.")
				return
			default:
			}

			beforeID = msg.ID
			if msg.Author.ID != userID {
				continue
			}

			preview := msg.Content
			if len(preview) > 50 {
				preview = preview[:50] + "..."
			}
			ts := msg.Timestamp
			if len(ts) > 19 {
				ts = ts[:19]
			}

			err := a.session.DeleteMessage(channelID, msg.ID)

			a.mu.Lock()
			if err == nil {
				a.deleted++
			}
			elapsed := time.Since(a.startTime)
			a.elapsed = elapsed
			if a.deleted > 0 {
				a.rate = formatDeletionRate(a.deleted, elapsed)
			}
			logLine := fmt.Sprintf("[%d] %s | %s", a.deleted, ts, preview)
			if err != nil {
				logLine += fmt.Sprintf(" FAIL: %v", err)
			}
			a.logs = append(a.logs, logLine)
			if len(a.logs) > maxDeletionLogs {
				a.logs = a.logs[len(a.logs)-maxDeletionLogs:]
			}
			a.mu.Unlock()
			g.Update()
		}
	}
}

func (a *App) finishDeletion(reason string) {
	a.mu.Lock()
	a.deleting = false
	a.deleteCancel = nil
	a.elapsed = time.Since(a.startTime)
	a.state = stateDone
	a.doneMsg = fmt.Sprintf("%s Deleted %d messages in %s.",
		reason, a.deleted, discord.FormatDuration(a.elapsed))
	a.mu.Unlock()
	g.Update()
}

func formatDeletionRate(deleted int, elapsed time.Duration) string {
	if deleted <= 0 || elapsed <= 0 {
		return ""
	}

	rate := float64(deleted) / elapsed.Seconds()
	return fmt.Sprintf("Rate: %.1f msg/s", rate)
}
