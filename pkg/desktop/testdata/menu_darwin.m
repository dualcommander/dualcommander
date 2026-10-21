#import <AppKit/AppKit.h>
#include <assert.h>

void removeServicesMenu(NSMenu *appMenu, NSMenu *services);
void clearUnmodifiedMenuShortcut(NSMenuItem *item, NSSet *titles);
void fixUnmodifiedMenuShortcuts(const char *json);

int main(void) {
    @autoreleasepool {
        NSMenu *shortcuts = [[NSMenu alloc] initWithTitle:@"File"];
        NSSet *titles = [NSSet setWithArray:@[@"Edit", @"Copy", @"Move", @"New Folder", @"Delete", @"Terminal"]];
        for (NSString *title in titles) {
            NSMenuItem *item = [shortcuts addItemWithTitle:title action:@selector(tapped:) keyEquivalent:@"\uF70A"];
            assert(item.keyEquivalentModifierMask == NSEventModifierFlagCommand);
        }
        NSMenuItem *rename = [shortcuts addItemWithTitle:@"Rename" action:@selector(tapped:) keyEquivalent:@"\uF709"];
        rename.keyEquivalentModifierMask = NSEventModifierFlagShift;
        NSMenuItem *open = [shortcuts addItemWithTitle:@"Open" action:@selector(tapped:) keyEquivalent:@"o"];
        NSMenuItem *commandF = [shortcuts addItemWithTitle:@"Other" action:@selector(tapped:) keyEquivalent:@"\uF70A"];
        NSMenuItem *root = [[NSMenuItem alloc] initWithTitle:@"Nested" action:nil keyEquivalent:@""];
        root.submenu = shortcuts;
        clearUnmodifiedMenuShortcut(root, titles);
        clearUnmodifiedMenuShortcut(root, titles); // idempotent
        for (NSMenuItem *item in shortcuts.itemArray) {
            if ([titles containsObject:item.title]) assert(item.keyEquivalentModifierMask == 0);
        }
        assert(rename.keyEquivalentModifierMask == NSEventModifierFlagShift);
        assert(open.keyEquivalentModifierMask == NSEventModifierFlagCommand);
        assert(commandF.keyEquivalentModifierMask == NSEventModifierFlagCommand);

        fixUnmodifiedMenuShortcuts("[\"New Folder\"]");
        // Let main-queue registration run, then simulate Fyne rebuilding a menu.
        [NSRunLoop.currentRunLoop runUntilDate:[NSDate dateWithTimeIntervalSinceNow:0.05]];
        NSMenuItem *rebuilt = [shortcuts addItemWithTitle:@"New Folder" action:@selector(tapped:) keyEquivalent:@"\uF70A"];
        assert(rebuilt.keyEquivalentModifierMask == 0);
        [root release];
        [shortcuts release];
        NSMenu *menu = [[NSMenu alloc] initWithTitle:@"App"];
        NSMenu *services = [[NSMenu alloc] initWithTitle:@"Localized services"];
        NSMenuItem *about = [menu addItemWithTitle:@"About" action:nil keyEquivalent:@""];
        NSMenuItem *settings = [menu addItemWithTitle:@"Settings" action:nil keyEquivalent:@","];
        NSMenuItem *serviceItem = [menu addItemWithTitle:@"Localized services" action:nil keyEquivalent:@""];
        serviceItem.submenu = services;
        NSMenuItem *hide = [menu addItemWithTitle:@"Hide" action:nil keyEquivalent:@"h"];
        NSMenuItem *quit = [menu addItemWithTitle:@"Quit" action:nil keyEquivalent:@"q"];
        removeServicesMenu(menu, nil);
        assert(menu.numberOfItems == 5);
        removeServicesMenu(menu, services);
        assert(menu.numberOfItems == 4);
        assert([menu itemAtIndex:0] == about);
        assert([menu itemAtIndex:1] == settings);
        assert([menu itemAtIndex:2] == hide);
        assert([menu itemAtIndex:3] == quit);
        removeServicesMenu(menu, services);
        assert(menu.numberOfItems == 4);
        removeServicesMenu(nil, services);
        [services release];
        [menu release];
    }
    return 0;
}