//go:build darwin && cgo

#import <AppKit/AppKit.h>

@interface DCFileDragSource : NSObject <NSDraggingSource>
@end

@implementation DCFileDragSource
- (NSDragOperation)draggingSession:(NSDraggingSession *)session
    sourceOperationMaskForDraggingContext:(NSDraggingContext)context {
    return NSDragOperationCopy;
}
- (BOOL)ignoreModifierKeysForDraggingSession:(NSDraggingSession *)session {
    return YES;
}
@end

int startFileDrag(unsigned long handle, const char *json) {
    @autoreleasepool {
        NSWindow *window = (NSWindow *)handle;
        NSEvent *event = NSApp.currentEvent;
        if (!window || !event || event.window != window ||
            event.type != NSEventTypeLeftMouseDragged ||
            !(NSEvent.pressedMouseButtons & 1)) return 0;
        NSData *data = [[NSString stringWithUTF8String:json] dataUsingEncoding:NSUTF8StringEncoding];
        NSArray *paths = [NSJSONSerialization JSONObjectWithData:data options:0 error:nil];
        if (![paths isKindOfClass:NSArray.class] || !paths.count) return 0;
        NSMutableArray *items = [NSMutableArray arrayWithCapacity:paths.count];
        NSView *view = window.contentView;
        NSPoint point = [view convertPoint:event.locationInWindow fromView:nil];
        for (id path in paths) {
            if (![path isKindOfClass:NSString.class] || ![path isAbsolutePath] ||
                [path rangeOfString:@"\0"].location != NSNotFound) return 0;
            NSDraggingItem *item = [[NSDraggingItem alloc] initWithPasteboardWriter:[NSURL fileURLWithPath:path]];
            // Reuse system document icon; never scan/read dragged files on UI thread.
            NSImage *image = [NSImage imageNamed:NSImageNameMultipleDocuments];
            [item setDraggingFrame:NSMakeRect(point.x, point.y, 32, 32) contents:image];
            [items addObject:item];
            [item release];
        }
        static DCFileDragSource *source;
        if (!source) source = [DCFileDragSource new];
        return [view beginDraggingSessionWithItems:items event:event source:source] != nil;
    }
}