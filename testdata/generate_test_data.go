//go:build ignore

// Run with: go run testdata/generate_test_data.go
package main

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
)

// ICO file structures
type head struct {
	Zero   uint16
	Type   uint16
	Number uint16
}

type direntry struct {
	Width   byte
	Height  byte
	Palette byte
	_       byte
	Plane   uint16
	Bits    uint16
	Size    uint32
	Offset  uint32
}

func main() {
	testdataDir := "testdata"

	// Generate size variants (PNG format)
	sizes := []int{16, 32, 48, 64, 128, 256}
	for _, size := range sizes {
		generateSizeVariant(testdataDir, size)
	}

	// Generate multi-image ICO
	generateMultiImageICO(testdataDir)

	// Generate BMP-format ICO files
	generateBMPFormatICO(testdataDir)

	// Generate different bit depth ICO files (BMP format)
	generateBitDepthVariants(testdataDir)

	// Generate edge cases
	generateEdgeCases(testdataDir)

	fmt.Println("Test data generation complete!")
}

// createTestImage creates a simple colored square image for testing
func createTestImage(size int, c color.NRGBA) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			// Create a gradient pattern for visual verification
			r := uint8((x * 255) / size)
			g := uint8((y * 255) / size)
			img.SetNRGBA(x, y, color.NRGBA{
				R: (c.R + r) / 2,
				G: (c.G + g) / 2,
				B: c.B,
				A: c.A,
			})
		}
	}
	return img
}

func generateSizeVariant(dir string, size int) {
	// Create test image
	c := color.NRGBA{R: 100, G: 150, B: 200, A: 255}
	img := createTestImage(size, c)

	// Save PNG for verification
	pngPath := filepath.Join(dir, fmt.Sprintf("%dx%d.png", size, size))
	pngFile, err := os.Create(pngPath)
	if err != nil {
		fmt.Printf("Error creating %s: %v\n", pngPath, err)
		return
	}
	if err := png.Encode(pngFile, img); err != nil {
		fmt.Printf("Error encoding %s: %v\n", pngPath, err)
		pngFile.Close()
		return
	}
	pngFile.Close()

	// Create ICO file
	icoPath := filepath.Join(dir, fmt.Sprintf("%dx%d.ico", size, size))
	icoFile, err := os.Create(icoPath)
	if err != nil {
		fmt.Printf("Error creating %s: %v\n", icoPath, err)
		return
	}
	if err := encodeSingleICO(icoFile, img); err != nil {
		fmt.Printf("Error encoding %s: %v\n", icoPath, err)
		icoFile.Close()
		return
	}
	icoFile.Close()

	fmt.Printf("Generated %dx%d test files\n", size, size)
}

func encodeSingleICO(w *os.File, img image.Image) error {
	b := img.Bounds()
	width := b.Dx()
	height := b.Dy()

	// Encode image as PNG
	pngBuf := new(bytes.Buffer)
	pngWriter := bufio.NewWriter(pngBuf)
	if err := png.Encode(pngWriter, img); err != nil {
		return err
	}
	if err := pngWriter.Flush(); err != nil {
		return err
	}

	header := head{
		Zero:   0,
		Type:   1,
		Number: 1,
	}

	// For 256x256, width/height should be 0 in the directory entry
	entryWidth := uint8(width)
	entryHeight := uint8(height)
	if width == 256 {
		entryWidth = 0
	}
	if height == 256 {
		entryHeight = 0
	}

	entry := direntry{
		Width:   entryWidth,
		Height:  entryHeight,
		Palette: 0,
		Plane:   1,
		Bits:    32,
		Size:    uint32(pngBuf.Len()),
		Offset:  22, // 6 (header) + 16 (one entry)
	}

	// Write header
	if err := binary.Write(w, binary.LittleEndian, header); err != nil {
		return err
	}
	// Write entry
	if err := binary.Write(w, binary.LittleEndian, entry); err != nil {
		return err
	}
	// Write PNG data
	if _, err := w.Write(pngBuf.Bytes()); err != nil {
		return err
	}

	return nil
}

