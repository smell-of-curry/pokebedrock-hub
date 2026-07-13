package slapper

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/df-mc/dragonfly/server/player"
	"github.com/df-mc/dragonfly/server/player/skin"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/go-gl/mathgl/mgl64"
	"github.com/sandertv/gophertunnel/minecraft/text"

	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/npc"
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

type viewLayerViewer interface {
	ViewLayer() *world.ViewLayer
}

func viewerForLayer(viewers []world.Viewer, layer *world.ViewLayer) world.Viewer {
	if layer == nil {
		return nil
	}
	for _, viewer := range viewers {
		layered, ok := viewer.(viewLayerViewer)
		if ok && layered.ViewLayer() == layer {
			return viewer
		}
	}
	return nil
}

// NewSlapper creates a Slapper and loads its skin assets.
func NewSlapper(conf *Config, resManager *resources.Manager) (*Slapper, error) {
	s := &Slapper{
		conf:       conf,
		resManager: resManager,
	}
	assetIdentifier := conf.Identifier
	if err := s.preloadSkin(assetIdentifier); err != nil {
		if !errors.Is(err, os.ErrNotExist) || conf.Identifier == "black" {
			return nil, err
		}
		slog.Warn("slapper assets missing; using fallback", "identifier", conf.Identifier, "fallback", "black", "error", err)
		assetIdentifier = "black"
		if err = s.preloadSkin(assetIdentifier); err != nil {
			return nil, fmt.Errorf("slapper %s fallback failed: %w", conf.Identifier, err)
		}
	}
	s.animation = world.NewEntityAnimation(fmt.Sprintf("animation.npc_%s.idle", assetIdentifier))

	return s, nil
}

// preloadSkin loads the skin texture and model from file paths based on the slapper's configuration.
func (s *Slapper) preloadSkin(identifier string) error {
	texturePath, err := s.resManager.FindFileInPack(
		"pokebedrock-hub-res",
		"textures",
		"entity",
		"npcs",
		identifier+".png",
	)
	if err != nil {
		return fmt.Errorf("slapper %s missing texture: %w", identifier, err)
	}

	geometryPath, err := s.resManager.FindFileInPack(
		"pokebedrock-hub-res",
		"models",
		"entity",
		"npcs",
		identifier+".geo.json",
	)
	if err != nil {
		return fmt.Errorf("slapper %s missing geometry: %w", identifier, err)
	}

	texture, err := npc.ParseTexture(texturePath)
	if err != nil {
		return fmt.Errorf("slapper %s invalid texture: %w", identifier, err)
	}
	model, err := npc.ParseModel(geometryPath)
	if err != nil {
		return fmt.Errorf("slapper %s invalid geometry: %w", identifier, err)
	}
	s.skin, err = npc.Skin(texture, model)
	if err != nil {
		return fmt.Errorf("slapper %s invalid skin: %w", identifier, err)
	}
	return nil
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

// SendAnimation plays the slapper idle animation for the joining/interacting player after a short delay.
func (s *Slapper) SendAnimation(p *player.Player) {
	viewer := p.H()
	npcHandle := s.handle
	if viewer == nil || npcHandle == nil {
		return
	}

	player.DoAfter(viewer, time.Second, func(tx *world.Tx, p *player.Player) {
		ent, ok := npcHandle.Entity(tx)
		if !ok {
			return
		}
		target := viewerForLayer(tx.Viewers(p.Position()), p.ViewLayer())
		if target != nil {
			target.ViewEntityAnimation(ent, s.animation)
		}
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
