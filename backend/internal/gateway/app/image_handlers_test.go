package app

import (
	"bytes"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"testing"
)

func TestInspectImage(t *testing.T) {
	picture := image.NewRGBA(image.Rect(0, 0, 2, 2))
	picture.Set(0, 0, color.RGBA{R: 255, A: 255})
	encoders := []struct {
		name      string
		encode    func(*bytes.Buffer) error
		mediaType string
		extension string
	}{
		{name: "jpeg", encode: func(output *bytes.Buffer) error { return jpeg.Encode(output, picture, nil) }, mediaType: "image/jpeg", extension: ".jpg"},
		{name: "png", encode: func(output *bytes.Buffer) error { return png.Encode(output, picture) }, mediaType: "image/png", extension: ".png"},
		{name: "gif", encode: func(output *bytes.Buffer) error { return gif.Encode(output, picture, nil) }, mediaType: "image/gif", extension: ".gif"},
	}
	for _, test := range encoders {
		t.Run(test.name, func(t *testing.T) {
			var data bytes.Buffer
			if err := test.encode(&data); err != nil {
				t.Fatal(err)
			}
			mediaType, extension, err := inspectImage(data.Bytes())
			if err != nil {
				t.Fatal(err)
			}
			if mediaType != test.mediaType || extension != test.extension {
				t.Fatalf("inspectImage() = (%q, %q), want (%q, %q)", mediaType, extension, test.mediaType, test.extension)
			}
		})
	}

	t.Run("unsupported", func(t *testing.T) {
		if _, _, err := inspectImage([]byte("not an image")); err != errUnsupportedImage {
			t.Fatalf("inspectImage() error = %v, want %v", err, errUnsupportedImage)
		}
	})
	t.Run("invalid image data", func(t *testing.T) {
		if _, _, err := inspectImage([]byte("\x89PNG\r\n\x1a\ninvalid")); err != errInvalidImage {
			t.Fatalf("inspectImage() error = %v, want %v", err, errInvalidImage)
		}
	})
	t.Run("over size limit", func(t *testing.T) {
		if _, _, err := inspectImage(make([]byte, maxImageSize+1)); err != errImageTooLarge {
			t.Fatalf("inspectImage() error = %v, want %v", err, errImageTooLarge)
		}
	})
}