func generateMultiImageICO(dir string) {
	sizes := []int{16, 32, 48, 256}
	images := make([]*image.NRGBA, len(sizes))
	pngDatas := make([][]byte, len(sizes))

	// Create images and encode as PNG
	for i, size := range sizes {
		c := color.NRGBA{
			R: uint8(50 + i*50),
			G: uint8(100 + i*30),
			B: uint8(150 + i*20),
			A: 255,
		}
		images[i] = createTestImage(size, c)

		buf := new(bytes.Buffer)
		writer := bufio.NewWriter(buf)
		if err := png.Encode(writer, images[i]); err != nil {
			fmt.Printf("Error encoding multi-image PNG: %v\n", err)
			return
		}
		writer.Flush()
		pngDatas[i] = buf.Bytes()
	}

	// Calculate offsets
	headerSize := 6
	entrySize := 16
	dataOffset := headerSize + (entrySize * len(sizes))

	// Create ICO file
	icoPath := filepath.Join(dir, "multi_sizes.ico")
	icoFile, err := os.Create(icoPath)
	if err != nil {
		fmt.Printf("Error creating %s: %v\n", icoPath, err)
		return
	}
	defer icoFile.Close()

	// Write header
	header := head{
		Zero:   0,
		Type:   1,
		Number: uint16(len(sizes)),
	}
	if err := binary.Write(icoFile, binary.LittleEndian, header); err != nil {
		fmt.Printf("Error writing header: %v\n", err)
		return
	}

	// Write directory entries
	currentOffset := uint32(dataOffset)
	for i, size := range sizes {
		entryWidth := uint8(size)
		entryHeight := uint8(size)
		if size == 256 {
			entryWidth = 0
			entryHeight = 0
		}

		entry := direntry{
			Width:   entryWidth,
			Height:  entryHeight,
			Palette: 0,
			Plane:   1,
			Bits:    32,
			Size:    uint32(len(pngDatas[i])),
			Offset:  currentOffset,
		}
		if err := binary.Write(icoFile, binary.LittleEndian, entry); err != nil {
			fmt.Printf("Error writing entry: %v\n", err)
			return
		}
		currentOffset += uint32(len(pngDatas[i]))
	}

	// Write PNG data for each image
	for _, data := range pngDatas {
		if _, err := icoFile.Write(data); err != nil {
			fmt.Printf("Error writing PNG data: %v\n", err)
			return
		}
	}

	// Also save individual PNGs for verification
	for i, size := range sizes {
		pngPath := filepath.Join(dir, fmt.Sprintf("multi_%dx%d.png", size, size))
		pngFile, err := os.Create(pngPath)
		if err != nil {
			continue
		}
		png.Encode(pngFile, images[i])
		pngFile.Close()
	}

	fmt.Println("Generated multi_sizes.ico")
}

