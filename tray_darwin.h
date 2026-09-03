#ifndef CLIPSYNC_TRAY_DARWIN_H
#define CLIPSYNC_TRAY_DARWIN_H

#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

typedef struct {
    const void *iconData;
    long        iconLen;
    const char *tooltip;
} csTraySetupArgs;

// csSetupTray 一次性把 NSStatusItem 建出来,并设置菜单/图标/tooltip。
// 所有 Cocoa 调用都通过 dispatch_async(dispatch_get_main_queue(), ...)
// 派发到主线程;调用方无需在主线程。需要在 NSApp 已初始化之后调用（典型: Wails OnStartup）。
void csSetupTray(const void *iconData, long iconLen,
                 const char *tooltip);

void csRebuildMenu(void);
void csSetIcon(const void *data, long len);
void csSetTooltip(const char *t);

// csSystemPrefersDark 读取系统外观是否为深色（AppleInterfaceStyle 仅在深色下存在）。
// 线程安全，可在任意线程调用。
int csSystemPrefersDark(void);

// csApplyAppearance 把 NSApp 外观设为深色/浅色；dark<0 表示清除强制外观、跟随系统。
// 内部派发到主队列，可在任意线程调用。
void csApplyAppearance(int dark);

#ifdef __cplusplus
}
#endif

#endif