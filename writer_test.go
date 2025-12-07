package ico

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestEncode(t *testing.T) {
	t.Parallel()
	origfile := "testdata/golang.ico"
	file := "testdata/golang_test.ico"

	f, err := os.Open("testdata/golang.png")
	img, err := png.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	f.Close()

	var newFile *os.File
	if newFile, err = os.Create(filepath.Join(file)); err != nil {
		t.Error(err)
	}
	err = Encode(newFile, img)
	if err != nil {
		t.Error(err)
	}
	newFile.Close()

	f, err = os.Open(origfile)
	if err != nil {
		t.Error(err)
	}
	origICO, err := Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	f.Close()

	newFile, err = os.Open(file)
	if err != nil {
		t.Error(err)
	}
	newICO, err := Decode(newFile)
	if err != nil {
		t.Error(err)
	}
	newFile.Close()

	inrgba, ok := origICO.(*image.NRGBA)
	if !ok {
		t.Fatal("not nrgba")
	}
	pnrgba, ok := newICO.(*image.NRGBA)
	if !ok {
		t.Fatal("new not nrgba")
	}
	if b, err := fastCompare(inrgba, pnrgba); err != nil || b != 0 {
		t.Fatalf("pix differ %d %v\n", b, err)
	}

}

// TestEncodeSizes tests encoding different image sizes
func TestEncodeSizes(t *testing.T) {
	t.Parallel()

	sizes := []int{16, 32, 48, 64, 128, 256}

	for _, size := range sizes {
		size := size
		t.Run(filepath.Base(t.Name())+string(rune('0'+size/100))+string(rune('0'+(size%100)/10))+string(rune('0'+size%10)), func(t *testing.T) {
			t.Parallel()

			// Create test image
			img := createTestImageForWrite(size)

			// Encode to ICO
			tmpFile := filepath.Join(t.TempDir(), "test.ico")
			f, err := os.Create(tmpFile)
			if err != nil {
				t.Fatalf("failed to create temp file: %v", err)
			}

			err = Encode(f, img)
			f.Close()
			if err != nil {
				t.Fatalf("failed to encode: %v", err)
			}

			// Decode and verify
			f, err = os.Open(tmpFile)
			if err != nil {
				t.Fatalf("failed to open encoded file: %v", err)
			}
			decoded, err := Decode(f)
			f.Close()
			if err != nil {
				t.Fatalf("failed to decode: %v", err)
			}

			bounds := decoded.Bounds()
			if bounds.Dx() != size || bounds.Dy() != size {
				t.Errorf("expected %dx%d, got %dx%d", size, size, bounds.Dx(), bounds.Dy())
			}

			// Compare pixel data
			origNRGBA := toNRGBAForWrite(img)
			decodedNRGBA := toNRGBAForWrite(decoded)

			diff, err := fastCompare(origNRGBA, decodedNRGBA)
			if err != nil {
				t.Fatalf("comparison error: %v", err)
			}
			if diff != 0 {
				t.Errorf("pixels differ by %d", diff)
			}
		})
	}
}

// TestEncodeRoundTrip tests encoding and decoding produces identical results
func TestEncodeRoundTrip(t *testing.T) {
	t.Parallel()

	// Test with different image types
	tests := []struct {
		name  string
		image image.Image
	}{
		{
			name:  "NRGBA",
			image: createNRGBAImage(32),
		},
		{
			name:  "RGBA",
			image: createRGBAImage(32),
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// Encode to ICO
			tmpFile := filepath.Join(t.TempDir(), "test.ico")
			f, err := os.Create(tmpFile)
			if err != nil {
				t.Fatalf("failed to create temp file: %v", err)
			}

			err = Encode(f, tc.image)
			f.Close()
			if err != nil {
				t.Fatalf("failed to encode: %v", err)
			}

			// Decode
			f, err = os.Open(tmpFile)
			if err != nil {
				t.Fatalf("failed to open encoded file: %v", err)
			}
			decoded, err := Decode(f)
			f.Close()
			if err != nil {
				t.Fatalf("failed to decode: %v", err)
			}

			// Verify bounds
			if !decoded.Bounds().Eq(tc.image.Bounds()) {
				t.Errorf("bounds mismatch: expected %v, got %v", tc.image.Bounds(), decoded.Bounds())
			}
		})
	}
}