func generateBMPFormatICO(dir string) {
	// Create a 32x32 ICO with BMP format (32-bit BGRA)
	size := 32
	c := color.NRGBA{R: 200, G: 100, B: 50, A: 255}
	img := createTestImage(size, c)

	// Save PNG for verification
	pngPath := filepath.Join(dir, "bmp_format.png")
	pngFile, err := os.Create(pngPath)
	if err != nil {
		fmt.Printf("Error creating %s: %v\n", pngPath, err)
		return
	}
	png.Encode(pngFile, img)
	pngFile.Close()

	// Create BMP-format ICO
	icoPath := filepath.Join(dir, "bmp_format.ico")
	icoFile, err := os.Create(icoPath)
	if err != nil {
		fmt.Printf("Error creating %s: %v\n", icoPath, err)
		return
	}
	defer icoFile.Close()

	// Create BITMAPINFOHEADER (40 bytes) + pixel data
	// Height is doubled in ICO BMP to account for XOR and AND masks
	dibHeaderSize := uint32(40)
	width := uint32(size)
	height := uint32(size * 2) // Doubled for ICO format
	planes := uint16(1)
	bitsPerPixel := uint16(32)
	compression := uint32(0)
	rowSize := size * 4 // 32-bit = 4 bytes per pixel
	imageSize := uint32(rowSize * size)
	maskRowSize := (size + 31) / 32 * 4
	maskSize := maskRowSize * size

	// DIB header
	dibHeader := make([]byte, 40)
	binary.LittleEndian.PutUint32(dibHeader[0:4], dibHeaderSize)
	binary.LittleEndian.PutUint32(dibHeader[4:8], width)
	binary.LittleEndian.PutUint32(dibHeader[8:12], height)
	binary.LittleEndian.PutUint16(dibHeader[12:14], planes)
	binary.LittleEndian.PutUint16(dibHeader[14:16], bitsPerPixel)
	binary.LittleEndian.PutUint32(dibHeader[16:20], compression)
	binary.LittleEndian.PutUint32(dibHeader[20:24], imageSize)
	// Rest can be zeros

	// Pixel data (BGRA, bottom-up)
	pixelData := make([]byte, imageSize)
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			c := img.NRGBAAt(x, size-1-y) // Bottom-up
			offset := y*rowSize + x*4
			pixelData[offset+0] = c.B
			pixelData[offset+1] = c.G
			pixelData[offset+2] = c.R
			pixelData[offset+3] = c.A
		}
	}

	// AND mask (all zeros = fully opaque)
	andMask := make([]byte, maskSize)

	// Total image data size
	totalImageSize := uint32(len(dibHeader)) + imageSize + uint32(maskSize)

	// ICO header
	header := head{Zero: 0, Type: 1, Number: 1}
	entry := direntry{
		Width:   uint8(size),
		Height:  uint8(size),
		Palette: 0,
		Plane:   1,
		Bits:    32,
		Size:    totalImageSize,
		Offset:  22,
	}

	binary.Write(icoFile, binary.LittleEndian, header)
	binary.Write(icoFile, binary.LittleEndian, entry)
	icoFile.Write(dibHeader)
	icoFile.Write(pixelData)
	icoFile.Write(andMask)

	fmt.Println("Generated bmp_format.ico")
}

func generateBitDepthVariants(dir string) {
	// Generate 24-bit BMP ICO (no alpha channel)
	generate24BitICO(dir)

	// Generate 8-bit (256 color palette) ICO
	generate8BitICO(dir)

	// Generate 4-bit (16 color palette) ICO
	generate4BitICO(dir)

	// Generate 1-bit (monochrome) ICO
	generate1BitICO(dir)
}

func generate24BitICO(dir string) {
	size := 32
	c := color.NRGBA{R: 100, G: 200, B: 150, A: 255}
	img := createTestImage(size, c)

	// Save PNG for verification
	pngPath := filepath.Join(dir, "24bit.png")
	pngFile, _ := os.Create(pngPath)
	png.Encode(pngFile, img)
	pngFile.Close()

	icoPath := filepath.Join(dir, "24bit.ico")
	icoFile, err := os.Create(icoPath)
	if err != nil {
		fmt.Printf("Error creating %s: %v\n", icoPath, err)
		return
	}
	defer icoFile.Close()

	// 24-bit BMP
	dibHeaderSize := uint32(40)
	width := uint32(size)
	height := uint32(size * 2)
	planes := uint16(1)
	bitsPerPixel := uint16(24)
	rowSize := ((size*3 + 3) / 4) * 4 // Padded to 4 bytes
	imageSize := uint32(rowSize * size)
	maskRowSize := (size + 31) / 32 * 4
	maskSize := maskRowSize * size

	dibHeader := make([]byte, 40)
	binary.LittleEndian.PutUint32(dibHeader[0:4], dibHeaderSize)
	binary.LittleEndian.PutUint32(dibHeader[4:8], width)
	binary.LittleEndian.PutUint32(dibHeader[8:12], height)
	binary.LittleEndian.PutUint16(dibHeader[12:14], planes)
	binary.LittleEndian.PutUint16(dibHeader[14:16], bitsPerPixel)

	pixelData := make([]byte, imageSize)
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			c := img.NRGBAAt(x, size-1-y)
			offset := y*rowSize + x*3
			pixelData[offset+0] = c.B
			pixelData[offset+1] = c.G
			pixelData[offset+2] = c.R
		}
	}

	andMask := make([]byte, maskSize)
	totalImageSize := uint32(len(dibHeader)) + imageSize + uint32(maskSize)

	header := head{Zero: 0, Type: 1, Number: 1}
	entry := direntry{
		Width:   uint8(size),
		Height:  uint8(size),
		Palette: 0,
		Plane:   1,
		Bits:    24,
		Size:    totalImageSize,
		Offset:  22,
	}

	binary.Write(icoFile, binary.LittleEndian, header)
	binary.Write(icoFile, binary.LittleEndian, entry)
	icoFile.Write(dibHeader)
	icoFile.Write(pixelData)
	icoFile.Write(andMask)

	fmt.Println("Generated 24bit.ico")
}

