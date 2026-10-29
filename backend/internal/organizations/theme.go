package organizations

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"math"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

const maxBrandingImageBytes = 200 * 1024

var hexColor = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

type Sponsor struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

type Theme struct {
	PrimaryColor   string    `json:"primaryColor"`
	SecondaryColor string    `json:"secondaryColor"`
	AccentColor    string    `json:"accentColor"`
	SurfaceColor   string    `json:"surfaceColor"`
	TextColor      string    `json:"textColor"`
	LogoURL        string    `json:"logoUrl"`
	CoverURL       string    `json:"coverUrl"`
	Sponsors       []Sponsor `json:"sponsors"`
	KitColors      []string  `json:"kitColors"`
}

func DefaultTheme() Theme {
	return Theme{PrimaryColor: "#00c8e8", SecondaryColor: "#07172b", AccentColor: "#8bf000", SurfaceColor: "#07172b", TextColor: "#ffffff", Sponsors: []Sponsor{}, KitColors: []string{}}
}

func validateTheme(theme *Theme) error {
	if theme == nil {
		return errors.New("theme is required")
	}
	for name, value := range map[string]*string{"primaryColor": &theme.PrimaryColor, "secondaryColor": &theme.SecondaryColor, "accentColor": &theme.AccentColor, "surfaceColor": &theme.SurfaceColor, "textColor": &theme.TextColor} {
		*value = strings.TrimSpace(*value)
		if !hexColor.MatchString(*value) {
			return fmt.Errorf("%s must be a six-digit hex color", name)
		}
		*value = strings.ToLower(*value)
	}
	if contrast(theme.TextColor, theme.SurfaceColor) < 4.5 {
		return errors.New("textColor and surfaceColor must have contrast of at least 4.5:1")
	}
	theme.LogoURL = strings.TrimSpace(theme.LogoURL)
	theme.CoverURL = strings.TrimSpace(theme.CoverURL)
	if err := validateImageURL(theme.LogoURL); err != nil {
		return fmt.Errorf("logoUrl: %w", err)
	}
	if err := validateImageURL(theme.CoverURL); err != nil {
		return fmt.Errorf("coverUrl: %w", err)
	}
	if len(theme.Sponsors) > 20 {
		return errors.New("at most 20 sponsors are allowed")
	}
	if theme.Sponsors == nil {
		theme.Sponsors = []Sponsor{}
	}
	for i := range theme.Sponsors {
		sponsor := &theme.Sponsors[i]
		sponsor.Name = strings.TrimSpace(sponsor.Name)
		sponsor.URL = strings.TrimSpace(sponsor.URL)
		if utf8.RuneCountInString(sponsor.Name) < 1 || utf8.RuneCountInString(sponsor.Name) > 120 {
			return errors.New("sponsor name must contain 1 to 120 characters")
		}
		if sponsor.URL != "" {
			if err := validateHTTPS(sponsor.URL); err != nil {
				return fmt.Errorf("sponsor URL: %w", err)
			}
		}
	}
	if len(theme.KitColors) > 6 {
		return errors.New("at most six kit colors are allowed")
	}
	if theme.KitColors == nil {
		theme.KitColors = []string{}
	}
	for i, c := range theme.KitColors {
		c = strings.TrimSpace(c)
		if !hexColor.MatchString(c) {
			return errors.New("kit colors must be six-digit hex colors")
		}
		theme.KitColors[i] = strings.ToLower(c)
	}
	return nil
}

func validateHTTPS(value string) error {
	if len(value) > 2048 {
		return errors.New("HTTPS URL is too long")
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" || parsed.User != nil || parsed.Opaque != "" || strings.ContainsAny(value, "\r\n\t") {
		return errors.New("use an HTTPS URL without embedded credentials")
	}
	return nil
}

