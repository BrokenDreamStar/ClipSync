#import <Cocoa/Cocoa.h>

#include "tray_darwin.h"

extern void csGoOpenTrampoline(void);
extern void csGoQuitTrampoline(void);

static NSStatusItem *csStatusItem = nil;
static NSMenu *csMenu = nil;

@interface CSTrayTarget : NSObject
- (void)onOpen:(id)sender;
- (void)onQuit:(id)sender;
@end

@implementation CSTrayTarget
- (void)onOpen:(id)sender { csGoOpenTrampoline(); }
- (void)onQuit:(id)sender { csGoQuitTrampoline(); }
@end

static CSTrayTarget *csTarget = nil;

static void csEnsureStatusItem(void) {
    if (csStatusItem != nil) return;
    csStatusItem = [[NSStatusBar systemStatusBar] statusItemWithLength:NSVariableStatusItemLength];
    csMenu = [[NSMenu alloc] init];
    [csMenu setAutoenablesItems:NO];
    [csStatusItem setMenu:csMenu];
    csTarget = [CSTrayTarget new];
}

static void csApplyTray(csTraySetupArgs *args) {
    csEnsureStatusItem();
    if (csStatusItem == nil) return;

    while ([csMenu numberOfItems] > 0) {
        [csMenu removeItemAtIndex:0];
    }
    NSMenuItem *open = [[NSMenuItem alloc]
        initWithTitle:@"打开主窗口"
               action:@selector(onOpen:)
        keyEquivalent:@""];
    [open setTarget:csTarget];
    [csMenu addItem:open];
    [csMenu addItem:[NSMenuItem separatorItem]];
    NSMenuItem *quit = [[NSMenuItem alloc]
        initWithTitle:@"退出 ClipSync"
               action:@selector(onQuit:)
        keyEquivalent:@"q"];
    [quit setTarget:csTarget];
    [csMenu addItem:quit];

    if (csStatusItem.button != nil) {
        // 默认使用 SF Symbol doc.on.clipboard,与迁移前的 trayhelper 行为一致。
        // SF Symbol 自带 template 语义,跟随系统主题自动反色;10.13 不一定有,
        // 加载失败时退回到 args 里的 PNG 字节。
        NSImage *img = [NSImage imageWithSystemSymbolName:@"doc.on.clipboard"
                                  accessibilityDescription:@"ClipSync"];
        if (img == nil && args->iconData != NULL && args->iconLen > 0) {
            NSData *png = [NSData dataWithBytes:args->iconData
                                          length:(NSUInteger)args->iconLen];
            img = [[NSImage alloc] initWithData:png];
        }
        if (img != nil) {
            [img setSize: NSMakeSize(18, 18)];
            [csStatusItem.button setImage: img];
        }
    }
    if (args->tooltip != NULL) {
        [csStatusItem.button setToolTip:[NSString stringWithUTF8String:args->tooltip]];
    }
}

void csSetupTray(const void *iconData, long iconLen,
                 const char *tooltip) {
    csTraySetupArgs *args = malloc(sizeof(csTraySetupArgs));
    args->iconData = iconData;
    args->iconLen = iconLen;
    args->tooltip = tooltip;
    dispatch_async(dispatch_get_main_queue(), ^{
        csApplyTray(args);
        free(args);
    });
}