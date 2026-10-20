//go:build darwin && cgo

#import <AppKit/AppKit.h>
#import <UniformTypeIdentifiers/UniformTypeIdentifiers.h>
#include <string.h>

@interface DCContextTarget : NSObject
@property NSInteger selected;
- (void)choose:(NSMenuItem *)sender;
@end

char *chooseApplication(unsigned long handle) {
    __block char *chosen = NULL;
    dispatch_sync(dispatch_get_main_queue(), ^{
        @autoreleasepool {
            NSWindow *window = (NSWindow *)handle;
            if (!window.visible) return;
            NSOpenPanel *panel = [NSOpenPanel openPanel];
            panel.title = @"Choose Application";
            panel.prompt = @"Open";
            panel.message = @"Choose an application to open this item.";
            panel.directoryURL = [NSURL fileURLWithPath:@"/Applications" isDirectory:YES];
            panel.canChooseFiles = YES;
            panel.canChooseDirectories = NO;
            panel.allowsMultipleSelection = NO;
            panel.treatsFilePackagesAsDirectories = NO;
            panel.allowedContentTypes = @[UTTypeApplicationBundle];
            // ponytail: browse application bundles, not Finder's suggested-app list.
            if ([panel runModal] == NSModalResponseOK) {
                NSBundle *bundle = [NSBundle bundleWithURL:panel.URL];
                if (bundle.executableURL) chosen = strdup(panel.URL.path.fileSystemRepresentation);
            }
            [window makeKeyAndOrderFront:nil];
        }
    });
    return chosen;
}

@implementation DCContextTarget
- (void)choose:(NSMenuItem *)sender { self.selected = sender.tag; }
@end

int showContextMenu(unsigned long handle, const char *json, float x, float y) {
    __block int selected = -1;
    dispatch_sync(dispatch_get_main_queue(), ^{
        @autoreleasepool {
            NSWindow *window = (NSWindow *)handle;
            NSView *view = window.contentView;
            if (!view || !window.visible) return;
            NSArray *items = [NSJSONSerialization JSONObjectWithData:
                [[NSString stringWithUTF8String:json] dataUsingEncoding:NSUTF8StringEncoding]
                options:0 error:nil];
            NSMenu *menu = [[NSMenu alloc] initWithTitle:@""];
            menu.autoenablesItems = NO;
            DCContextTarget *target = [[DCContextTarget alloc] init];
            target.selected = -1;
            NSInteger index = 0;
            for (NSDictionary *entry in items) {
                NSString *title = entry[@"Title"];
                if (!title.length) {
                    [menu addItem:NSMenuItem.separatorItem];
                } else {
                    NSMenuItem *item = [[NSMenuItem alloc] initWithTitle:title action:@selector(choose:) keyEquivalent:@""];
                    item.target = target;
                    item.tag = index;
                    item.enabled = ![entry[@"Disabled"] boolValue];
                    [menu addItem:item];
                    [item release];
                }
                index++;
            }
            NSPoint point = NSMakePoint(x, view.isFlipped ? y : view.bounds.size.height-y);
            [menu popUpMenuPositioningItem:nil atLocation:point inView:view];
            selected = (int)target.selected;
            [menu release];
            [target release];
        }
    });
    return selected;
}