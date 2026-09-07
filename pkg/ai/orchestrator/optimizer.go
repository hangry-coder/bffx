package orchestrator

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"image"
	_ "image/gif"
	"image/jpeg"
	"image/png"
	"net/http"
)

// MediaAsset holds the processed representation of an image passed to the VLM.
type MediaAsset struct {
	RawData   []byte
	Optimized []byte
	MimeType  string
	SHA256    string
	Width     int
	Height    int
}

// Optimizer performs sizing checks, sniffing, and compression/resizing of VLM input files.
type Optimizer struct{}

func NewOptimizer() *Optimizer {
	return &Optimizer{}
}

// Optimize analyses metadata, hashes, and rescales images to standard constraints.
func (o *Optimizer) Optimize(ctx context.Context, raw []byte, maxWidth int) (*MediaAsset, error) {
	mimeType := http.DetectContentType(raw)
	h := sha256.New()
	h.Write(raw)
	shaHex := hex.EncodeToString(h.Sum(nil))

	asset := &MediaAsset{
		RawData:   raw,
		Optimized: raw,
		MimeType:  mimeType,
		SHA256:    shaHex,
	}

	// Read config dimensions without decoding the whole file
	cfg, format, err := image.DecodeConfig(bytes.NewReader(raw))
	if err == nil {
		asset.Width = cfg.Width
		asset.Height = cfg.Height

		// Scale down if image width exceeds maximum constraint
		if maxWidth > 0 && cfg.Width > maxWidth {
			img, _, err := image.Decode(bytes.NewReader(raw))
			if err == nil {
				resized := resizeImage(img, maxWidth)
				var buf bytes.Buffer
				var encodeErr error
				switch format {
				case "jpeg":
					encodeErr = jpeg.Encode(&buf, resized, &jpeg.Options{Quality: 85})
				case "png":
					encodeErr = png.Encode(&buf, resized)
				default:
					buf.Write(raw)
				}
				if encodeErr == nil {
					asset.Optimized = buf.Bytes()
					asset.Width = maxWidth
					asset.Height = (cfg.Height * maxWidth) / cfg.Width
					if asset.Height <= 0 {
						asset.Height = 1
					}
				}
			}
		}
	}

	return asset, nil
}

// resizeImage scales images using clean nearest-neighbor interpolation.
func resizeImage(img image.Image, targetWidth int) image.Image {
	bounds := img.Bounds()
	srcWidth := bounds.Dx()
	srcHeight := bounds.Dy()
	if srcWidth <= 0 || srcHeight <= 0 || targetWidth >= srcWidth {
		return img
	}

	targetHeight := (srcHeight * targetWidth) / srcWidth
	if targetHeight <= 0 {
		targetHeight = 1
	}

	dst := image.NewRGBA(image.Rect(0, 0, targetWidth, targetHeight))

	// Fast scaling without external imports
	for y := 0; y < targetHeight; y++ {
		for x := 0; x < targetWidth; x++ {
			srcX := (x * srcWidth) / targetWidth
			srcY := (y * srcHeight) / targetHeight
			dst.Set(x, y, img.At(bounds.Min.X+srcX, bounds.Min.Y+srcY))
		}
	}
	return dst
}