func generate8BitICO(dir string) {
	size := 32

	icoPath := filepath.Join(dir, "8bit.ico")
	icoFile, err := os.Create(icoPath)
	if err != nil {
		fmt.Printf("Error creating %s: %v\n", icoPath, err)
		return
	}
	defer icoFile.Close()

	// 8-bit (256 color) BMP with palette
	dibHeaderSize := uint32(40)
	width := uint32(size)
	height := uint32(size * 2)
	planes := uint16(1)
	bitsPerPixel := uint16(8)
	numColors := uint32(256)
	paletteSize := numColors * 4 // BGRA for each color
	rowSize := ((size + 3) / 4) * 4
	imageSize := uint32(rowSize * size)
	maskRowSize := (size + 31) / 32 * 4
	maskSize := maskRowSize * size

	dibHeader := make([]byte, 40)
	binary.LittleEndian.PutUint32(dibHeader[0:4], dibHeaderSize)
	binary.LittleEndian.PutUint32(dibHeader[4:8], width)
	binary.LittleEndian.PutUint32(dibHeader[8:12], height)
	binary.LittleEndian.PutUint16(dibHeader[12:14], planes)
	binary.LittleEndian.PutUint16(dibHeader[14:16], bitsPerPixel)
	binary.LittleEndian.PutUint32(dibHeader[32:36], numColors)

	// Create a simple gradient palette
	palette := make([]byte, paletteSize)
	for i := 0; i < 256; i++ {
		offset := i * 4
		palette[offset+0] = uint8(i)       // B
		palette[offset+1] = uint8(255 - i) // G
		palette[offset+2] = uint8(i / 2)   // R
		palette[offset+3] = 0              // Reserved
	}

	// Pixel data (indices into palette)
	pixelData := make([]byte, imageSize)
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			idx := (x + y) % 256
			offset := y*rowSize + x
			pixelData[offset] = uint8(idx)
		}
	}

	andMask := make([]byte, maskSize)
	totalImageSize := uint32(len(dibHeader)) + uint32(paletteSize) + imageSize + uint32(maskSize)

	header := head{Zero: 0, Type: 1, Number: 1}
	entry := direntry{
		Width:   uint8(size),
		Height:  uint8(size),
		Palette: 0,
		Plane:   1,
		Bits:    8,
		Size:    totalImageSize,
		Offset:  22,
	}

	binary.Write(icoFile, binary.LittleEndian, header)
	binary.Write(icoFile, binary.LittleEndian, entry)
	icoFile.Write(dibHeader)
	icoFile.Write(palette)
	icoFile.Write(pixelData)
	icoFile.Write(andMask)

	fmt.Println("Generated 8bit.ico")
}

