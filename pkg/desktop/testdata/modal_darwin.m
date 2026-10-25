#import <AppKit/AppKit.h>
#include <assert.h>

void setWindowBlocked(unsigned long window, int blocked);

// No display needed: exercise native activation policy on window-shaped objects.
@interface TestWindow : NSObject
@property BOOL ignoresMouseEvents;
@end
@implementation TestWindow
- (BOOL)canBecomeKeyWindow { return YES; }
- (BOOL)canBecomeMainWindow { return YES; }
- (BOOL)isKeyWindow { return NO; }
- (BOOL)isMainWindow { return NO; }
@end

static void drain(void) {
    [NSRunLoop.currentRunLoop runUntilDate:[NSDate dateWithTimeIntervalSinceNow:0.01]];
}

int main(void) {
    @autoreleasepool {
        TestWindow *parent = [TestWindow new];
        TestWindow *other = [TestWindow new];
        assert(parent.canBecomeKeyWindow && parent.canBecomeMainWindow);
        for (int i = 0; i < 2; i++) {
            setWindowBlocked((unsigned long)parent, 1);
            drain();
            assert(!parent.canBecomeKeyWindow && !parent.canBecomeMainWindow);
            assert(!parent.ignoresMouseEvents); // Blocked windows must absorb clicks, not expose apps underneath.
            assert(other.canBecomeKeyWindow && other.canBecomeMainWindow);
            assert(!other.ignoresMouseEvents);
            setWindowBlocked((unsigned long)parent, 0);
            drain();
            assert(parent.canBecomeKeyWindow && parent.canBecomeMainWindow);
            assert(!parent.ignoresMouseEvents);
        }
        [parent release];
        [other release];
    }
    return 0;
}