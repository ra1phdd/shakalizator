package shakalizator

import (
	"bytes"
	"github.com/disintegration/imaging"
	"image"
	"image/jpeg"
	"log"
)

func ShakalizeImage(src image.Image, level int) *bytes.Buffer {
	level *= 3
	scaleFactor := 1.0 / (4.0 + float64(level)/3.0)
	jpegQuality := 31 - level

	smallWidth := int(float64(src.Bounds().Dx()) * scaleFactor)
	smallHeight := int(float64(src.Bounds().Dy()) * scaleFactor)
	small := imaging.Resize(src, smallWidth, smallHeight, imaging.Lanczos)
	large := imaging.Resize(small, src.Bounds().Dx(), src.Bounds().Dy(), imaging.NearestNeighbor)

	var buf bytes.Buffer
	err := jpeg.Encode(&buf, large, &jpeg.Options{Quality: jpegQuality})
	if err != nil {
		log.Println(err)
		return nil
	}

	return &buf
}
