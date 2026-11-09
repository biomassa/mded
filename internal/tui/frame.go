package tui

import (
	"time"

	tea "charm.land/bubbletea/v2"
)

// FPS is the frame rate of the renderer. The renderer wakes FPS times each second, also when
// nothing changes, so a lower rate gives less idle load.
const FPS = 30

// frameTime is the time of one frame.
const frameTime = time.Second / FPS

// frameMsg asks for the frame of the wheel events that came after the last frame.
type frameMsg struct{}

// holdWheel handles a wheel event that comes less than one frame after the last built
// frame. The event changes the scroll position, but the model does not lay out and View
// gives the last frame again. A frameMsg at the end of the frame time builds one frame for
// all these events. bubbletea calls View for each message, and the terminal shows at most
// FPS frames each second, so the frames between are work that nobody sees. It reports
// whether it handled the event.
func (m *Model) holdWheel(msg tea.Msg) (tea.Cmd, bool) {
	w, ok := msg.(tea.MouseWheelMsg)
	if !ok || m.noHold || m.vsel.drag || m.built.IsZero() {
		return nil, false
	}
	since := time.Since(m.built)
	if since >= frameTime {
		return nil, false
	}
	m.mouseWheel(w.Mouse())
	m.held = true
	cmds := m.cmds
	m.cmds = nil
	if !m.frameDue {
		m.frameDue = true
		cmds = append(cmds, tea.Tick(frameTime-since, func(time.Time) tea.Msg { return frameMsg{} }))
	}
	return tea.Batch(cmds...), true
}