func generate4BitICO(dir string) {
	size := 32

	icoPath := filepath.Join(dir, "4bit.ico")
	icoFile, err := os.Create(icoPath)
	if err != nil {
		fmt.Printf("Error creating %s: %v\n", icoPath, err)
		return
	}
	defer icoFile.Close()

	// 4-bit (16 color) BMP with palette
	dibHeaderSize := uint32(40)
	width := uint32(size)
	height := uint32(size * 2)
	planes := uint16(1)
	bitsPerPixel := uint16(4)
	numColors := uint32(16)
	rowSize := ((size/2 + 3) / 4) * 4
	imageSize := uint32(rowSize * size)
	maskRowSize := (size + 31) / 32 * 4
	maskSize := maskRowSize * size

	dibHeader := make([]byte, 40)
	binary.LittleEndian.PutUint32(dibHeader[0:4], dibHeaderSize)
	binary.LittleEndian.PutUint32(dibHeader[4:8], width)
	binary.LittleEndian.PutUint32(dibHeader[8:12], height)
	binary.LittleEndian.PutUint16(dibHeader[12:14], planes)
	binary.LittleEndian.PutUint16(dibHeader[14:16], bitsPerPixel)
	binary.LittleEndian.PutUint32(dibHeader[32:36], numColors)

	// Standard 16-color palette (Windows default)
	palette := []byte{
		0x00, 0x00, 0x00, 0x00, // Black
		0x00, 0x00, 0x80, 0x00, // Dark Red
		0x00, 0x80, 0x00, 0x00, // Dark Green
		0x00, 0x80, 0x80, 0x00, // Dark Yellow
		0x80, 0x00, 0x00, 0x00, // Dark Blue
		0x80, 0x00, 0x80, 0x00, // Dark Magenta
		0x80, 0x80, 0x00, 0x00, // Dark Cyan
		0xC0, 0xC0, 0xC0, 0x00, // Light Gray
		0x80, 0x80, 0x80, 0x00, // Dark Gray
		0x00, 0x00, 0xFF, 0x00, // Red
		0x00, 0xFF, 0x00, 0x00, // Green
		0x00, 0xFF, 0xFF, 0x00, // Yellow
		0xFF, 0x00, 0x00, 0x00, // Blue
		0xFF, 0x00, 0xFF, 0x00, // Magenta
		0xFF, 0xFF, 0x00, 0x00, // Cyan
		0xFF, 0xFF, 0xFF, 0x00, // White
	}

	// Pixel data (2 pixels per byte)
	pixelData := make([]byte, imageSize)
	for y := 0; y < size; y++ {
		for x := 0; x < size; x += 2 {
			idx1 := (x + y) % 16
			idx2 := (x + y + 1) % 16
			offset := y*rowSize + x/2
			pixelData[offset] = uint8(idx1<<4 | idx2)
		}
	}

	andMask := make([]byte, maskSize)
	totalImageSize := uint32(len(dibHeader)) + uint32(len(palette)) + imageSize + uint32(maskSize)

	header := head{Zero: 0, Type: 1, Number: 1}
	entry := direntry{
		Width:   uint8(size),
		Height:  uint8(size),
		Palette: 16,
		Plane:   1,
		Bits:    4,
		Size:    totalImageSize,
		Offset:  22,
	}

	binary.Write(icoFile, binary.LittleEndian, header)
	binary.Write(icoFile, binary.LittleEndian, entry)
	icoFile.Write(dibHeader)
	icoFile.Write(palette)
	icoFile.Write(pixelData)
	icoFile.Write(andMask)

	fmt.Println("Generated 4bit.ico")
}

func generate1BitICO(dir string) {
	size := 32

	icoPath := filepath.Join(dir, "1bit.ico")
	icoFile, err := os.Create(icoPath)
	if err != nil {
		fmt.Printf("Error creating %s: %v\n", icoPath, err)
		return
	}
	defer icoFile.Close()

	// 1-bit (monochrome) BMP with palette
	dibHeaderSize := uint32(40)
	width := uint32(size)
	height := uint32(size * 2)
	planes := uint16(1)
	bitsPerPixel := uint16(1)
	numColors := uint32(2)
	rowSize := (size + 31) / 32 * 4
	imageSize := uint32(rowSize * size)
	maskRowSize := (size + 31) / 32 * 4
	maskSize := maskRowSize * size

	dibHeader := make([]byte, 40)
	binary.LittleEndian.PutUint32(dibHeader[0:4], dibHeaderSize)
	binary.LittleEndian.PutUint32(dibHeader[4:8], width)
	binary.LittleEndian.PutUint32(dibHeader[8:12], height)
	binary.LittleEndian.PutUint16(dibHeader[12:14], planes)
	binary.LittleEndian.PutUint16(dibHeader[14:16], bitsPerPixel)
	binary.LittleEndian.PutUint32(dibHeader[32:36], numColors)

	// Black and white palette
	palette := []byte{
		0x00, 0x00, 0x00, 0x00, // Black
		0xFF, 0xFF, 0xFF, 0x00, // White
	}

	// Pixel data (8 pixels per byte) - create a checkerboard pattern
	pixelData := make([]byte, imageSize)
	for y := 0; y < size; y++ {
		for byteIdx := 0; byteIdx < rowSize; byteIdx++ {
			if byteIdx < size/8 {
				// Alternating pattern based on row
				if y%2 == 0 {
					pixelData[y*rowSize+byteIdx] = 0xAA // 10101010
				} else {
					pixelData[y*rowSize+byteIdx] = 0x55 // 01010101
				}
			}
		}
	}

	andMask := make([]byte, maskSize)
	totalImageSize := uint32(len(dibHeader)) + uint32(len(palette)) + imageSize + uint32(maskSize)

	header := head{Zero: 0, Type: 1, Number: 1}
	entry := direntry{
		Width:   uint8(size),
		Height:  uint8(size),
		Palette: 2,
		Plane:   1,
		Bits:    1,
		Size:    totalImageSize,
		Offset:  22,
	}

	binary.Write(icoFile, binary.LittleEndian, header)
	binary.Write(icoFile, binary.LittleEndian, entry)
	icoFile.Write(dibHeader)
	icoFile.Write(palette)
	icoFile.Write(pixelData)
	icoFile.Write(andMask)

	fmt.Println("Generated 1bit.ico")
}

