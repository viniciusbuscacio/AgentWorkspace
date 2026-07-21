//go:build darwin

#import <Cocoa/Cocoa.h>
#import <WebKit/WebKit.h>
#import <CoreGraphics/CoreGraphics.h>
#include <string.h>
#include <stdlib.h>
#include <dlfcn.h>

// CGWindowListCreateImage was obsoleted in the macOS 15 SDK headers (Apple
// points to ScreenCaptureKit) but the symbol still ships and works in the
// CoreGraphics dylib. We resolve it via dlsym to dodge the header's
// unavailability while staying functional; if it ever disappears, the pointer
// is NULL and the Go caller falls back to the takeSnapshot path.
typedef CGImageRef (*aw_window_image_fn)(CGRect, uint32_t, uint32_t, uint32_t);

static aw_window_image_fn aw_window_image_resolver(void) {
    static aw_window_image_fn fn = NULL;
    static dispatch_once_t once;
    dispatch_once(&once, ^{
        void *h = dlopen("/System/Library/Frameworks/CoreGraphics.framework/CoreGraphics", RTLD_LAZY);
        if (h) {
            fn = (aw_window_image_fn)dlsym(h, "CGWindowListCreateImage");
        }
    });
    return fn;
}

// Option bits are stable ABI values from CGWindow.h.
enum {
    aw_kCGWindowListOptionIncludingWindow = (1 << 3),
    aw_kCGWindowImageBoundsIgnoreFraming = (1 << 0),
};

// Encodes a CGImage as PNG into a malloc'd buffer (caller frees). Returns NULL
// on failure; *outLen receives the length.
static unsigned char *aw_cgimage_to_png(CGImageRef cg, int *outLen) {
    if (!cg) {
        return NULL;
    }
    NSData *png = nil;
    @autoreleasepool {
        @try {
            NSBitmapImageRep *rep = [[NSBitmapImageRep alloc] initWithCGImage:cg];
            png = [rep representationUsingType:NSBitmapImageFileTypePNG properties:@{}];
        } @catch (NSException *ex) {
            png = nil;
        }
    }
    if (!png || png.length == 0) {
        return NULL;
    }
    NSUInteger len = png.length;
    unsigned char *buf = (unsigned char *)malloc(len);
    if (!buf) {
        return NULL;
    }
    memcpy(buf, png.bytes, len);
    if (outLen) {
        *outLen = (int)len;
    }
    return buf;
}

static WKWebView *aw_find_webview(NSView *view) {
    if ([view isKindOfClass:[WKWebView class]]) {
        return (WKWebView *)view;
    }
    for (NSView *sub in view.subviews) {
        WKWebView *found = aw_find_webview(sub);
        if (found) {
            return found;
        }
    }
    return nil;
}

// aw_capture_window_png captures the app's window as it is actually composited
// on screen via CGWindowListCreateImage. Unlike -takeSnapshot it includes
// GPU-compositor effects — notably CSS backdrop-filter (the wallpaper "glass"
// blur). It needs Screen Recording permission; when that is denied the capture
// comes back empty and the Go caller falls back to the takeSnapshot path.
unsigned char *aw_capture_window_png(int *outLen) {
    if (outLen) {
        *outLen = 0;
    }
    __block CGWindowID windowID = 0;

    void (^findWin)(void) = ^{
        @autoreleasepool {
            @try {
                for (NSWindow *win in [NSApp windows]) {
                    if (!win.isVisible) {
                        continue;
                    }
                    if (aw_find_webview(win.contentView)) {
                        windowID = (CGWindowID)win.windowNumber;
                        break;
                    }
                }
            } @catch (NSException *ex) {
                windowID = 0;
            }
        }
    };

    if ([NSThread isMainThread]) {
        findWin();
    } else {
        dispatch_semaphore_t sem = dispatch_semaphore_create(0);
        dispatch_async(dispatch_get_main_queue(), ^{
            findWin();
            dispatch_semaphore_signal(sem);
        });
        if (dispatch_semaphore_wait(sem, dispatch_time(DISPATCH_TIME_NOW, (int64_t)(5 * NSEC_PER_SEC))) != 0) {
            return NULL;
        }
    }
    if (windowID == 0) {
        return NULL;
    }

    aw_window_image_fn createImage = aw_window_image_resolver();
    if (!createImage) {
        return NULL;
    }
    CGImageRef img = createImage(
        CGRectNull,
        aw_kCGWindowListOptionIncludingWindow,
        (uint32_t)windowID,
        aw_kCGWindowImageBoundsIgnoreFraming);
    if (!img) {
        return NULL;
    }
    if (CGImageGetWidth(img) == 0 || CGImageGetHeight(img) == 0) {
        CGImageRelease(img);
        return NULL;
    }
    unsigned char *buf = aw_cgimage_to_png(img, outLen);
    CGImageRelease(img);
    return buf;
}

