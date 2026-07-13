package npc

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/df-mc/dragonfly/server/player/skin"
)

func TestParseTextureAndModel(t *testing.T) {
	dir := t.TempDir()

	texPath := filepath.Join(dir, "skin.png")
	img := image.NewRGBA(image.Rect(0, 0, 64, 64))
	for y := 0; y < 64; y++ {
		for x := 0; x < 64; x++ {
			img.Set(x, y, color.RGBA{R: 255, A: 255})
		}
	}
	f, err := os.Create(texPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()

	modelPath := filepath.Join(dir, "skin.geo.json")
	modelJSON := `{
  "format_version": "1.12.0",
  "minecraft:geometry": [{
    "description": {
      "identifier": "geometry.test",
      "texture_width": 64,
      "texture_height": 64
    },
    "bones": []
  }]
}`
	if err := os.WriteFile(modelPath, []byte(modelJSON), 0o644); err != nil {
		t.Fatal(err)
	}

	tex, err := ParseTexture(texPath)
	if err != nil {
		t.Fatalf("ParseTexture: %v", err)
	}
	mod, err := ParseModel(modelPath)
	if err != nil {
		t.Fatalf("ParseModel: %v", err)
	}

	sk, err := Skin(tex, mod)
	if err != nil {
		t.Fatalf("Skin: %v", err)
	}
	if sk.ModelConfig.Default != "geometry.test" {
		t.Fatalf("unexpected model id %q", sk.ModelConfig.Default)
	}
	if len(sk.Pix) != 64*64*4 {
		t.Fatalf("unexpected pix len %d", len(sk.Pix))
	}
}

func TestSkinDimensionMismatch(t *testing.T) {
	tex := Texture{pix: make([]byte, 64*32*4), rect: image.Rect(0, 0, 64, 32)}
	mod := Model{conf: skin.ModelConfig{Default: "geometry.test"}, rect: image.Rect(0, 0, 64, 64)}
	if _, err := Skin(tex, mod); err == nil {
		t.Fatal("expected dimension mismatch error")
	}
}
