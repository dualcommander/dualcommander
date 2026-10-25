//go:build darwin && cgo

#import <AppKit/AppKit.h>
#import <objc/runtime.h>

static char blockedKey;

// GLFW subclasses NSWindow. Preserve its focus policy for every unblocked window.
static void guardActivation(Class cls, SEL selector) {
    Method method = class_getInstanceMethod(cls, selector);
    IMP original = method_getImplementation(method);
    IMP guarded = imp_implementationWithBlock(^BOOL(NSWindow *window) {
        if ([objc_getAssociatedObject(window, &blockedKey) boolValue]) return NO;
        return ((BOOL (*)(id, SEL))original)(window, selector);
    });
    class_replaceMethod(cls, selector, guarded, method_getTypeEncoding(method));
}

void setWindowBlocked(unsigned long handle, int blocked) {
    // RunNative supplies handles but does not marshal calls onto AppKit's main thread.
    dispatch_async(dispatch_get_main_queue(), ^{
        NSWindow *window = (NSWindow *)handle;
        static NSMutableSet *guardedClasses;
        if (!guardedClasses) guardedClasses = [NSMutableSet new];
        Class cls = window.class;
        NSString *name = NSStringFromClass(cls);
        if (![guardedClasses containsObject:name]) {
            guardActivation(cls, @selector(canBecomeKeyWindow));
            guardActivation(cls, @selector(canBecomeMainWindow));
            [guardedClasses addObject:name];
        }
        objc_setAssociatedObject(window, &blockedKey, @(blocked != 0), OBJC_ASSOCIATION_RETAIN_NONATOMIC);
        window.ignoresMouseEvents = blocked != 0;
        if (blocked && window.keyWindow) [window resignKeyWindow];
        if (blocked && window.mainWindow) [window resignMainWindow];
    });
}