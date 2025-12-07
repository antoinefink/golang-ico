# golang-ico

A small Go library for decoding and encoding Windows `.ico` files (PNG or BMP backed).

## Features
- Registers the `ico` format with Go's `image` package.
- `Decode`, `DecodeAll`, and `DecodeConfig` to read icons and dimensions safely.
- `Encode` writes PNG-based ICO files from any `image.Image`.

## Install
```
go get github.com/antoinefink/golang-ico
```

## Usage
Decode the first image in an icon:
```
f, _ := os.Open("icon.ico")
defer f.Close()
img, err := ico.Decode(f)
```

Decode every embedded size:
```
imgs, err := ico.DecodeAll(f)
```

Encode an image as ICO:
```
out, _ := os.Create("icon.ico")
defer out.Close()
err := ico.Encode(out, img)
```

## Testing
```
go test ./...
```

Forked from https://github.com/biessek/golang-ico itself based on work from https://github.com/zyxar/image2ascii and https://github.com/Kodeworks/golang-image-ico.
