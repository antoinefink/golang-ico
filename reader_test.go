package ico

import (
	"fmt"
	"image"
	"image/png"
	"math"
	"os"
	"strings"
	"testing"
)

func sqDiffUInt8(x, y uint8) uint64 {
	d := uint64(x) - uint64(y)
	return d * d
}

func fastCompare(img1, img2 *image.NRGBA) (int64, error) {
	if img1.Bounds() != img2.Bounds() {
		return 0, fmt.Errorf("image bounds not equal: %+v, %+v", img1.Bounds(), img2.Bounds())
	}

	accumError := int64(0)

	for i := 0; i < len(img1.Pix); i++ {
		accumError += int64(sqDiffUInt8(img1.Pix[i], img2.Pix[i]))
	}

	return int64(math.Sqrt(float64(accumError))), nil
}

// toNRGBA converts an image to NRGBA format for comparison
func toNRGBA(img image.Image) *image.NRGBA {
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

// TestDecode tests the basic Decode function with the original test file
func TestDecode(t *testing.T) {
	t.Parallel()
	file := "testdata/golang.ico"
	copyFile := "testdata/golang.png"
	reader, err := os.Open(file)
	if err != nil {
		t.Fatal(err)
	}
	icoImage, err := Decode(reader)
	if err != nil {
		t.Fatal(err)
	}
	reader.Close()

	reader, err = os.Open(copyFile)
	if err != nil {
		t.Fatal(err)
	}
	pngImage, err := png.Decode(reader)
	if err != nil {
		t.Fatal(err)
	}
	reader.Close()

	if icoImage == nil || !icoImage.Bounds().Eq(pngImage.Bounds()) {
		t.Fatal("bounds differ")
	}
	inrgba, ok := icoImage.(*image.NRGBA)
	if !ok {
		t.Fatal("not nrgba")
	}
	pnrgba, ok := pngImage.(*image.NRGBA)
	if !ok {
		t.Fatal("png not nrgba")
	}

	if b, err := fastCompare(inrgba, pnrgba); err != nil || b != 0 {
		t.Fatalf("pix differ %d %v\n", b, err)
	}
}

// TestDecodeSizes tests decoding ICO files of various sizes
func TestDecodeSizes(t *testing.T) {
	t.Parallel()

	sizes := []struct {
		name   string
		width  int
		height int
	}{
		{"16x16", 16, 16},
		{"32x32", 32, 32},
		{"48x48", 48, 48},
		{"64x64", 64, 64},
		{"128x128", 128, 128},
		{"256x256", 256, 256},
	}

	for _, tc := range sizes {
		tc := tc // capture range variable
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			icoFile := fmt.Sprintf("testdata/%s.ico", tc.name)
			pngFile := fmt.Sprintf("testdata/%s.png", tc.name)

			// Decode ICO
			icoReader, err := os.Open(icoFile)
			if err != nil {
				t.Fatalf("failed to open ICO file: %v", err)
			}
			icoImg, err := Decode(icoReader)
			icoReader.Close()
			if err != nil {
				t.Fatalf("failed to decode ICO: %v", err)
			}

			// Check bounds
			bounds := icoImg.Bounds()
			if bounds.Dx() != tc.width || bounds.Dy() != tc.height {
				t.Errorf("expected %dx%d, got %dx%d", tc.width, tc.height, bounds.Dx(), bounds.Dy())
			}

			// Decode PNG for comparison
			pngReader, err := os.Open(pngFile)
			if err != nil {
				t.Fatalf("failed to open PNG file: %v", err)
			}
			pngImg, err := png.Decode(pngReader)
			pngReader.Close()
			if err != nil {
				t.Fatalf("failed to decode PNG: %v", err)
			}

			// Compare images
			icoNRGBA := toNRGBA(icoImg)
			pngNRGBA := toNRGBA(pngImg)

			diff, err := fastCompare(icoNRGBA, pngNRGBA)
			if err != nil {
				t.Fatalf("comparison error: %v", err)
			}
			if diff != 0 {
				t.Errorf("images differ by %d", diff)
			}
		})
	}
}