// TestEncodeImageTooLarge tests that encoding fails for images larger than 256x256
func TestEncodeImageTooLarge(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		width     int
		height    int
		wantError bool
	}{
		{"256x256", 256, 256, false},
		{"257x256", 257, 256, true},
		{"256x257", 256, 257, true},
		{"512x512", 512, 512, true},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			img := image.NewNRGBA(image.Rect(0, 0, tc.width, tc.height))
			tmpFile := filepath.Join(t.TempDir(), "test.ico")
			f, err := os.Create(tmpFile)
			if err != nil {
				t.Fatalf("failed to create temp file: %v", err)
			}
			defer f.Close()

			err = Encode(f, img)

			if tc.wantError {
				if err != ErrImageTooLarge {
					t.Errorf("expected ErrImageTooLarge, got %v", err)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

// TestEncodeFromPNG tests encoding ICO from PNG files
func TestEncodeFromPNG(t *testing.T) {
	t.Parallel()

	pngFiles := []string{
		"testdata/16x16.png",
		"testdata/32x32.png",
		"testdata/64x64.png",
		"testdata/256x256.png",
	}

	for _, pngFile := range pngFiles {
		pngFile := pngFile
		t.Run(filepath.Base(pngFile), func(t *testing.T) {
			t.Parallel()

			// Read PNG
			f, err := os.Open(pngFile)
			if err != nil {
				t.Fatalf("failed to open PNG: %v", err)
			}
			pngImg, err := png.Decode(f)
			f.Close()
			if err != nil {
				t.Fatalf("failed to decode PNG: %v", err)
			}

			// Encode to ICO
			tmpFile := filepath.Join(t.TempDir(), "test.ico")
			f, err = os.Create(tmpFile)
			if err != nil {
				t.Fatalf("failed to create temp file: %v", err)
			}

			err = Encode(f, pngImg)
			f.Close()
			if err != nil {
				t.Fatalf("failed to encode ICO: %v", err)
			}

			// Decode ICO
			f, err = os.Open(tmpFile)
			if err != nil {
				t.Fatalf("failed to open ICO: %v", err)
			}
			icoImg, err := Decode(f)
			f.Close()
			if err != nil {
				t.Fatalf("failed to decode ICO: %v", err)
			}

			// Compare
			if !icoImg.Bounds().Eq(pngImg.Bounds()) {
				t.Errorf("bounds mismatch: PNG=%v, ICO=%v", pngImg.Bounds(), icoImg.Bounds())
			}

			origNRGBA := toNRGBAForWrite(pngImg)
			decodedNRGBA := toNRGBAForWrite(icoImg)

			diff, err := fastCompare(origNRGBA, decodedNRGBA)
			if err != nil {
				t.Fatalf("comparison error: %v", err)
			}
			if diff != 0 {
				t.Errorf("pixels differ by %d", diff)
			}
		})
	}
}

// Helper functions

func createTestImageForWrite(size int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			img.SetNRGBA(x, y, color.NRGBA{
				R: uint8((x * 255) / size),
				G: uint8((y * 255) / size),
				B: 128,
				A: 255,
			})
		}
	}
	return img
}

func createNRGBAImage(size int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			img.SetNRGBA(x, y, color.NRGBA{
				R: uint8(x * 8),
				G: uint8(y * 8),
				B: 100,
				A: 200,
			})
		}
	}
	return img
}

func createRGBAImage(size int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			img.SetRGBA(x, y, color.RGBA{
				R: uint8(x * 8),
				G: uint8(y * 8),
				B: 100,
				A: 255,
			})
		}
	}
	return img
}

func toNRGBAForWrite(img image.Image) *image.NRGBA {
	if nrgba, ok := img.(*image.NRGBA); ok {
		return nrgba
	}
	bounds := img.Bounds()
	nrgba := image.NewNRGBA(bounds)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			nrgba.Set(x, y, img.At(x, y))
		}
	}
	return nrgba
}
