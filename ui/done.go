package ui

import g "github.com/AllenDang/giu"

func (a *App) doneUI() []g.Widget {
	a.mu.Lock()
	msg := a.doneMsg
	a.mu.Unlock()

	return []g.Widget{
		g.Label("Complete"),
		g.Separator(),
		g.Dummy(0, 10),
		g.Label(msg).Wrapped(true),
		g.Dummy(0, 10),
		g.Row(
			g.Button("Delete More").Size(120, 30).OnClick(func() {
				a.mu.Lock()
				a.targetID = ""
				a.targetErr = ""
				a.state = stateTarget
				a.mu.Unlock()
			}),
			g.Button("Logout").Size(100, 30).OnClick(a.onLogout),
		),
	}
}
