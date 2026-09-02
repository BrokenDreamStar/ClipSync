package engine

import (
	"bytes"
	"fmt"

	"golang.design/x/clipboard"
)

// pngSignature 是 PNG 文件的 8 字节魔数。
var pngSignature = []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}

// isPNGSignal 判断一段数据是否为合法的 PNG 文件头。
// 完整解码一张图代价很高（多 MB），在校验场景下只需确认魔数即可，避免为校验解压整张位图。
func isPNGSignal(data []byte) bool {
	return len(data) >= len(pngSignature) && bytes.Equal(data[:len(pngSignature)], pngSignature)
}

// writeClipboard 把消息内容写入系统剪贴板。
func writeClipboard(kind MessageKind, data []byte) error {
	switch kind {
	case KindText:
		clipboard.Write(clipboard.FmtText, data)
		return nil
	case KindImage:
		// 校验 PNG 魔数，防止对端传坏数据；不做全量解码校验以降低大图开销。
		if !isPNGSignal(data) {
			return fmt.Errorf("非法图片数据: 非 PNG")
		}
		clipboard.Write(clipboard.FmtImage, data)
		return nil
	default:
		return fmt.Errorf("未知类型: %s", kind)
	}
}
