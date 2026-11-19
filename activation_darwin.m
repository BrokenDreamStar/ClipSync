// activation_darwin.m — 按主窗口可见性动态切换 activation policy。
//
// 设计：本应用是纯托盘应用，Info.plist 声明 LSUIElement（进程出生即
// UIElement，不进 Dock / Cmd+Tab——托盘常驻形态，也天然规避了 v0.1.2 上
// "运行中被系统翻成 Foreground 后关窗仍赖在 Cmd+Tab"的个案）。主窗口打开
// 期间按常规应用呈现（用户要求：开窗后能被 Cmd+Tab 切回；代价是 Dock 里
// 会短暂出现图标，这是公开 API 下两者绑定的必然），关窗即切回 Accessory
// 消失。setActivationPolicy 是 Apple 文档支持的 LSUIElement 应用动态显隐
// Dock 图标的标准途径。
#import <Cocoa/Cocoa.h>

#include "activation_darwin.h"

void csSetActivationPolicy(int regular) {
    NSApplicationActivationPolicy target = regular
        ? NSApplicationActivationPolicyRegular
        : NSApplicationActivationPolicyAccessory;
    void (^apply)(void) = ^{
        if ([NSApp activationPolicy] != target) {
            [NSApp setActivationPolicy:target];
        }
    };
    // 主线程（Wails 事件循环的钩子回调）直接执行；其它线程同步等主队列，
    // 避免"policy 还没切、窗口已经 Show"的时序反转。
    if ([NSThread isMainThread]) {
        apply();
    } else {
        dispatch_sync(dispatch_get_main_queue(), apply);
    }
}
