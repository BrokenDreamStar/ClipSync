#ifndef CLIPSYNC_ACTIVATION_DARWIN_H
#define CLIPSYNC_ACTIVATION_DARWIN_H

#ifdef __cplusplus
extern "C" {
#endif

// csSetActivationPolicy 切换 NSApp activation policy：regular 非 0 →
// Regular（进 Dock / Cmd+Tab），0 → Accessory。已是目标 policy 时不做任何
// 事（不触发 LS/Dock 重新注册）。同步执行：主线程直接调用，其它线程
// dispatch_sync 到主队列，返回时已生效（保证与后续 Show/Focus 的顺序）。
void csSetActivationPolicy(int regular);

#ifdef __cplusplus
}
#endif

#endif
