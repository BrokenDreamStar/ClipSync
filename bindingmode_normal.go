//go:build !bindings

package main

// 正常构建（无 bindings 标签）时为 false，见 bindingmode_bindings.go。
const bindingGen = false
