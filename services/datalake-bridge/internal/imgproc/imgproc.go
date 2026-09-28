// Package imgproc normaliza las fotos de las máquinas a un formato de catálogo:
// fondo blanco, recortada al contenido y cuadrada. Funciona bien con imágenes de
// catálogo (ya sobre blanco o transparente); NO hace segmentación real, así que
// una foto con fondo ocupado (bodega, pasto) no queda recortada del objeto —
// para eso haría falta un servicio de recorte por IA (remove.bg/rembg).
package imgproc

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	_ "image/gif" // registrar decoders
	"image/jpeg"
	_ "image/jpeg" //
	_ "image/png"  //

	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/webp"
)

const (
	targetSide  = 1000 // lado final en px
	whiteThresh = 244  // r,g,b >= esto se considera fondo blanco
	padFrac     = 0.04 // margen alrededor del recorte
)

func decode(raw []byte) (image.Image, error) {
	if img, _, err := image.Decode(bytes.NewReader(raw)); err == nil {
		return img, nil
	}
	if img, err := webp.Decode(bytes.NewReader(raw)); err == nil {
		return img, nil
	}
	return nil, fmt.Errorf("formato de imagen no soportado")
}

// WhiteBgTrim aplana sobre blanco, recorta al contenido, cuadra y redimensiona.
// Devuelve JPEG. Si la imagen es toda blanca (o no se detecta contenido), no
// recorta: solo cuadra y redimensiona.
func WhiteBgTrim(raw []byte) ([]byte, error) {
	src, err := decode(raw)
	if err != nil {
		return nil, err
	}
	b := src.Bounds()

	// Aplanar sobre blanco (elimina transparencia).
	flat := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(flat, flat.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
	draw.Draw(flat, flat.Bounds(), src, b.Min, draw.Over)

	// Bounding box de los píxeles NO blancos (acceso directo a Pix por velocidad).
	w, h := flat.Bounds().Dx(), flat.Bounds().Dy()
	minX, minY, maxX, maxY := w, h, -1, -1
	for y := 0; y < h; y++ {
		row := y * flat.Stride
		for x := 0; x < w; x++ {
			o := row + x*4
			if flat.Pix[o] < whiteThresh || flat.Pix[o+1] < whiteThresh || flat.Pix[o+2] < whiteThresh {
				if x < minX {
					minX = x
				}
				if x > maxX {
					maxX = x
				}
				if y < minY {
					minY = y
				}
				if y > maxY {
					maxY = y
				}
			}
		}
	}

	var crop image.Image = flat
	if maxX >= minX && maxY >= minY {
		cw, ch := maxX-minX+1, maxY-minY+1
		pad := int(float64(maxi(cw, ch)) * padFrac)
		r := image.Rect(
			clamp(minX-pad, 0, w), clamp(minY-pad, 0, h),
			clamp(maxX+1+pad, 0, w), clamp(maxY+1+pad, 0, h),
		)
		crop = flat.SubImage(r)
	}

	// Cuadrar centrado sobre blanco.
	cb := crop.Bounds()
	side := maxi(cb.Dx(), cb.Dy())
	square := image.NewRGBA(image.Rect(0, 0, side, side))
	draw.Draw(square, square.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
	ox, oy := (side-cb.Dx())/2, (side-cb.Dy())/2
	draw.Draw(square, image.Rect(ox, oy, ox+cb.Dx(), oy+cb.Dy()), crop, cb.Min, draw.Over)

	// Redimensionar al lado objetivo (alta calidad).
	out := image.NewRGBA(image.Rect(0, 0, targetSide, targetSide))
	xdraw.CatmullRom.Scale(out, out.Bounds(), square, square.Bounds(), xdraw.Src, nil)

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, out, &jpeg.Options{Quality: 90}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func maxi(a, b int) int {
	if a > b {
		return a
	}
	return b
}
func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
