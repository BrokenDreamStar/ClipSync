package engine

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"
)

func makeTestImage(w, h int) image.Image {
	m := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			m.SetNRGBA(x, y, color.NRGBA{R: uint8(x * 4), G: uint8(y * 5), B: uint8((x + y) * 2), A: 255})
		}
	}
	return m
}

func makeTestImageSolid(w, h int, c color.Color) image.Image {
	m := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			m.Set(x, y, c)
		}
	}
	return m
}

func pngBytes(im image.Image) []byte {
	var b bytes.Buffer
	_ = png.Encode(&b, im)
	return b.Bytes()
}

func TestImageFPStableAcrossReencode(t *testing.T) {
	orig := pngBytes(makeTestImage(64, 48))
	fp1, err := imageFP(orig)
	if err != nil {
		t.Fatalf("imageFP(orig): %v", err)
	}

	// 用不同压缩级别重编码同一张图（模拟 macOS 对 PNG 的重编码：字节不同、像素一致）。
	var buf bytes.Buffer
	enc := png.Encoder{CompressionLevel: png.BestCompression}
	if err := enc.Encode(&buf, decodePNG(t, orig)); err != nil {
		t.Fatalf("re-encode: %v", err)
	}
	re := buf.Bytes()
	if bytes.Equal(orig, re) {
		t.Fatal("测试前提失败：重编码后字节竟相同")
	}
	fp2, err := imageFP(re)
	if err != nil {
		t.Fatalf("imageFP(re): %v", err)
	}
	if fp1 != fp2 {
		t.Fatalf("同一图片重编码后内容指纹不一致:\n%x\n%x", fp1, fp2)
	}
}

func TestImageFPDiffersForDifferentImage(t *testing.T) {
	a := pngBytes(makeTestImage(64, 48))
	b := pngBytes(makeTestImageSolid(64, 48, color.RGBA{R: 255, A: 255}))
	fpa, err := imageFP(a)
	if err != nil {
		t.Fatal(err)
	}
	fpb, err := imageFP(b)
	if err != nil {
		t.Fatal(err)
	}
	if fpa == fpb {
		t.Fatal("不同图片内容指纹不应相同")
	}
}

func TestImageFPRejectsNonPNG(t *testing.T) {
	if _, err := imageFP([]byte("not a png at all")); err == nil {
		t.Fatal("非 PNG 数据应返回错误")
	}
}

func TestTruncateRunes(t *testing.T) {
	cases := []struct {
		in   string
		n    int
		want string
	}{
		{"hello", 3, "hel"},
		{"汉语abc", 2, "汉语"},
		{"汉语abc", 5, "汉语abc"},
		{"汉语abc", 100, "汉语abc"},
		{"", 3, ""},
		{"你好", 0, ""},
		{"a", 1, "a"},
	}
	for _, c := range cases {
		if got := truncateRunes(c.in, c.n); got != c.want {
			t.Errorf("truncateRunes(%q, %d) = %q, want %q", c.in, c.n, got, c.want)
		}
	}
}

func decodePNG(t *testing.T, b []byte) image.Image {
	t.Helper()
	img, err := png.Decode(bytes.NewReader(b))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	return img
}
