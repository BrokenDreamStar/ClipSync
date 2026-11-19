#ifndef CLIPSYNC_APPEARANCE_DARWIN_H
#define CLIPSYNC_APPEARANCE_DARWIN_H

#ifdef __cplusplus
extern "C" {
#endif

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
