#import <Foundation/Foundation.h>
#import <CoreVideo/CoreVideo.h>
#import <AppKit/AppKit.h>

#include <string.h>

BOOL forceScreenRecording = true;

struct WinInfo {
    long wid;
    long pid;
    NSString *title;
    CGRect rect;
    int scale;
};

NSString *removeNonASCIIAndTruncate(NSString *str) {
    NSMutableString *s = [NSMutableString stringWithCapacity:str.length];
    for (NSUInteger i = 0; i < str.length; ++i) {
        unichar c = [str characterAtIndex:i];
        if (c >= 32 && c <= 127) {
            [s appendFormat:@"%C", c];
        }
    }
    NSRange range = [s rangeOfString:@" - "];
    if (range.length != 0) {
        s = (NSMutableString *)[s substringToIndex:range.location];
    }
    return s;
}

int canControlComputer(int prompt) {
    NSDictionary *options = @{(id)kAXTrustedCheckOptionPrompt: @NO};
    if (prompt) {
        options = @{(id)kAXTrustedCheckOptionPrompt: @YES};
    }
    if (AXIsProcessTrustedWithOptions((CFDictionaryRef)options)) {
        return 1;
    }
    return 0;
}

int getWindowCountForPID(long pid) {
    AXUIElementRef appElementRef = AXUIElementCreateApplication(pid);
    //  NSLog(@"getWindowCountForPID: %ld %p\n", pid, appElementRef);
    CFArrayRef windowArray = nil;
    AXUIElementCopyAttributeValue(appElementRef, kAXWindowsAttribute, (CFTypeRef*)&windowArray);
    if (windowArray == nil) {
        return -1;
    }
    CFIndex count = CFArrayGetCount(windowArray);
    return (int)count;
}

// also: https://stackoverflow.com/questions/56597221/detecting-screen-recording-settings-on-macos-catalina/58991936#58991936
int canRecordScreen() {
    if (@available(macOS 10.15, *)) {
        CGDisplayStreamRef stream = CGDisplayStreamCreate(CGMainDisplayID(), 1, 1, kCVPixelFormatType_32BGRA, nil, ^(CGDisplayStreamFrameStatus status, uint64_t displayTime, IOSurfaceRef frameSurface, CGDisplayStreamUpdateRef updateRef) {
        });        
        int can = 0;
        if (stream != NULL) {
            can = 1;
        }
        // NSLog(@"NSCanRecord: %d", can);
        if (stream) {
            CFRelease(stream);
        }
        return can;
    } 
    return 1;
}

NSScreen *getBestScreenForBounds(CGRect bounds) {
    NSScreen *bestScreen = nil;        
    CGFloat bestArea = 0.0;
    NSArray *screens = [NSScreen screens];
    for (NSScreen *s in screens) {
        // NSLog(@"screen: %f %f\n", s.frame.size.width, s.backingScaleFactor);
        CGRect inter = CGRectIntersection(s.frame, bounds);
        CGFloat a = inter.size.width * inter.size.height;
        if (a > bestArea) {
            bestArea = a;
            bestScreen = s;
        }
    }
    if (bestScreen != nil) {
        CFRetain(bestScreen);
    }
    CFRelease(screens);
    return bestScreen;
}

const char *getWindowIDs(struct WinInfo *find, BOOL debug, BOOL(*gotWin)(struct WinInfo *find, struct WinInfo w)) {
    if (forceScreenRecording) {
        if (!canRecordScreen()) {
            NSLog(@"Can't record screen");
            return "can't record screen";
        }
        forceScreenRecording = false;
    }
    CFArrayRef windowList = CGWindowListCopyWindowInfo(kCGWindowListOptionOnScreenOnly, kCGNullWindowID);
    for (NSMutableDictionary* entry in (__bridge NSArray*)windowList)
    {
        struct WinInfo w;
        w.title = [entry objectForKey:(id)kCGWindowName];
        w.title = removeNonASCIIAndTruncate(w.title);
        if (debug) {
            NSLog(@"Win: %@\n", w.title);
        }
        w.pid = (long)[[entry objectForKey:(id)kCGWindowOwnerPID] integerValue];
        w.wid = (long)[[entry objectForKey:(id)kCGWindowNumber] integerValue];

        NSDictionary *dict = [entry objectForKey:(id)kCGWindowBounds];
        w.rect.origin.x = (CGFloat)[(NSNumber *)dict[@"X"] floatValue];
        w.rect.origin.y = (CGFloat)[(NSNumber *)dict[@"Y"] floatValue];
        w.rect.size.width = (CGFloat)[(NSNumber *)dict[@"Width"] floatValue];
        w.rect.size.height = (CGFloat)[(NSNumber *)dict[@"Height"] floatValue];
        // NSLog(@"Size: %@ %g %g %g %g", w.title, (float)w.rect.origin.x, (float)w.rect.origin.y, (float)w.rect.size.width, (float)w.rect.size.height);
        if (gotWin(find, w)) {
            CFRelease(windowList);
            return "";
        }
    }
    CFRelease(windowList);
    return "window not found";
}

