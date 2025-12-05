package slapper

import (
	"fmt"
	"time"

	"github.com/df-mc/dragonfly/server/player"
	"github.com/df-mc/dragonfly/server/player/skin"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/npc"
	"github.com/go-gl/mathgl/mgl64"
	"github.com/sandertv/gophertunnel/minecraft/text"

	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/resources"
	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/srv"
)

// Slapper represents an NPC that displays status information about a Minecraft server.
// It includes configuration, skin, and methods to spawn and update the slapper in the world.
type Slapper struct {
	conf       *Config
	resManager *resources.Manager

	skin      skin.Skin
	animation world.EntityAnimation
	handle    *world.EntityHandle
}

// NewSlapper creates and returns a new Slapper instance with the provided configuration and resource manager.
// It also preloads the skin for the slapper.
func NewSlapper(conf *Config, resManager *resources.Manager) *Slapper {
	s := &Slapper{
		conf:       conf,
		resManager: resManager,

		animation: world.NewEntityAnimation(fmt.Sprintf("animation.npc_%s.idle", conf.Identifier)),
	}
	s.preloadSkin()

	return s
}

// preloadSkin loads the skin texture and model from file paths based on the slapper's configuration.
func (s *Slapper) preloadSkin() {
	texturePath, err := s.resManager.FindFileInPack(
		"pokebedrock-hub-res",
		"textures",
		"entity",
		"npcs",
		s.conf.Identifier+".png",
	)
	if err != nil {
		panic(fmt.Sprintf("slapper %s missing texture: %v", s.conf.Identifier, err))
	}

	geometryPath, err := s.resManager.FindFileInPack(
		"pokebedrock-hub-res",
		"models",
		"entity",
		"npcs",
		s.conf.Identifier+".geo.json",
	)
	if err != nil {
		panic(fmt.Sprintf("slapper %s missing geometry: %v", s.conf.Identifier, err))
	}

	s.skin = npc.MustSkin(
		npc.MustParseTexture(texturePath),
		npc.MustParseModel(geometryPath),
	)
}

// Spawn creates the slapper NPC in the world with its configured properties and assigns an interaction handler.
func (s *Slapper) Spawn(tx *world.Tx) {
	n := npc.Create(
		npc.Settings{
			Name: text.Colourf("%s", s.conf.Name),

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

// update refreshes the slapper's name tag based on the server's status.
// It displays the server's online status and player count.
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

// SendAnimation ...
func (s *Slapper) SendAnimation(p *player.Player) {
	e, exists := s.handle.Entity(p.Tx())
	if !exists {
		return
	}
	sess := player_session(p)
	if sess == nil {
		return
	}
	time.AfterFunc(time.Second, func() {
		sess.ViewEntityAnimation(e, s.animation)
	})
}

// Server retrieves the server associated with the slapper based on its configured server identifier.
func (s *Slapper) Server() *srv.Server {
	return srv.FromIdentifier(s.conf.Identifier)
}

// Skin returns the skin of the Slapper.
func (s *Slapper) Skin() skin.Skin {
	return s.skin
}

// Handle returns the Entity Handle of the Slapper.
func (s *Slapper) Handle() *world.EntityHandle {
	return s.handle
}