// TestDecodeConfig tests the DecodeConfig function
func TestDecodeConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		ico    string
		png    string
		width  int
		height int
	}{
		{"16x16", "testdata/16x16.ico", "testdata/16x16.png", 16, 16},
		{"32x32", "testdata/32x32.ico", "testdata/32x32.png", 32, 32},
		{"64x64", "testdata/64x64.ico", "testdata/64x64.png", 64, 64},
		{"256x256", "testdata/256x256.ico", "testdata/256x256.png", 256, 256},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// Get ICO config
			icoReader, err := os.Open(tc.ico)
			if err != nil {
				t.Fatalf("failed to open ICO file: %v", err)
			}
			icoConfig, err := DecodeConfig(icoReader)
			icoReader.Close()
			if err != nil {
				t.Fatalf("failed to decode ICO config: %v", err)
			}

			// Get PNG config for comparison
			pngReader, err := os.Open(tc.png)
			if err != nil {
				t.Fatalf("failed to open PNG file: %v", err)
			}
			pngConfig, err := png.DecodeConfig(pngReader)
			pngReader.Close()
			if err != nil {
				t.Fatalf("failed to decode PNG config: %v", err)
			}

			// Compare dimensions
			if icoConfig.Width != pngConfig.Width {
				t.Errorf("width mismatch: ICO=%d, PNG=%d", icoConfig.Width, pngConfig.Width)
			}
			if icoConfig.Height != pngConfig.Height {
				t.Errorf("height mismatch: ICO=%d, PNG=%d", icoConfig.Height, pngConfig.Height)
			}

			// Also verify against expected values
			if icoConfig.Width != tc.width || icoConfig.Height != tc.height {
				t.Errorf("expected %dx%d, got %dx%d", tc.width, tc.height, icoConfig.Width, icoConfig.Height)
			}
		})
	}
}

// TestDecodeAll tests decoding multi-image ICO files
func TestDecodeAll(t *testing.T) {
	t.Parallel()

	reader, err := os.Open("testdata/multi_sizes.ico")
	if err != nil {
		t.Fatalf("failed to open multi_sizes.ico: %v", err)
	}
	defer reader.Close()

	images, err := DecodeAll(reader)
	if err != nil {
		t.Fatalf("failed to decode multi_sizes.ico: %v", err)
	}

	expectedSizes := []struct {
		width  int
		height int
	}{
		{16, 16},
		{32, 32},
		{48, 48},
		{256, 256},
	}

	if len(images) != len(expectedSizes) {
		t.Fatalf("expected %d images, got %d", len(expectedSizes), len(images))
	}

	for i, expected := range expectedSizes {
		bounds := images[i].Bounds()
		if bounds.Dx() != expected.width || bounds.Dy() != expected.height {
			t.Errorf("image %d: expected %dx%d, got %dx%d",
				i, expected.width, expected.height, bounds.Dx(), bounds.Dy())
		}

		// Verify against corresponding PNG
		pngFile := fmt.Sprintf("testdata/multi_%dx%d.png", expected.width, expected.height)
		pngReader, err := os.Open(pngFile)
		if err != nil {
			t.Logf("skipping PNG comparison for %s: %v", pngFile, err)
			continue
		}
		pngImg, err := png.Decode(pngReader)
		pngReader.Close()
		if err != nil {
			t.Logf("skipping PNG comparison for %s: %v", pngFile, err)
			continue
		}

		icoNRGBA := toNRGBA(images[i])
		pngNRGBA := toNRGBA(pngImg)

		diff, err := fastCompare(icoNRGBA, pngNRGBA)
		if err != nil {
			t.Errorf("image %d comparison error: %v", i, err)
			continue
		}
		if diff != 0 {
			t.Errorf("image %d: pixels differ by %d", i, diff)
		}
	}
}

// TestDecodeErrors tests error handling for invalid ICO files
func TestDecodeErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		file        string
		expectError string
	}{
		{
			name:        "empty ICO",
			file:        "testdata/empty.ico",
			expectError: "no images",
		},
		{
			name:        "corrupt header",
			file:        "testdata/corrupt_header.ico",
			expectError: "corrupted head",
		},
		{
			name:        "truncated file",
			file:        "testdata/truncated.ico",
			expectError: "EOF",
		},
		{
			name:        "invalid size",
			file:        "testdata/invalid_size.ico",
			expectError: "corrupted entry",
		},
		{
			name:        "bad offset",
			file:        "testdata/bad_offset.ico",
			expectError: "EOF",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			reader, err := os.Open(tc.file)
			if err != nil {
				t.Fatalf("failed to open test file: %v", err)
			}
			defer reader.Close()

			_, err = Decode(reader)
			if err == nil {
				t.Fatal("expected error, got nil")
			}

			if !strings.Contains(err.Error(), tc.expectError) {
				t.Errorf("expected error containing %q, got %q", tc.expectError, err.Error())
			}
		})
	}
}

