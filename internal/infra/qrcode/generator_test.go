package qrcode

import (
	"bytes"
	"testing"
)

func TestGeneratePNG(t *testing.T) {
	png, err := GeneratePNG("[Interface]\nPrivateKey = private-key\n")
	if err != nil {
		t.Fatalf("GeneratePNG() error = %v", err)
	}

	if len(png) == 0 {
		t.Fatal("GeneratePNG() returned empty png")
	}

	if !bytes.HasPrefix(png, []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}) {
		t.Fatalf("GeneratePNG() did not return a PNG")
	}
}

func TestGeneratePNGRequiresConfig(t *testing.T) {
	png, err := GeneratePNG("   ")
	if err == nil {
		t.Fatal("GeneratePNG() error = nil, want non-nil")
	}

	if png != nil {
		t.Fatalf("GeneratePNG() png = %v, want nil", png)
	}
}
