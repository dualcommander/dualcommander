//go:build darwin && cgo

#import <AppKit/AppKit.h>
#import <UniformTypeIdentifiers/UniformTypeIdentifiers.h>
#import <QuickLookThumbnailing/QuickLookThumbnailing.h>
#include <stdlib.h>
#include <string.h>

void *filePreview(const char *path, size_t *length) {
    @autoreleasepool {
        NSString *name = @(path);
        if (![[NSFileManager defaultManager] fileExistsAtPath:name]) return NULL;
        NSURL *url = [NSURL fileURLWithPath:name];
        QLThumbnailGenerationRequest *request = [[QLThumbnailGenerationRequest alloc]
            initWithFileAtURL:url size:CGSizeMake(64, 64) scale:1
            representationTypes:QLThumbnailGenerationRequestRepresentationTypeAll];
        dispatch_semaphore_t done = dispatch_semaphore_create(0);
        __block CGImageRef image = NULL;
        [[QLThumbnailGenerator sharedGenerator] generateBestRepresentationForRequest:request
            completionHandler:^(QLThumbnailRepresentation *thumbnail, NSError *error) {
                if (thumbnail.CGImage) image = CGImageRetain(thumbnail.CGImage);
                dispatch_semaphore_signal(done);
            }];
        // Called only on background workers, never the app's main thread.
        dispatch_semaphore_wait(done, DISPATCH_TIME_FOREVER);
        dispatch_release(done);
        [request release];
        NSBitmapImageRep *bitmap = nil;
        if (image) {
            bitmap = [[NSBitmapImageRep alloc] initWithCGImage:image];
            CGImageRelease(image);
        } else {
            NSImage *icon = [[NSWorkspace sharedWorkspace] iconForFile:name];
            NSRect rect = NSMakeRect(0, 0, 64, 64);
            CGImageRef fallback = [icon CGImageForProposedRect:&rect context:nil hints:nil];
            if (fallback) bitmap = [[NSBitmapImageRep alloc] initWithCGImage:fallback];
        }
        if (!bitmap) return NULL;
        NSData *png = [bitmap representationUsingType:NSBitmapImageFileTypePNG properties:@{}];
        *length = png.length;
        void *result = malloc(*length);
        if (result) memcpy(result, png.bytes, *length);
        else *length = 0;
        [bitmap release];
        return result;
    }
}

void *fileIcon(const char *extension, int folder, size_t *length) {
    @autoreleasepool {
        UTType *type = folder ? UTTypeFolder : [UTType typeWithFilenameExtension:@(extension)];
        NSImage *icon = [[NSWorkspace sharedWorkspace] iconForContentType:type ?: UTTypeData];
        NSRect rect = NSMakeRect(0, 0, 32, 32);
        CGImageRef image = [icon CGImageForProposedRect:&rect context:nil hints:nil];
        if (!image) return NULL;
        NSBitmapImageRep *bitmap = [[NSBitmapImageRep alloc] initWithCGImage:image];
        NSData *png = [bitmap representationUsingType:NSBitmapImageFileTypePNG properties:@{}];
        *length = png.length;
        void *result = malloc(*length);
        if (result) memcpy(result, png.bytes, *length);
        else *length = 0;
        [bitmap release];
        return result;
    }
}