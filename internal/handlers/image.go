package handlers

import (
	"bytes"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"net/http"
	"strconv"

	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

// ImageParams holds parsed image transformation parameters
type ImageParams struct {
	Width   int    // target width (0 = auto)
	Height  int    // target height (0 = auto)
	Fit     string // "contain" (default) or "cover"
	Quality int    // JPEG quality 1-100 (default 80)
}

// parseImageParams extracts image resize params from query string
func parseImageParams(r *http.Request) *ImageParams {
	q := r.URL.Query()

	w, _ := strconv.Atoi(q.Get("w"))
	h, _ := strconv.Atoi(q.Get("h"))

	if w <= 0 && h <= 0 {
		return nil
	}

	fit := q.Get("fit")
	if fit == "" {
		if w > 0 && h > 0 {
			fit = "cover"
		} else {
			fit = "contain"
		}
	}

	quality, _ := strconv.Atoi(q.Get("q"))
	if quality <= 0 || quality > 100 {
		quality = 80
	}

	return &ImageParams{
		Width:   w,
		Height:  h,
		Fit:     fit,
		Quality: quality,
	}
}

// isImageContentType checks if the content type is a resizable image
func isImageContentType(contentType string) bool {
	switch contentType {
	case "image/jpeg", "image/png", "image/jpg", "image/webp", "image/gif":
		return true
	}
	return false
}

// resizeImage resizes the image data according to the given params
func resizeImage(data []byte, contentType string, params *ImageParams) ([]byte, string, error) {
	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, "", err
	}

	srcBounds := src.Bounds()
	srcW := srcBounds.Dx()
	srcH := srcBounds.Dy()

	dstW, dstH := calculateDimensions(srcW, srcH, params)

	if dstW >= srcW && dstH >= srcH {
		return data, contentType, nil
	}

	var dst *image.NRGBA

	if params.Fit == "cover" {
		dst = resizeCover(src, srcW, srcH, dstW, dstH)
	} else {
		dst = resizeContain(src, srcW, srcH, dstW, dstH)
	}

	var buf bytes.Buffer
	outType := contentType

	switch contentType {
	case "image/png":
		err = png.Encode(&buf, dst)
	case "image/gif":
		err = gif.Encode(&buf, dst, nil)
	default:
		outType = "image/jpeg"
		err = jpeg.Encode(&buf, dst, &jpeg.Options{Quality: params.Quality})
	}

	if err != nil {
		return nil, "", err
	}

	return buf.Bytes(), outType, nil
}

func calculateDimensions(srcW, srcH int, params *ImageParams) (int, int) {
	w := params.Width
	h := params.Height

	if w > 0 && h <= 0 {
		h = srcH * w / srcW
	} else if h > 0 && w <= 0 {
		w = srcW * h / srcH
	}

	if w > srcW {
		w = srcW
	}
	if h > srcH {
		h = srcH
	}

	return w, h
}

func resizeContain(src image.Image, srcW, srcH, dstW, dstH int) *image.NRGBA {
	scaleW := float64(dstW) / float64(srcW)
	scaleH := float64(dstH) / float64(srcH)
	scale := scaleW
	if scaleH < scaleW {
		scale = scaleH
	}

	finalW := int(float64(srcW) * scale)
	finalH := int(float64(srcH) * scale)
	if finalW < 1 {
		finalW = 1
	}
	if finalH < 1 {
		finalH = 1
	}

	dst := image.NewNRGBA(image.Rect(0, 0, finalW, finalH))
	draw.BiLinear.Scale(dst, dst.Bounds(), src, src.Bounds(), draw.Over, nil)
	return dst
}

func resizeCover(src image.Image, srcW, srcH, dstW, dstH int) *image.NRGBA {
	scaleW := float64(dstW) / float64(srcW)
	scaleH := float64(dstH) / float64(srcH)
	scale := scaleW
	if scaleH > scaleW {
		scale = scaleH
	}

	interW := int(float64(srcW) * scale)
	interH := int(float64(srcH) * scale)
	if interW < 1 {
		interW = 1
	}
	if interH < 1 {
		interH = 1
	}

	inter := image.NewNRGBA(image.Rect(0, 0, interW, interH))
	draw.BiLinear.Scale(inter, inter.Bounds(), src, src.Bounds(), draw.Over, nil)

	cropX := (interW - dstW) / 2
	cropY := (interH - dstH) / 2

	dst := image.NewNRGBA(image.Rect(0, 0, dstW, dstH))
	draw.Copy(dst, image.Point{}, inter, image.Rect(cropX, cropY, cropX+dstW, cropY+dstH), draw.Over, nil)
	return dst
}
