package ico

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io"

	bmp "github.com/jsummers/gobmp"
)

const maxICOSize = int64(64 << 20) // hard cap to avoid OOM panics on hostile inputs

func init() {
	image.RegisterFormat("ico", "\x00\x00\x01\x00?????\x00", Decode, DecodeConfig)
}

// ---- public ----

func Decode(r io.Reader) (image.Image, error) {
	var d decoder
	if err := d.decode(r); err != nil {
		return nil, err
	}
	if len(d.images) == 0 {
		return nil, fmt.Errorf("ico: no images")
	}
	return d.images[0], nil
}

func DecodeAll(r io.Reader) ([]image.Image, error) {
	var d decoder
	if err := d.decode(r); err != nil {
		return nil, err
	}
	return d.images, nil
}

func DecodeConfig(r io.Reader) (image.Config, error) {
	var (
		d   decoder
		cfg image.Config
		err error
	)

	file, err := readAllICO(r)
	if err != nil {
		return cfg, err
	}

	br := bytes.NewReader(file)
	if err = d.decodeHeader(br); err != nil {
		return cfg, err
	}
	if err = d.decodeEntries(br); err != nil {
		return cfg, err
	}
	if len(d.entries) == 0 {
		return cfg, fmt.Errorf("ico: no images")
	}

	e := &(d.entries[0])
	entryData, err := d.entryBytes(file, e)
	if err != nil {
		return cfg, err
	}

	if len(entryData) >= len(pngHeader) && bytes.Equal(entryData[:len(pngHeader)], pngHeader) {
		return png.DecodeConfig(bytes.NewReader(entryData))
	}

	buf := make([]byte, 14+len(entryData))
	copy(buf[14:], entryData)

	_, bmpSize, err := d.forgeBMPHead(buf, e)
	if err != nil {
		return cfg, err
	}
	return bmp.DecodeConfig(bytes.NewReader(buf[:bmpSize]))
}

// ---- private ----

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

type head struct {
	Zero   uint16
	Type   uint16
	Number uint16
}

type decoder struct {
	head    head
	entries []direntry
	images  []image.Image
}

func readAllICO(r io.Reader) ([]byte, error) {
	b, err := io.ReadAll(io.LimitReader(r, maxICOSize+1))
	if err != nil {
		return nil, err
	}
	if int64(len(b)) > maxICOSize {
		return nil, fmt.Errorf("ico: file too large")
	}
	return b, nil
}

func (d *decoder) entryBytes(file []byte, e *direntry) ([]byte, error) {
	start := int64(e.Offset)
	size := int64(e.Size)
	if size <= 0 {
		return nil, fmt.Errorf("ico: corrupted entry (size=%d)", e.Size)
	}
	end := start + size
	if start < 0 || end < start || end > int64(len(file)) {
		return nil, io.ErrUnexpectedEOF
	}
	return file[int(start):int(end)], nil
}

func (d *decoder) decode(r io.Reader) (err error) {
	file, err := readAllICO(r)
	if err != nil {
		return err
	}

	br := bytes.NewReader(file)
	if err = d.decodeHeader(br); err != nil {
		return err
	}
	if err = d.decodeEntries(br); err != nil {
		return err
	}

	d.images = make([]image.Image, len(d.entries))
	for i := range d.entries {
		e := &(d.entries[i])

		entryData, err := d.entryBytes(file, e)
		if err != nil {
			return err
		}

		if len(entryData) >= len(pngHeader) && bytes.Equal(entryData[:len(pngHeader)], pngHeader) { // decode as PNG
			if d.images[i], err = png.Decode(bytes.NewReader(entryData)); err != nil {
				return err
			}
			continue
		}

		// decode as BMP
		data := make([]byte, 14+len(entryData))
		copy(data[14:], entryData)

		maskData, bmpSize, err := d.forgeBMPHead(data, e)
		if err != nil {
			return err
		}

		bmpImg, err := bmp.Decode(bytes.NewReader(data[:bmpSize]))
		if err != nil {
			return err
		}

		bounds := bmpImg.Bounds()
		w, h := bounds.Dx(), bounds.Dy()
		if w <= 0 || h <= 0 {
			d.images[i] = bmpImg
			continue
		}

		mask := image.NewAlpha(image.Rect(0, 0, w, h))
		masked := image.NewNRGBA(image.Rect(0, 0, w, h))

		if maskData != nil {
			rowSize := (w + 31) / 32 * 4
			need := rowSize * h
			if need > len(maskData) {
				return fmt.Errorf("ico: corrupted mask data")
			}
			for row := 0; row < h; row++ {
				rowOff := row * rowSize
				for col := 0; col < w; col++ {
					if (maskData[rowOff+col/8]>>(7-uint(col)%8))&0x01 != 1 {
						mask.SetAlpha(col, h-row-1, color.Alpha{255})
					}
				}
			}
		} else { // 32-Bit (alpha in pixel data)
			bmpData := data[:bmpSize]
			if len(bmpData) < 14 {
				return fmt.Errorf("ico: corrupted bmp data")
			}

			rowSize := (w*32 + 31) / 32 * 4
			offset := int(binary.LittleEndian.Uint32(bmpData[10:14]))
			if offset < 0 || offset+rowSize*h > len(bmpData) {
				return fmt.Errorf("ico: corrupted bmp alpha data")
			}

			for row := 0; row < h; row++ {
				rowOff := offset + row*rowSize
				for col := 0; col < w; col++ {
					mask.SetAlpha(col, h-row-1, color.Alpha{bmpData[rowOff+col*4+3]})
				}
			}
		}

		draw.DrawMask(masked, masked.Bounds(), bmpImg, bounds.Min, mask, bounds.Min, draw.Src)
		d.images[i] = masked
	}

	return nil
}