BOOL findTitle(struct WinInfo *find, struct WinInfo w) {
    if ([w.title compare: find->title] == NSOrderedSame) {
        *find = w;
        // NSLog(@"scale:%d bounds: %f,%f %fx%f\n", find->scale, bounds.origin.x, bounds.origin.y, bounds.size.width, bounds.size.height);
        return YES;
    }
    return NO;
}

typedef struct WinIDInfo {
    long       winID;
    int        scale;
    const char *err;
    int        x;
    int        y;
    int        w;
    int        h;
} WinIDInfo;

WinIDInfo WindowGetIDScaleAndRectForTitle(const char *title, long pid) {
    // NSLog(@"WindowGetIDAndScaleForTitle1\n");
    struct WinInfo find;
    struct WinIDInfo got;    
    find.wid = 0;
    find.scale = 0;
    find.title = [NSString stringWithUTF8String: title];
    find.pid = pid;
    got.err = getWindowIDs(&find, NO, findTitle);
    if (got.err != nil && strlen(got.err) > 0) {
        NSLog(@"getwin err: %s\n", got.err);
        return got;
    }
    // NSLog(@"got win: %@ %g %g %g %g", find.title, (float)find.rect.origin.x, (float)find.rect.origin.y, (float)find.rect.size.width, (float)find.rect.size.height);
    got.scale = getBestScreenForBounds(find.rect).backingScaleFactor;
    got.winID = find.wid;
    got.x = find.rect.origin.x;
    got.y = find.rect.origin.y;
    got.w = find.rect.size.width;
    got.h = find.rect.size.height;
    CFRelease(find.title);
    return got;
}

AXUIElementRef getAXElementOfWindowForTitle(const char *title, long pid, BOOL debug) {
    NSString *nsTitle = [NSString stringWithUTF8String: title];
    AXUIElementRef appElementRef = AXUIElementCreateApplication(pid);
    //   NSLog(@"getAXElementOfWindowForTitle: %s %ld %p\n", title, pid, appElementRef);
    CFArrayRef windowArray = nil;
    AXUIElementCopyAttributeValue(appElementRef, kAXWindowsAttribute, (CFTypeRef*)&windowArray);
    if (windowArray == nil) {
        NSLog(@"getAXElementOfWindowForTitle is nil: %s %ld\n", title, pid);
        CFRelease(appElementRef);
        return nil;
    }
    AXUIElementRef matchinWinRef = nil;
    CFIndex nItems = CFArrayGetCount(windowArray);
    // NSLog(@"getAXElementOfWindowForTitle: %s %p %d\n", title, windowArray, (int)nItems);
    for (int i = 0; i < nItems; i++) {
        AXUIElementRef winRef = (AXUIElementRef) CFArrayGetValueAtIndex(windowArray, i);
        NSString *winTitle = nil;
        AXUIElementCopyAttributeValue(winRef, kAXTitleAttribute, (CFTypeRef *)&winTitle);
        if (winTitle == nil) {
            // NSLog(@"Win: <nil-title> # %@\n", nsTitle);
            continue;
        }
        winTitle = removeNonASCIIAndTruncate(winTitle);
       if (debug) {
            NSLog(@"Win1: '%@' %d\n", winTitle, (int)[winTitle length]);
            // NSLog(@"Win2: '%@' %d %lu\n", nsTitle, (int)[nsTitle length], strlen(title));
       }
        if ([winTitle compare:nsTitle] == NSOrderedSame) {
            matchinWinRef = winRef;
            CFRetain(matchinWinRef);
            CFRelease(winTitle);
            break;
        }
        CFRelease(winTitle);
    }
    CFRelease(appElementRef);
    CFRelease(windowArray);
    CFRelease(nsTitle);
    return matchinWinRef;
}

