//go:build darwin && cgo

#import <AppKit/AppKit.h>

void clearUnmodifiedMenuShortcut(NSMenuItem *item, NSSet *titles) {
    if (item.action == @selector(tapped:) && item.keyEquivalent.length &&
        [titles containsObject:item.title]) {
        item.keyEquivalentModifierMask = 0;
    }
    for (NSMenuItem *child in item.submenu.itemArray) {
        clearUnmodifiedMenuShortcut(child, titles);
    }
}

void fixUnmodifiedMenuShortcuts(const char *json) {
    NSData *data = [[NSString stringWithUTF8String:json] dataUsingEncoding:NSUTF8StringEncoding];
    NSArray *labels = [NSJSONSerialization JSONObjectWithData:data options:0 error:nil];
    NSSet *titles = [NSSet setWithArray:labels ?: @[]];
    dispatch_async(dispatch_get_main_queue(), ^{
        // ponytail: Fyne supplies no native menu hook; observe insertions until upstream fixes zero masks.
        static id observer;
        if (observer) [NSNotificationCenter.defaultCenter removeObserver:observer];
        observer = [NSNotificationCenter.defaultCenter
            addObserverForName:NSMenuDidAddItemNotification object:nil queue:nil
            usingBlock:^(NSNotification *note) {
                NSMenu *menu = note.object;
                NSInteger index = [note.userInfo[@"NSMenuItemIndex"] integerValue];
                clearUnmodifiedMenuShortcut([menu itemAtIndex:index], titles);
            }];
        for (NSMenuItem *item in NSApplication.sharedApplication.mainMenu.itemArray) {
            clearUnmodifiedMenuShortcut(item, titles);
        }
    });
}

void removeServicesMenu(NSMenu *appMenu, NSMenu *services) {
    if (!services) return;
    for (NSMenuItem *item in appMenu.itemArray) {
        if (item.submenu == services) {
            [appMenu removeItem:item];
            return;
        }
    }
}

void hideServicesMenu(void) {
    dispatch_async(dispatch_get_main_queue(), ^{
        NSApplication *app = NSApplication.sharedApplication;
        removeServicesMenu(app.mainMenu.itemArray.firstObject.submenu, app.servicesMenu);
        app.servicesMenu = nil;
    });
}