package slapper

import (
	"fmt"
	"log/slog"
	"path/filepath"

	"github.com/df-mc/dragonfly/server/player"
	"github.com/df-mc/dragonfly/server/player/skin"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/npc"
	"github.com/go-gl/mathgl/mgl64"
	"github.com/sandertv/gophertunnel/minecraft/text"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/resources"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/srv"
)

// Slapper ...
type Slapper struct {
	log        *slog.Logger
	conf       *Config
	resManager *resources.Manager
	skin       skin.Skin
	handle     *world.EntityHandle
}

// NewSlapper ...
func NewSlapper(log *slog.Logger, conf *Config, resManager *resources.Manager) *Slapper {
	s := &Slapper{
		log:        log,
		conf:       conf,
		resManager: resManager,
	}
	s.preloadSkin()
	return s
}

// preloadSkin ...
func (s *Slapper) preloadSkin() {
	// Convert paths to full filesystem paths
	unpackedPath := s.resManager.UnpackedPath()
	texturePath := filepath.Join(unpackedPath, "textures", "entity", "hub_npcs", s.conf.ServerIdentifier) + ".png"
	geometryPath := filepath.Join(unpackedPath, "models", "entity", "hub_npcs", s.conf.ServerIdentifier) + ".geo.json"

	s.skin = npc.MustSkin(
		npc.MustParseTexture(texturePath),
		npc.MustParseModel(geometryPath),
	)
}

// Spawn ...
func (s *Slapper) Spawn(tx *world.Tx) {
	n := npc.Create(
		npc.Settings{
			Name: text.Colourf(s.conf.Name),

			Scale: s.conf.Scale,
			Yaw:   s.conf.Yaw,
			Pitch: s.conf.Pitch,

			Position: mgl64.Vec3{
				s.conf.Position.X,
				s.conf.Position.Y,
				s.conf.Position.Z,
			},

			Skin: s.skin,

			Immobile:   true,
			Vulnerable: false,
		}, tx, s.handleInteract,
	)

	s.handle = n.H()
}

// update ...
func (s *Slapper) update(tx *world.Tx) {
	ent, ok := s.handle.Entity(tx)
	if !ok {
		return
	}

	p := ent.(*player.Player)
	st := s.Server().Status()

	var status string
	if st.Online {
		status = text.Colourf(
			"<white>Status:</white> <green>Online</green> <grey>|</grey> <white>%d/%d</white>",
			st.PlayerCount, st.MaxPlayerCount,
		)
	} else {
		status = text.Colourf("<white>Status:</white> <red>Offline</red>")
	}

	p.SetNameTag(fmt.Sprintf("%s\n%s", s.conf.Name, status))
}

// Server ...
func (s *Slapper) Server() *srv.Server {
	return srv.FromIdentifier(s.conf.ServerIdentifier)
}
