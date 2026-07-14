package npc

import (
	"encoding/json"
	"fmt"
	"image"
	_ "image/png"
	"os"

	"github.com/df-mc/dragonfly/server/player/skin"
)

// Model is geometry JSON plus texture bounds from a geo file.
type Model struct {
	json []byte
	conf skin.ModelConfig
	rect image.Rectangle
}

// Texture is raw RGBA skin pixels and bounds.
type Texture struct {
	pix  []byte
	rect image.Rectangle
}

// Skin creates a skin.Skin from Texture and Model.
//
// @param tex Parsed skin texture.
// @param mod Parsed geometry model.
// @returns the assembled skin.
// @throws if texture dimensions do not match the model.
func Skin(tex Texture, mod Model) (skin.Skin, error) {
	if tex.rect != mod.rect {
		return skin.Skin{}, fmt.Errorf("skin texture dimensions did not match those specified in model: %v specified but got %v", mod.rect, tex.rect)
	}
	s := skin.New(tex.rect.Dx(), tex.rect.Dy())
	s.ModelConfig = mod.conf
	s.Model = mod.json
	s.Pix = tex.pix
	return s, nil
}

// ParseModel parses a Model from a JSON geometry file.
//
// @param path Path to the geometry JSON file.
// @returns the parsed model.
// @throws if the file cannot be opened or parsed.
func ParseModel(path string) (Model, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Model{}, fmt.Errorf("failed opening model file: %w", err)
	}

	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return Model{}, fmt.Errorf("failed decoding model: %w", err)
	}

	geometries, ok := raw["minecraft:geometry"].([]any)
	if !ok || len(geometries) == 0 {
		return Model{}, fmt.Errorf("model missing minecraft:geometry")
	}
	geo, ok := geometries[0].(map[string]any)
	if !ok {
		return Model{}, fmt.Errorf("invalid geometry entry")
	}
	desc, ok := geo["description"].(map[string]any)
	if !ok {
		return Model{}, fmt.Errorf("geometry missing description")
	}

	w, okW := desc["texture_width"].(float64)
	h, okH := desc["texture_height"].(float64)
	id, okID := desc["identifier"].(string)
	if !okW || !okH || !okID {
		return Model{}, fmt.Errorf("geometry description missing texture size or identifier")
	}

	return Model{
		json: data,
		conf: skin.ModelConfig{Default: id},
		rect: image.Rect(0, 0, int(w), int(h)),
	}, nil
}

// ParseTexture parses a Texture from an image file.
//
// @param path Path to the PNG texture.
// @returns the parsed texture.
// @throws if the file cannot be opened or is an invalid skin size.
func ParseTexture(path string) (Texture, error) {
	f, err := os.Open(path)
	if err != nil {
		return Texture{}, fmt.Errorf("failed opening texture file: %w", err)
	}
	defer f.Close()

	img, _, err := image.Decode(f)
	if err != nil {
		return Texture{}, fmt.Errorf("failed decoding texture: %w", err)
	}

	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	if !(w == 64 && h == 32) && !(w == 64 && h == 64) && !(w == 128 && h == 128) {
		return Texture{}, fmt.Errorf("invalid skin texture dimensions: %vx%v", w, h)
	}

	pix := make([]byte, 0, w*h*4)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			r, g, b, a := img.At(x, y).RGBA()
			pix = append(pix, byte(r>>8), byte(g>>8), byte(b>>8), byte(a>>8))
		}
	}
	return Texture{pix: pix, rect: img.Bounds()}, nil
}