func validateImageURL(value string) error {
	if value == "" {
		return nil
	}
	if !strings.HasPrefix(value, "data:") {
		return validateHTTPS(value)
	}
	if len(value) > base64.StdEncoding.EncodedLen(maxBrandingImageBytes)+32 {
		return errors.New("image must not exceed 200 KiB")
	}
	header, encoded, ok := strings.Cut(value, ",")
	if !ok {
		return errors.New("invalid image data URL")
	}
	var format string
	switch header {
	case "data:image/png;base64":
		format = "png"
	case "data:image/jpeg;base64":
		format = "jpeg"
	case "data:image/webp;base64":
		format = "webp"
	default:
		return errors.New("only PNG, JPEG and WebP images are allowed")
	}
	// Strict prevents noncanonical pad bits; line breaks are rejected explicitly.
	if strings.ContainsAny(encoded, "\r\n\t ") {
		return errors.New("invalid image base64")
	}
	data, err := base64.StdEncoding.Strict().DecodeString(encoded)
	if err != nil || len(data) == 0 {
		return errors.New("invalid image base64")
	}
	if len(data) > maxBrandingImageBytes {
		return errors.New("image must not exceed 200 KiB")
	}
	if format == "webp" {
		return validateWebP(data)
	}
	cfg, detected, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || detected != format {
		return errors.New("image content does not match its declared format")
	}
	if cfg.Width < 1 || cfg.Height < 1 || int64(cfg.Width)*int64(cfg.Height) > 16_000_000 {
		return errors.New("image dimensions must not exceed 16 megapixels")
	}
	return nil
}

// The standard library has no WebP decoder. Validate its RIFF framing, first
// image chunk and dimensions without adding a decoder dependency or executing it.
func validateWebP(data []byte) error {
	invalid := errors.New("invalid or unsupported WebP image")
	if len(data) < 25 || string(data[:4]) != "RIFF" || string(data[8:12]) != "WEBP" || uint64(binary.LittleEndian.Uint32(data[4:8]))+8 != uint64(len(data)) {
		return invalid
	}
	foundImage := false
	for offset := 12; offset < len(data); {
		if len(data)-offset < 8 {
			return invalid
		}
		chunk := string(data[offset : offset+4])
		length := int64(binary.LittleEndian.Uint32(data[offset+4 : offset+8]))
		padded := length + (length % 2)
		if padded > int64(len(data)-offset-8) {
			return invalid
		}
		payload := data[offset+8 : offset+8+int(length)]
		var width, height int64
		switch chunk {
		case "VP8 ":
			if foundImage || len(payload) < 10 || payload[0]&1 != 0 || !bytes.Equal(payload[3:6], []byte{0x9d, 0x01, 0x2a}) {
				return invalid
			}
			width = int64(binary.LittleEndian.Uint16(payload[6:8]) & 0x3fff)
			height = int64(binary.LittleEndian.Uint16(payload[8:10]) & 0x3fff)
			foundImage = true
		case "VP8L":
			if foundImage || len(payload) < 5 || payload[0] != 0x2f {
				return invalid
			}
			bits := binary.LittleEndian.Uint32(payload[1:5])
			if bits>>29 != 0 {
				return invalid
			}
			width = int64(bits&0x3fff) + 1
			height = int64((bits>>14)&0x3fff) + 1
			foundImage = true
		case "VP8X":
			if offset != 12 || len(payload) != 10 || payload[0]&0x02 != 0 {
				return invalid
			}
			width = (int64(payload[4]) | int64(payload[5])<<8 | int64(payload[6])<<16) + 1
			height = (int64(payload[7]) | int64(payload[8])<<8 | int64(payload[9])<<16) + 1
		case "ANIM", "ANMF":
			return invalid
		}
		if (width != 0 || height != 0) && (width < 1 || height < 1 || width*height > 16_000_000) {
			return errors.New("image dimensions must not exceed 16 megapixels")
		}
		offset += 8 + int(padded)
	}
	if !foundImage {
		return invalid
	}
	return nil
}

func luminance(color string) float64 {
	var components [3]float64
	for i := 0; i < 3; i++ {
		n, _ := strconv.ParseUint(color[1+i*2:3+i*2], 16, 8)
		c := float64(n) / 255
		if c <= 0.04045 {
			components[i] = c / 12.92
		} else {
			components[i] = math.Pow((c+0.055)/1.055, 2.4)
		}
	}
	return 0.2126*components[0] + 0.7152*components[1] + 0.0722*components[2]
}

func contrast(a, b string) float64 {
	l1, l2 := luminance(a), luminance(b)
	if l1 < l2 {
		l1, l2 = l2, l1
	}
	return (l1 + 0.05) / (l2 + 0.05)
}
