// appearance_darwin.m — 运行时切换 NSApp 外观 / 探测系统深浅色。
// 托盘已由 Wails v3 内置 SystemTray 接管，本文件只保留外观相关的两个 C 函数。
#import <Cocoa/Cocoa.h>

#include "appearance_darwin.h"

int csSystemPrefersDark(void) {
    NSString *style = [[NSUserDefaults standardUserDefaults]
        stringForKey:@"AppleInterfaceStyle"];
    return [style isEqualToString:@"Dark"] ? 1 : 0;
}

void csApplyAppearance(int dark) {
    dispatch_async(dispatch_get_main_queue(), ^{
        if (@available(macOS 10.14, *)) {
            if (dark < 0) {
                [NSApp setAppearance:nil];
                return;
            }
            [NSApp setAppearance:[NSAppearance appearanceNamed:
                dark ? NSAppearanceNameDarkAqua : NSAppearanceNameAqua]];
        }
    });
}
