package sync

import (
	"bytes"
	"image"
	"image/color"
	stddraw "image/draw"
	"image/jpeg"
	_ "image/png" // registra el decoder PNG

	xdraw "golang.org/x/image/draw"
	_ "golang.org/x/image/webp" // registra el decoder WebP
)

const (
	// odooMaxImageSide es el lado máximo que Odoo conserva en image_1920; subir
	// algo más grande solo infla el payload (y dispara 413 en el proxy de Odoo).
	odooMaxImageSide = 1920
	// odooImageSkipBytes: imágenes ya pequeñas se envían tal cual, sin recomprimir.
	odooImageSkipBytes = 512 * 1024
	odooJPEGQuality    = 85
)

// optimizeImageForOdoo reduce el peso de una imagen antes de enviarla en base64
// a Odoo: la escala a odooMaxImageSide y la recomprime como JPEG. Si la imagen
// ya es pequeña, o no se puede decodificar, devuelve los bytes originales.
func optimizeImageForOdoo(data []byte) []byte {
	if len(data) <= odooImageSkipBytes {
		return data
	}

	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return data
	}

	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	if w > odooMaxImageSide || h > odooMaxImageSide {
		if w >= h {
			h = h * odooMaxImageSide / w
			w = odooMaxImageSide
		} else {
			w = w * odooMaxImageSide / h
			h = odooMaxImageSide
		}
		if w < 1 {
			w = 1
		}
		if h < 1 {
			h = 1
		}
	}

	// Fondo blanco: JPEG no tiene alpha (PNG/WebP con transparencia).
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	stddraw.Draw(dst, dst.Bounds(), &image.Uniform{C: color.White}, image.Point{}, stddraw.Src)
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), src, b, xdraw.Over, nil)

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, dst, &jpeg.Options{Quality: odooJPEGQuality}); err != nil {
		return data
	}
	if buf.Len() >= len(data) {
		return data
	}

	return buf.Bytes()
}
