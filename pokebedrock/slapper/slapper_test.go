package slapper

import (
	"image"
	"image/png"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/df-mc/dragonfly/server/world"

	"github.com/smell-of-curry/pokebedrock-hub/pokebedrock/resources"
)

type testLayerViewer struct {
	world.NopViewer
	layer *world.ViewLayer
}

func (v *testLayerViewer) ViewLayer() *world.ViewLayer {
	return v.layer
}

func TestViewerForLayer(t *testing.T) {
	targetLayer := new(world.ViewLayer)
	target := &testLayerViewer{layer: targetLayer}
	other := &testLayerViewer{layer: new(world.ViewLayer)}

	if got := viewerForLayer([]world.Viewer{other, target}, targetLayer); got != target {
		t.Fatalf("expected target viewer, got %T", got)
	}
}

func TestNewSlapperReturnsMissingAssetError(t *testing.T) {
	manager := resources.NewManager(slog.New(slog.NewTextHandler(io.Discard, nil)), t.TempDir())

	_, err := NewSlapper(&Config{Identifier: "missing"}, manager)
	if err == nil || !strings.Contains(err.Error(), "missing texture") {
		t.Fatalf("expected missing texture error, got %v", err)
	}
}

func TestNewSlapperFallsBackToBlackAssets(t *testing.T) {
	resourceDir := t.TempDir()
	textureDir := filepath.Join(resourceDir, "unpacked", "textures", "entity", "npcs")
	modelDir := filepath.Join(resourceDir, "unpacked", "models", "entity", "npcs")
	if err := os.MkdirAll(textureDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(modelDir, 0o755); err != nil {
		t.Fatal(err)
	}

	texture, err := os.Create(filepath.Join(textureDir, "black.png"))
	if err != nil {
		t.Fatal(err)
	}
	if err = png.Encode(texture, image.NewRGBA(image.Rect(0, 0, 64, 64))); err != nil {
		t.Fatal(err)
	}
	if err = texture.Close(); err != nil {
		t.Fatal(err)
	}

	model := `{"minecraft:geometry":[{"description":{"identifier":"geometry.npc_black","texture_width":64,"texture_height":64}}]}`
	if err = os.WriteFile(filepath.Join(modelDir, "black.geo.json"), []byte(model), 0o644); err != nil {
		t.Fatal(err)
	}

	manager := resources.NewManager(slog.New(slog.NewTextHandler(io.Discard, nil)), resourceDir)
	slapper, err := NewSlapper(&Config{Identifier: "diamond"}, manager)
	if err != nil {
		t.Fatal(err)
	}
	if slapper.skin.ModelConfig.Default != "geometry.npc_black" {
		t.Fatalf("expected black fallback model, got %q", slapper.skin.ModelConfig.Default)
	}
	if name := slapper.animation.Name(); name != "animation.npc_black.idle" {
		t.Fatalf("expected black fallback animation, got %q", name)
	}
}
