package qrcode

import (
	"fmt"
	"strings"

	qrcode "github.com/skip2/go-qrcode"
)

const pngSize = 512

func GeneratePNG(config string) ([]byte, error) {
	if strings.TrimSpace(config) == "" {
		return nil, fmt.Errorf("config text is required")
	}

	png, err := qrcode.Encode(config, qrcode.Medium, pngSize)
	if err != nil {
		return nil, fmt.Errorf("encode qr png: %w", err)
	}

	return png, nil
}
