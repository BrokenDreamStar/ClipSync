//go:build darwin

package main

/*
#cgo CFLAGS: -x objective-c -fobjc-arc -I${SRCDIR}
#cgo LDFLAGS: -framework Cocoa
#include "activation_darwin.h"
*/
import "C"

// setAppRegular 切换应用的 Dock / Cmd+Tab 呈现：主窗口打开期间 Regular
// （可被 Cmd+Tab 切回），关闭后回 Accessory。同步生效，见 activation_darwin.m。
func setAppRegular(regular bool) {
	p := 0
	if regular {
		p = 1
	}
	C.csSetActivationPolicy(C.int(p))
}