func (d *decoder) decodeHeader(r io.Reader) error {
	if err := binary.Read(r, binary.LittleEndian, &(d.head)); err != nil {
		return err
	}
	if d.head.Zero != 0 || d.head.Type != 1 {
		return fmt.Errorf("corrupted head: [%x,%x]", d.head.Zero, d.head.Type)
	}
	if d.head.Number == 0 {
		return fmt.Errorf("ico: no images")
	}
	return nil
}

func (d *decoder) decodeEntries(r io.Reader) error {
	n := int(d.head.Number)
	d.entries = make([]direntry, n)
	for i := 0; i < n; i++ {
		if err := binary.Read(r, binary.LittleEndian, &(d.entries[i])); err != nil {
			return err
		}
	}
	return nil
}

func (d *decoder) forgeBMPHead(buf []byte, e *direntry) (mask []byte, bmpSize int, err error) {
	// See en.wikipedia.org/wiki/BMP_file_format
	if len(buf) < 14+4 {
		return nil, 0, io.ErrUnexpectedEOF
	}

	data := buf[14:]
	if len(data) < 4 {
		return nil, 0, io.ErrUnexpectedEOF
	}

	dibSize := binary.LittleEndian.Uint32(data[:4])
	if dibSize < 12 {
		return nil, 0, fmt.Errorf("ico: corrupted DIB header size (%d)", dibSize)
	}
	if len(data) < int(dibSize) {
		return nil, 0, io.ErrUnexpectedEOF
	}

	var (
		w         uint32
		h         uint32
		bits      uint16
		numColors uint32
	)

	switch dibSize {
	case 12: // BITMAPCOREHEADER
		if len(data) < 12 {
			return nil, 0, io.ErrUnexpectedEOF
		}
		w = uint32(binary.LittleEndian.Uint16(data[4:6]))
		h = uint32(binary.LittleEndian.Uint16(data[6:8]))
		bits = binary.LittleEndian.Uint16(data[10:12])
		numColors = 0
	default: // BITMAPINFOHEADER and later
		if len(data) < 16 {
			return nil, 0, io.ErrUnexpectedEOF
		}
		w = binary.LittleEndian.Uint32(data[4:8])
		h = binary.LittleEndian.Uint32(data[8:12])
		bits = binary.LittleEndian.Uint16(data[14:16])
		if len(data) >= 36 {
			numColors = binary.LittleEndian.Uint32(data[32:36])
		} else {
			numColors = 0
		}
	}

	// ICO BMP height is commonly stored as (XOR+AND) i.e. 2*height.
	// Keep the old heuristic but also handle non-square entries via the directory height.
	entryH := uint32(e.Height)
	if entryH == 0 {
		entryH = 256
	}
	if h%2 == 0 {
		half := h / 2
		if half == entryH || half == w || h > w {
			h = half
			if dibSize == 12 {
				if h > 0xFFFF {
					return nil, 0, fmt.Errorf("ico: corrupted bmp height (%d)", h)
				}
				binary.LittleEndian.PutUint16(data[6:8], uint16(h))
			} else {
				binary.LittleEndian.PutUint32(data[8:12], h)
			}
		}
	}

	imageSize := int64(len(data))
	if bits != 32 {
		if w == 0 || h == 0 {
			return nil, 0, fmt.Errorf("ico: corrupted bmp dimensions")
		}
		rowSize := (int64(w) + 31) / 32 * 4
		maskSize := rowSize * int64(h)
		if maskSize <= 0 || maskSize > imageSize {
			return nil, 0, fmt.Errorf("ico: corrupted bmp mask size")
		}
		imageSize -= maskSize
		if imageSize <= 0 {
			return nil, 0, fmt.Errorf("ico: corrupted bmp image size")
		}
		mask = data[int(imageSize):]
	}

	copy(buf[0:2], "\x42\x4D") // Magic number

	bmpSize = 14 + int(imageSize)
	binary.LittleEndian.PutUint32(buf[2:6], uint32(bmpSize)) // File size

	// Calculate offset into image data
	switch bits {
	case 1, 2, 4, 8:
		x := uint32(1) << bits
		if numColors == 0 || numColors > x {
			numColors = x
		}
	default:
		numColors = 0
	}

	var numColorsSize uint32
	switch dibSize {
	case 12, 64:
		numColorsSize = numColors * 3
	default:
		numColorsSize = numColors * 4
	}

	offset := uint32(14) + dibSize + numColorsSize
	ds := int(dibSize)
	if dibSize > 40 && ds >= 8 && ds-4 <= len(data) {
		offset += binary.LittleEndian.Uint32(data[ds-8 : ds-4])
	}

	if offset >= uint32(bmpSize) {
		return nil, 0, fmt.Errorf("ico: corrupted bmp data offset")
	}

	binary.LittleEndian.PutUint32(buf[10:14], offset)
	return mask, bmpSize, nil
}

var pngHeader = []byte{'\x89', 'P', 'N', 'G', '\r', '\n', '\x1a', '\n'}