// Captures the app's WKWebView content as PNG. Returns a malloc'd buffer the
// Go caller frees, or NULL on failure. Renders the live WebKit layer, so CSS
// backdrop-filter (the wallpaper glass) is preserved.
unsigned char *aw_capture_webview_png(int *outLen) {
    if (outLen) {
        *outLen = 0;
    }
    __block NSData *png = nil;
    dispatch_semaphore_t sem = dispatch_semaphore_create(0);

    void (^work)(void) = ^{
        @autoreleasepool {
            @try {
                WKWebView *web = nil;
                for (NSWindow *win in [NSApp windows]) {
                    if (!win.isVisible) {
                        continue;
                    }
                    web = aw_find_webview(win.contentView);
                    if (web) {
                        break;
                    }
                }
                if (!web) {
                    dispatch_semaphore_signal(sem);
                    return;
                }
                WKSnapshotConfiguration *cfg = [[WKSnapshotConfiguration alloc] init];
                cfg.afterScreenUpdates = NO;
                [web takeSnapshotWithConfiguration:cfg completionHandler:^(NSImage *image, NSError *error) {
                    @autoreleasepool {
                        @try {
                            if (image) {
                                CGImageRef cg = [image CGImageForProposedRect:NULL context:nil hints:nil];
                                if (cg) {
                                    NSBitmapImageRep *rep = [[NSBitmapImageRep alloc] initWithCGImage:cg];
                                    png = [rep representationUsingType:NSBitmapImageFileTypePNG properties:@{}];
                                }
                            }
                        } @catch (NSException *ex) {
                            png = nil;
                        }
                        dispatch_semaphore_signal(sem);
                    }
                }];
            } @catch (NSException *ex) {
                dispatch_semaphore_signal(sem);
            }
        }
    };

    // takeSnapshot's completion runs on the main queue, so we must NOT be on the
    // main thread when we block waiting for it. The agent tool path runs on a
    // background goroutine, satisfying this.
    if ([NSThread isMainThread]) {
        // Avoid a deadlock; the caller contract is to run off-main.
        return NULL;
    }
    dispatch_async(dispatch_get_main_queue(), work);
    // Bounded wait: if the snapshot stalls, fall back to the frontend path
    // instead of hanging the agent tool call. Only read png when the snapshot
    // actually signaled completion (return value 0); on timeout png may still
    // be written concurrently, so we treat it as a miss.
    long timedOut = dispatch_semaphore_wait(sem, dispatch_time(DISPATCH_TIME_NOW, (int64_t)(5 * NSEC_PER_SEC)));
    if (timedOut != 0) {
        return NULL;
    }
    if (!png || png.length == 0) {
        return NULL;
    }
    NSUInteger len = png.length;
    unsigned char *buf = (unsigned char *)malloc(len);
    if (!buf) {
        return NULL;
    }
    memcpy(buf, png.bytes, len);
    if (outLen) {
        *outLen = (int)len;
    }
    return buf;
}