func generateEdgeCases(dir string) {
	// 1. Empty ICO (header says 0 images)
	emptyPath := filepath.Join(dir, "empty.ico")
	emptyFile, err := os.Create(emptyPath)
	if err == nil {
		header := head{Zero: 0, Type: 1, Number: 0}
		binary.Write(emptyFile, binary.LittleEndian, header)
		emptyFile.Close()
		fmt.Println("Generated empty.ico")
	}

	// 2. Corrupt header (invalid magic bytes)
	corruptPath := filepath.Join(dir, "corrupt_header.ico")
	corruptFile, err := os.Create(corruptPath)
	if err == nil {
		// Write invalid header (Type should be 1 for ICO)
		header := head{Zero: 0xFF, Type: 0xFF, Number: 1}
		binary.Write(corruptFile, binary.LittleEndian, header)
		corruptFile.Close()
		fmt.Println("Generated corrupt_header.ico")
	}

	// 3. Truncated file (valid header, but missing image data)
	truncPath := filepath.Join(dir, "truncated.ico")
	truncFile, err := os.Create(truncPath)
	if err == nil {
		header := head{Zero: 0, Type: 1, Number: 1}
		entry := direntry{
			Width:   32,
			Height:  32,
			Palette: 0,
			Plane:   1,
			Bits:    32,
			Size:    1000, // Claims 1000 bytes but we won't write them
			Offset:  22,
		}
		binary.Write(truncFile, binary.LittleEndian, header)
		binary.Write(truncFile, binary.LittleEndian, entry)
		// Don't write any image data - file is truncated
		truncFile.Close()
		fmt.Println("Generated truncated.ico")
	}

	// 4. Invalid entry size (size = 0)
	invalidSizePath := filepath.Join(dir, "invalid_size.ico")
	invalidSizeFile, err := os.Create(invalidSizePath)
	if err == nil {
		header := head{Zero: 0, Type: 1, Number: 1}
		entry := direntry{
			Width:   32,
			Height:  32,
			Palette: 0,
			Plane:   1,
			Bits:    32,
			Size:    0, // Invalid: size is 0
			Offset:  22,
		}
		binary.Write(invalidSizeFile, binary.LittleEndian, header)
		binary.Write(invalidSizeFile, binary.LittleEndian, entry)
		invalidSizeFile.Close()
		fmt.Println("Generated invalid_size.ico")
	}

	// 5. Offset beyond file (valid header, entry points past EOF)
	badOffsetPath := filepath.Join(dir, "bad_offset.ico")
	badOffsetFile, err := os.Create(badOffsetPath)
	if err == nil {
		header := head{Zero: 0, Type: 1, Number: 1}
		entry := direntry{
			Width:   32,
			Height:  32,
			Palette: 0,
			Plane:   1,
			Bits:    32,
			Size:    100,
			Offset:  10000, // Points way past end of file
		}
		binary.Write(badOffsetFile, binary.LittleEndian, header)
		binary.Write(badOffsetFile, binary.LittleEndian, entry)
		badOffsetFile.Close()
		fmt.Println("Generated bad_offset.ico")
	}
}