// TestDecodeBMPFormat tests decoding ICO files with BMP internal format
func TestDecodeBMPFormat(t *testing.T) {
	t.Parallel()

	reader, err := os.Open("testdata/bmp_format.ico")
	if err != nil {
		t.Fatalf("failed to open bmp_format.ico: %v", err)
	}
	defer reader.Close()

	img, err := Decode(reader)
	if err != nil {
		t.Fatalf("failed to decode bmp_format.ico: %v", err)
	}

	bounds := img.Bounds()
	if bounds.Dx() != 32 || bounds.Dy() != 32 {
		t.Errorf("expected 32x32, got %dx%d", bounds.Dx(), bounds.Dy())
	}

	// Verify against PNG
	pngReader, err := os.Open("testdata/bmp_format.png")
	if err != nil {
		t.Fatalf("failed to open bmp_format.png: %v", err)
	}
	defer pngReader.Close()

	pngImg, err := png.Decode(pngReader)
	if err != nil {
		t.Fatalf("failed to decode bmp_format.png: %v", err)
	}

	icoNRGBA := toNRGBA(img)
	pngNRGBA := toNRGBA(pngImg)

	diff, err := fastCompare(icoNRGBA, pngNRGBA)
	if err != nil {
		t.Fatalf("comparison error: %v", err)
	}
	if diff != 0 {
		t.Errorf("images differ by %d", diff)
	}
}

// TestDecodeBitDepths tests decoding ICO files with various bit depths
func TestDecodeBitDepths(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		file   string
		width  int
		height int
	}{
		{"24-bit", "testdata/24bit.ico", 32, 32},
		{"8-bit", "testdata/8bit.ico", 32, 32},
		{"4-bit", "testdata/4bit.ico", 32, 32},
		{"1-bit", "testdata/1bit.ico", 32, 32},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			reader, err := os.Open(tc.file)
			if err != nil {
				t.Fatalf("failed to open %s: %v", tc.file, err)
			}
			defer reader.Close()

			img, err := Decode(reader)
			if err != nil {
				t.Fatalf("failed to decode %s: %v", tc.file, err)
			}

			bounds := img.Bounds()
			if bounds.Dx() != tc.width || bounds.Dy() != tc.height {
				t.Errorf("expected %dx%d, got %dx%d", tc.width, tc.height, bounds.Dx(), bounds.Dy())
			}
		})
	}
}

// TestDecodePNGFormat tests decoding ICO files with PNG internal format
func TestDecodePNGFormat(t *testing.T) {
	t.Parallel()

	// All our generated size variant ICOs use PNG format
	sizes := []int{16, 32, 64, 256}

	for _, size := range sizes {
		size := size
		t.Run(fmt.Sprintf("%dx%d", size, size), func(t *testing.T) {
			t.Parallel()

			icoFile := fmt.Sprintf("testdata/%dx%d.ico", size, size)
			reader, err := os.Open(icoFile)
			if err != nil {
				t.Fatalf("failed to open %s: %v", icoFile, err)
			}
			defer reader.Close()

			img, err := Decode(reader)
			if err != nil {
				t.Fatalf("failed to decode %s: %v", icoFile, err)
			}

			bounds := img.Bounds()
			if bounds.Dx() != size || bounds.Dy() != size {
				t.Errorf("expected %dx%d, got %dx%d", size, size, bounds.Dx(), bounds.Dy())
			}
		})
	}
}

// TestDecodeConfigErrors tests DecodeConfig error handling
func TestDecodeConfigErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		file        string
		expectError string
	}{
		{
			name:        "empty ICO",
			file:        "testdata/empty.ico",
			expectError: "no images",
		},
		{
			name:        "corrupt header",
			file:        "testdata/corrupt_header.ico",
			expectError: "corrupted head",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			reader, err := os.Open(tc.file)
			if err != nil {
				t.Fatalf("failed to open test file: %v", err)
			}
			defer reader.Close()

			_, err = DecodeConfig(reader)
			if err == nil {
				t.Fatal("expected error, got nil")
			}

			if !strings.Contains(err.Error(), tc.expectError) {
				t.Errorf("expected error containing %q, got %q", tc.expectError, err.Error())
			}
		})
	}
}
