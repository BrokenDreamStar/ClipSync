package clipboard

import "testing"

func TestPickMime(t *testing.T) {
	cases := []struct {
		name       string
		offers     []string
		candidates []string
		want       string
	}{
		{
			name:       "标准 utf8 文本",
			offers:     []string{"text/plain;charset=utf-8", "text/plain", "STRING"},
			candidates: textReadMimes,
			want:       "text/plain;charset=utf-8",
		},
		{
			name:       "仅 text/plain 时降级",
			offers:     []string{"text/plain"},
			candidates: textReadMimes,
			want:       "text/plain",
		},
		{
			name:       "X11 风格 UTF8_STRING",
			offers:     []string{"UTF8_STRING", "STRING"},
			candidates: textReadMimes,
			want:       "UTF8_STRING",
		},
		{
			name:       "PNG 图片",
			offers:     []string{"image/png", "text/html"},
			candidates: imageReadMimes,
			want:       "image/png",
		},
		{
			name:       "无匹配类型",
			offers:     []string{"text/html", "image/jpeg"},
			candidates: imageReadMimes,
			want:       "",
		},
		{
			name:       "空 offer 列表",
			offers:     nil,
			candidates: textReadMimes,
			want:       "",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := pickMime(c.offers, c.candidates); got != c.want {
				t.Fatalf("pickMime(%v) = %q, 期望 %q", c.offers, got, c.want)
			}
		})
	}
}
