//go:build darwin && cgo

#import <AppKit/AppKit.h>
#include <string.h>

char *readFileURLs(void) {
    @autoreleasepool {
        NSArray *urls = [NSPasteboard.generalPasteboard readObjectsForClasses:@[NSURL.class]
            options:@{NSPasteboardURLReadingFileURLsOnlyKey: @YES}];
        NSMutableArray *paths = [NSMutableArray array];
        for (NSURL *url in urls) {
            if (url.isFileURL && url.path != nil) [paths addObject:url.path];
        }
        NSData *data = [NSJSONSerialization dataWithJSONObject:paths options:0 error:nil];
        if (data == nil) return NULL;
        return strdup([[NSString alloc] initWithData:data encoding:NSUTF8StringEncoding].UTF8String);
    }
}

int copyFileURLs(const char *json) {
    @autoreleasepool {
        NSData *data = [[NSString stringWithUTF8String:json] dataUsingEncoding:NSUTF8StringEncoding];
        NSArray *paths = [NSJSONSerialization JSONObjectWithData:data options:0 error:nil];
        if (![paths isKindOfClass:NSArray.class] || paths.count == 0) return 0;
        NSMutableArray *urls = [NSMutableArray arrayWithCapacity:paths.count];
        for (id path in paths) {
            if (![path isKindOfClass:NSString.class] || ![path isAbsolutePath] ||
                [path rangeOfString:@"\0"].location != NSNotFound) return 0;
            [urls addObject:[NSURL fileURLWithPath:path]];
        }
        NSPasteboard *pasteboard = NSPasteboard.generalPasteboard;
        [pasteboard clearContents];
        return [pasteboard writeObjects:urls];
    }
}