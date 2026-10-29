package organizations

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"math"
	"strings"
	"testing"
)

func TestDefaultThemeAndNormalization(t *testing.T) {
	theme := DefaultTheme()
	theme.PrimaryColor = " #00C8E8 "
	theme.Sponsors = []Sponsor{{Name: " Sponsor ", URL: " https://example.org/sponsor "}}
	theme.KitColors = []string{" #FFFFFF "}
	if err := validateTheme(&theme); err != nil {
		t.Fatal(err)
	}
	if theme.PrimaryColor != "#00c8e8" || theme.Sponsors[0].Name != "Sponsor" || theme.KitColors[0] != "#ffffff" {
		t.Fatal("theme was not normalized")
	}
	if math.Abs(contrast("#ffffff", "#000000")-21) > 0.001 {
		t.Fatal("incorrect WCAG contrast")
	}
	if math.Abs(contrast("#888888", "#888888")-1) > 0.001 {
		t.Fatal("identical colors contrast must equal one")
	}
	empty := DefaultTheme()
	empty.Sponsors = nil
	empty.KitColors = nil
	if err := validateTheme(&empty); err != nil {
		t.Fatal(err)
	}
	if empty.Sponsors == nil || empty.KitColors == nil {
		t.Fatal("arrays must serialize as arrays")
	}
}

func TestThemeRejectsInvalidColorsContrastAndMetadata(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*Theme)
	}{
		{"short hex", func(t *Theme) { t.PrimaryColor = "#abc" }},
		{"CSS function", func(t *Theme) { t.AccentColor = "rgb(0,0,0)" }},
		{"low contrast", func(t *Theme) { t.TextColor = t.SurfaceColor }},
		{"invalid kit", func(t *Theme) { t.KitColors = []string{"red"} }},
		{"too many kits", func(t *Theme) {
			t.KitColors = []string{"#000000", "#000000", "#000000", "#000000", "#000000", "#000000", "#000000"}
		}},
		{"missing sponsor name", func(t *Theme) { t.Sponsors = []Sponsor{{Name: " "}} }},
		{"long sponsor", func(t *Theme) { t.Sponsors = []Sponsor{{Name: strings.Repeat("a", 121)}} }},
		{"many sponsors", func(t *Theme) { t.Sponsors = make([]Sponsor, 21) }},
		{"unsafe sponsor URL", func(t *Theme) { t.Sponsors = []Sponsor{{Name: "Example", URL: "javascript:alert(1)"}} }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			theme := DefaultTheme()
			tc.change(&theme)
			if err := validateTheme(&theme); err == nil {
				t.Fatal("unsafe theme accepted")
			}
		})
	}
	if err := validateTheme(nil); err == nil {
		t.Fatal("nil theme accepted")
	}
}

func TestThemeAcceptsHTTPSAndEmbeddedRasterImages(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.White)
	var pngBuffer, jpegBuffer bytes.Buffer
	if err := png.Encode(&pngBuffer, img); err != nil {
		t.Fatal(err)
	}
	if err := jpeg.Encode(&jpegBuffer, img, nil); err != nil {
		t.Fatal(err)
	}
	// Minimal lossless header with declared dimensions, used to verify RIFF framing.
	webp := make([]byte, 26)
	copy(webp, "RIFF")
	binary.LittleEndian.PutUint32(webp[4:8], 18)
	copy(webp[8:], "WEBPVP8L")
	binary.LittleEndian.PutUint32(webp[16:20], 5)
	webp[20] = 0x2f
	for _, tc := range []struct{ name, url string }{
		{"remote", "https://assets.example.org/logo.png?version=1"},
		{"png", "data:image/png;base64," + base64.StdEncoding.EncodeToString(pngBuffer.Bytes())},
		{"jpeg", "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(jpegBuffer.Bytes())},
		{"webp framing", "data:image/webp;base64," + base64.StdEncoding.EncodeToString(webp)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			theme := DefaultTheme()
			theme.LogoURL = tc.url
			if err := validateTheme(&theme); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestThemeRejectsUnsafeOrMalformedImages(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	var pngBuffer bytes.Buffer
	if err := png.Encode(&pngBuffer, img); err != nil {
		t.Fatal(err)
	}
	for name, value := range map[string]string{
		"http":                "http://assets.example.org/logo.png",
		"protocol relative":   "//example.org/logo.png",
		"script":              "javascript:alert(1)",
		"credentials":         "https://user:password@example.org/logo.png",
		"missing host":        "https:///logo.png",
		"overlong remote":     "https://example.org/" + strings.Repeat("a", 2048),
		"svg":                 "data:image/svg+xml;base64," + base64.StdEncoding.EncodeToString([]byte(`<svg/>`)),
		"html":                "data:text/html;base64," + base64.StdEncoding.EncodeToString([]byte(`<script/>`)),
		"bad base64":          "data:image/png;base64,!!!",
		"empty":               "data:image/png;base64,",
		"missing comma":       "data:image/png;base64",
		"counterfeit png":     "data:image/png;base64," + base64.StdEncoding.EncodeToString([]byte("not a PNG")),
		"mime mismatch":       "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(pngBuffer.Bytes()),
		"oversized":           "data:image/png;base64," + base64.StdEncoding.EncodeToString(make([]byte, maxBrandingImageBytes+1)),
		"fake webp":           "data:image/webp;base64," + base64.StdEncoding.EncodeToString(make([]byte, 32)),
		"embedded whitespace": "data:image/png;base64,\n" + base64.StdEncoding.EncodeToString(pngBuffer.Bytes()),
	} {
		t.Run(name, func(t *testing.T) {
			theme := DefaultTheme()
			theme.CoverURL = value
			if err := validateTheme(&theme); err == nil {
				t.Fatal("unsafe image accepted")
			}
		})
	}
}
