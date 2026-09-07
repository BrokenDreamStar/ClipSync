//go:build bindings

package main

// wails build / wails dev 生成绑定时，会用 -tags bindings 构建并真实运行
// 本二进制以提取绑定方法。此文件仅在绑定模式下把标记置为 true。
const bindingGen = true
