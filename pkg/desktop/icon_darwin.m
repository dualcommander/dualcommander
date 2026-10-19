//go:build darwin && cgo

#import <AppKit/AppKit.h>

int setAppIcon(const void *bytes, size_t length) {
    @autoreleasepool {
        NSData *data = [NSData dataWithBytes:bytes length:length];
        NSImage *image = [[NSImage alloc] initWithData:data];
        if (!image) return 0;
        // AppKit must run on the main thread; retain image until queued work runs.
        dispatch_async(dispatch_get_main_queue(), ^{
            [NSApplication sharedApplication].applicationIconImage = image;
            [image release];
        });
        return 1;
    }
}