int CloseWindowForTitle(const char *title, long pid) {
    AXUIElementRef winRef = getAXElementOfWindowForTitle(title, pid, false);
    // if (winRef == 0) {
    //     getAXElementOfWindowForTitle(title, pid, true);
    // }
    //  NSLog(@"CloseWindowForTitle1 %s %p\n", title, winRef);
    if (winRef == nil) {
        return 0;
    }
    AXUIElementRef buttonRef = nil;
    AXUIElementCopyAttributeValue(winRef, kAXCloseButtonAttribute, (CFTypeRef*)&buttonRef);
    AXUIElementPerformAction(buttonRef, kAXPressAction);
    CFRelease(winRef);
    CFRelease(buttonRef);
    return 1;
}

int ActivateWindowForTitle(const char *title, long pid) {
    NSRunningApplication* app = [NSRunningApplication runningApplicationWithProcessIdentifier: pid];
    [app activateWithOptions: NSApplicationActivateIgnoringOtherApps];

    AXUIElementRef winRef = getAXElementOfWindowForTitle(title, pid, false);
    if (winRef == nil) {
        NSLog(@"ActivateWindowForTitle: no window for %s %ld\n", title, pid);
        return 0;
    }
    AXError err = AXUIElementPerformAction(winRef, kAXRaiseAction);
    if (err != 0) {
        NSLog(@"ActivateWindowForTitle error: %s %d\n", title, err);
        return 0;
    }
    CFRelease(winRef);
    return 1;
}

int SetWindowRectForTitle(const char *title, long pid, int x, int y, int w, int h) {
    // NSLog(@"PlaceWindowForTitle %s %ld\n", title, pid);
    AXUIElementRef winRef = getAXElementOfWindowForTitle(title, pid, NO);
    if (winRef == nil) {
        NSLog(@"PlaceWindowForTitle no window for %s %ld\n", title, pid);
        return 0;
    }
    NSSize winSize;
    winSize.width = w;
    winSize.height = h;
    CGPoint winPos;
    AXError err;
    winPos.x = x;
    winPos.y = y;
    CFTypeRef size = (CFTypeRef)(AXValueCreate(kAXValueCGSizeType, (const void *)&winSize));
    CFTypeRef pos = (CFTypeRef)(AXValueCreate(kAXValueCGPointType, (const void *)&winPos));
    err = AXUIElementSetAttributeValue(winRef, (__bridge CFStringRef)NSAccessibilitySizeAttribute, (CFTypeRef *)size);
    if (err != 0) {
        NSLog(@"SetWindowRectForTitle set size error: %s %d\n", title, err);
    }
    err = AXUIElementSetAttributeValue(winRef, (__bridge CFStringRef)NSAccessibilityPositionAttribute, (CFTypeRef *)pos);
    if (err != 0) {
        NSLog(@"SetWindowRectForTitle set pos error: %s %d\n", title, err);
    }
    // NSLog(@"PlaceWindowForTitle %s %ld\n", title, pid);
    CFRelease(winRef);
    CFRelease(size);

    return (err == 0) ? 1 : 0;
}

void ConvertARGBToRGBAOpaque(int w, int h, int stride, unsigned char *img) {
	for (int iy = 0; iy < h; iy++) {
        unsigned char *p = &img[iy*stride];
		for (int ix = 0; ix < w; ix++) {
			// ARGB => RGBA, and set A to 255
            p[0] = p[1];
            p[1] = p[2];
            p[2] = p[3];
            p[3] = 255; 
            p += 4;
		}
	}
}

CGImageRef GetWindowImage(long winID) {
    // https://stackoverflow.com/questions/48030214/capture-screenshot-of-macos-window
     CGImageRef image = CGWindowListCreateImage(CGRectNull, 
                            kCGWindowListOptionIncludingWindow,
                            (CGWindowID)winID, 
                            kCGWindowImageBoundsIgnoreFraming|kCGWindowImageNominalResolution|kCGWindowImageShouldBeOpaque);
    return image;
}
