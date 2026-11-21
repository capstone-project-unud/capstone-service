package httpapi

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"
)

func TestSignatureValidatesActualImage(t *testing.T) {
	for _, data := range [][]byte{[]byte("<svg>not a signature</svg>"), []byte("%PDF-1.4"), make([]byte, 2*1024*1024+1)} {
		if _, e := canonicalSignature(data); e == nil {
			t.Fatal("invalid signature accepted")
		}
	}
	im := image.NewNRGBA(image.Rect(0, 0, 240, 80))
	im.Set(5, 5, color.NRGBA{A: 255})
	var b bytes.Buffer
	_ = png.Encode(&b, im)
	clean, e := canonicalSignature(b.Bytes())
	if e != nil {
		t.Fatal(e)
	}
	cfg, format, e := image.DecodeConfig(bytes.NewReader(clean))
	if e != nil || format != "png" || cfg.Width != 240 || cfg.Height != 80 {
		t.Fatal("canonical signature changed dimensions")
	}
}
