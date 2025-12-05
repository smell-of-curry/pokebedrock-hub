package parkour

import (
	"fmt"

	"github.com/df-mc/dragonfly/server/player"
	"github.com/df-mc/dragonfly/server/player/form"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/sandertv/gophertunnel/minecraft/text"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/session"
)

// startForm ...
func (m *Manager) startForm(p *player.Player, cfg CourseConfig) {
	best := m.lb.best(cfg.Identifier, p.XUID())
	p.SendForm(form.NewModal(startForm{
		Yes: form.YesButton(), No: form.NoButton(),
		manager: m, course: cfg,
	}, text.Colourf("<green>%s</green>", cfg.Name)).
		WithBody(fmt.Sprintf("Start %s?\nYour best: %s", cfg.Name, formatDuration(best))))
}

// startForm ...
type startForm struct {
	Yes form.Button
	No  form.Button

	manager *Manager
	course  CourseConfig
}

// Submit ...
func (s startForm) Submit(sub form.Submitter, b form.Button, _ *world.Tx) {
	if b != form.YesButton() {
		return
	}
	p := sub.(*player.Player)

	var rankName string
	if handler, exists := p.Handler().(interface{ Ranks() *session.Ranks }); exists {
		rankName = handler.Ranks().HighestRank().Name()
	}
	_ = s.manager.StartCourse(p, s.course.Identifier, rankName)
}
