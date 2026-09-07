package clipboard

// Wayland 后端使用的 MIME 类型清单。
// 文本按常见度排列：Wayland 合成器普遍提供 text/plain;charset=utf-8，
// 个别应用只提供 text/plain 或 X11 风格的 UTF8_STRING；
// 图片与应用间协议一致，仅支持 PNG（engine 侧校验 PNG 魔数）。
var (
	textReadMimes  = []string{"text/plain;charset=utf-8", "text/plain", "UTF8_STRING"}
	imageReadMimes = []string{"image/png"}
)

const (
	textWriteMime  = "text/plain;charset=utf-8"
	imageWriteMime = "image/png"
)

// pickMime 从 data offer 提供的 mime 列表中选出 candidates 里第一个可用的；
// 找不到返回空串，表示剪贴板当前没有该类型的内容。
func pickMime(offers, candidates []string) string {
	for _, want := range candidates {
		for _, got := range offers {
			if got == want {
				return want
			}
		}
	}
	return ""
}
