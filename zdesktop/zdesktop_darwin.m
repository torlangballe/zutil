
#import <Foundation/Foundation.h>
#import <CoreFoundation/CoreFoundation.h>
#import <CoreVideo/CoreVideo.h>
#import <AppKit/AppKit.h>

#include <string.h>

// interesting about using gpu:
// https://adrianhesketh.com/2022/03/31/use-m1-gpu-with-go/

BOOL forceScreenRecording = true;

struct WinInfo {
    long wid;
    long pid;
    NSString *title;
    CGRect rect;
    int scale;
};

void removeNonASCIIAndTruncate(NSString **str) {
    NSRange range = [*str rangeOfString:@" - "];
    NSString *snew;
    snew = *str;
    if (range.length != 0) {
        range.length = range.location;
        range.location = 0;
        snew = [*str substringWithRange: range];
    }
    NSData *data = [snew dataUsingEncoding:NSASCIIStringEncoding allowLossyConversion:YES];
    CFRelease(*str);
    *str = [NSString stringWithUTF8String:[data bytes]];
    [data release];
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

int GetWindowCountForPID(long pid) {
    AXUIElementRef appElementRef = AXUIElementCreateApplication(pid);
    //  NSLog(@"getWindowCountForPID: %ld %p\n", pid, appElementRef);
    CFArrayRef windowArray = nil;
    AXUIElementCopyAttributeValue(appElementRef, kAXWindowsAttribute, (CFTypeRef*)&windowArray);
    CFRelease(appElementRef);
    if (windowArray == nil) {
        return -1;
    }
    CFIndex count = CFArrayGetCount(windowArray);
    CFRelease(windowArray);
    return (int)count;
}

// also: https://stackoverflow.com/questions/56597221/detecting-screen-recording-settings-on-macos-catalina/58991936#58991936
int CanRecordScreen() {
    // return CGPreflightScreenCaptureAccess() ? 1 : 0;
    return CGRequestScreenCaptureAccess() ? 1 : 0;
    // if (@available(macOS 10.15, *)) {
    //     CGDisplayStreamRef stream = CGDisplayStreamCreate(CGMainDisplayID(), 1, 1, kCVPixelFormatType_32BGRA, nil, ^(CGDisplayStreamFrameStatus status, uint64_t displayTime, IOSurfaceRef frameSurface, CGDisplayStreamUpdateRef updateRef) {
    //     });
    //     int can = 0;
    //     if (stream != NULL) {
    //         can = 1;
    //     }
    //     // NSLog(@"NSCanRecord: %d", can);
    //     if (stream) {
    //         CFRelease(stream);
    //     }
    //     return can;
    // }
    // return 1;
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

void CloseWindowForWindowRef(AXUIElementRef winRef) {
    AXUIElementRef buttonRef = nil;
    AXError err = AXUIElementCopyAttributeValue(winRef, kAXCloseButtonAttribute, (CFTypeRef*)&buttonRef);
    if (buttonRef == nil) { // it might be sheet in chrome window
        return;
    }
    AXError err2 = AXUIElementPerformAction(buttonRef, kAXPressAction);
    if (buttonRef != nil) {
        CFRelease(buttonRef);
    }
}

long getWindowIDForPIDAndInRect(long findPID, int x, int y, int w, int h) {
    long wid = 0;
    CFArrayRef windowList = CGWindowListCopyWindowInfo(kCGWindowListOptionOnScreenOnly|kCGWindowListExcludeDesktopElements, kCGNullWindowID);
    for (NSMutableDictionary* entry in (__bridge NSArray*)windowList)
    {
        int layer = [[entry objectForKey:(id)kCGWindowLayer] integerValue];
        if (layer == 0) {
            continue;
        }
        long pid = (long)[[entry objectForKey:(id)kCGWindowOwnerPID] integerValue];
        if (pid != findPID) {
            continue;
        }
        NSDictionary *dict = [entry objectForKey:(id)kCGWindowBounds];
        int dx = (int)[(NSNumber *)dict[@"X"] floatValue];
        int dy = (int)[(NSNumber *)dict[@"Y"] floatValue];
        int dw = (int)[(NSNumber *)dict[@"Width"] floatValue];
        int dh = (int)[(NSNumber *)dict[@"Height"] floatValue];
        if (dx < x) {
            // NSLog(@"X\n");
            continue;
        }
        if (dy < y) {
            // NSLog(@"Y\n");
            continue;
        }
        if (dx + dw > x + w) {
            // NSLog(@"W %d %d %d %d\n", w, dw, x, dx);
            continue;
        }
        if (dy + dh > y + h) {
            // NSLog(@"H\n");
            continue;
        }
        wid = (long)[[entry objectForKey:(id)kCGWindowNumber] integerValue];
        NSLog(@"Win: %ld wid:%ld %d\n", pid, wid, dx);
        break;
    }
    CFRelease(windowList);
    return wid;
}

CFArrayRef getWindowsForPID(long pid) {
    CFArrayRef windowArray = nil;
    AXUIElementRef appElementRef = AXUIElementCreateApplication(pid);
    AXError err = AXUIElementCopyAttributeValue(appElementRef, kAXWindowsAttribute, (CFTypeRef*)&windowArray);
    CFRelease(appElementRef);
    if (err != 0 && windowArray != 0) {
        NSLog(@"getWindowsForPID Err: %d %ld\n", err, (long)CFArrayGetCount(windowArray));
    }
    return windowArray;
}

NSMutableDictionary *openWinRefsForRectsDict = NULL;

NSString *CloseOldWindowWithSamePIDAndRectReturnKey(long pid, int x, int y, int w, int h) {
    NSString *key = [NSString stringWithFormat:@"%ld %d %d %d %d", pid, x, y, w, h];
    if (openWinRefsForRectsDict == NULL) {
        openWinRefsForRectsDict = [[NSMutableDictionary alloc] init];
    } else if (w == 0 && h == 0) {
        [ openWinRefsForRectsDict removeAllObjects ];
        return key;
    }
    NSValue *nsval = (NSValue*)[ openWinRefsForRectsDict objectForKey: key ];
    if (nsval != nil) {
        AXUIElementRef wref = (AXUIElementRef)[nsval pointerValue];
        CloseWindowForWindowRef(wref);
        CFRelease(wref);
    }
    return key;
}

void CloseOldWindowWithSamePIDAndRect(long pid, int x, int y, int w, int h) {
    CloseOldWindowWithSamePIDAndRectReturnKey(pid, x, y, w, h);
}

int CloseOldWindowWithSamePIDAndRectOnceNew(long pid, int x, int y, int w, int h) {
    CFArrayRef windowArray = getWindowsForPID(pid);
    NSLog(@"closeOldWindowWithIDInRectOnceNew1 %ld\n", pid);
    if (windowArray == nil) {
        // NSLog(@"getAXElementOfWindowForTitle is nil: %s pid=%ld err=%d\n", title, pid, err);
        return 0;
    }
    CFIndex nItems = CFArrayGetCount(windowArray);
    NSLog(@"closeWindowWithID: %ld %ld\n", pid, (long)nItems);
    for (int i = 0; i < nItems; i++) {
        CFTypeRef posValue, sizeValue;
        CGPoint point;
        CGSize size;
        NSString *title = nil;
        AXUIElementRef winRef = (AXUIElementRef) CFArrayGetValueAtIndex(windowArray, i);
        NSLog(@"closeWindowWithID loop: %d\n", i);
        AXUIElementCopyAttributeValue(winRef, kAXTitleAttribute, (CFTypeRef *)&title);
        AXUIElementCopyAttributeValue(winRef, kAXPositionAttribute, (CFTypeRef*)&posValue);
        AXValueGetValue(posValue, kAXValueCGPointType, &point);
        int ax = (int)point.x;
        int ay = (int)point.y;
        if (ax < x || ay < y) {
            CFRelease(windowArray);
            return 0;
        }
        AXUIElementCopyAttributeValue(winRef, kAXSizeAttribute, (CFTypeRef*)&sizeValue);
        AXValueGetValue(sizeValue, kAXValueCGSizeType, &size);
        int aw = (int)size.width;
        int ah = (int)size.height;
        if (ax + aw > x + w || ay + ah > y + h) {
            CFRelease(windowArray);
            return 0;
        }
        NSString *key = CloseOldWindowWithSamePIDAndRectReturnKey(pid, x, y, w, h);
        [ openWinRefsForRectsDict setValue: [NSValue valueWithPointer:winRef] forKey: key ];
        // NSLog(@"Close!!: %@\n", title);
        CFRelease(windowArray);
        return 1;
    }
    CFRelease(windowArray);
    return 0;
}

const char *getWindowIDs(struct WinInfo *find, BOOL debug, BOOL(*gotWin)(struct WinInfo *find, struct WinInfo w)) {
    if (forceScreenRecording) {
        if (!CanRecordScreen()) {
            NSLog(@"Can't record screen");
            return "can't record screen";
        }
        forceScreenRecording = false;
    }
    CFArrayRef windowList = CGWindowListCopyWindowInfo(kCGWindowListOptionOnScreenOnly, kCGNullWindowID);
    if (windowList == nil) {
        NSLog(@"getWindowIDs no windows!\n");
        return "";
    }
    if (CFArrayGetCount(windowList) == 0) {
        CFRelease(windowList);
        NSLog(@"[2] getWindowIDs no windows!\n");
        return "";
    }

    long winCount = (long)CFArrayGetCount(windowList);
    if (winCount > 20) {
        NSLog(@"getWindowIDs: %ld\n", winCount);
    }
    if (gotWin != NULL || debug)  {
        for (NSDictionary* entry in (__bridge NSArray*)windowList) {
            struct WinInfo w;
            w.title = [entry objectForKey:(id)kCGWindowName];
            if (w.title == nil) {
                NSLog(@"getWindowID loop: title is nil. This can happen if you exposé the windows away.\n");
                continue;
            }
            CFRetain(w.title);
            removeNonASCIIAndTruncate(&w.title);
            w.pid = (long)[[entry objectForKey:(id)kCGWindowOwnerPID] integerValue];
            w.wid = (long)[[entry objectForKey:(id)kCGWindowNumber] integerValue];
            if (debug) {
                NSLog(@"Win: %@ pid:%ld wid:%ld\n", w.title, w.pid, w.wid);
            }
            NSDictionary *dict = [entry objectForKey:(id)kCGWindowBounds];
            w.rect.origin.x = (CGFloat)[(NSNumber *)dict[@"X"] floatValue];
            w.rect.origin.y = (CGFloat)[(NSNumber *)dict[@"Y"] floatValue];
            w.rect.size.width = (CGFloat)[(NSNumber *)dict[@"Width"] floatValue];
            w.rect.size.height = (CGFloat)[(NSNumber *)dict[@"Height"] floatValue];
            // NSLog(@"Size: %@ %g %g %g %g", w.title, (float)w.rect.origin.x, (float)w.rect.origin.y, (float)w.rect.size.width, (float)w.rect.size.height);
            if (gotWin != NULL && gotWin(find, w)) {
                CFRelease(w.title);
                CFRelease(windowList);
                return "";
            }
            CFRelease(w.title);
        }
    }
    CFRelease(windowList);
    return "window not found for getall";
}

void PrintWindowTitles() {
    getWindowIDs(NULL, YES, NULL);
}

const char *ns2chars(NSString *s) {
    int max = [s length] * 2;
    char *chars = (char*)malloc(max);
    if ([s getCString:chars maxLength:max encoding:NSUTF8StringEncoding]) {
        return (const char *)chars;
    }
    return NULL;
}

const char *GetAllWindowTitlesTabSeparated(long pid) {
    NSMutableString *str = [NSMutableString stringWithCapacity: 5000];
    CFArrayRef windowList = CGWindowListCopyWindowInfo(kCGWindowListOptionOnScreenOnly, kCGNullWindowID);
    for (NSMutableDictionary* entry in (__bridge NSArray*)windowList)
    {
        long wpid = (long)[[entry objectForKey:(id)kCGWindowOwnerPID] integerValue];
        if (wpid != pid) {
            continue;
        }
        NSString *title = [entry objectForKey:(id)kCGWindowName];
        if (title == NULL) {
            continue;
        }
        if(str.length != 0) {
            [str appendString: @"\t"];
        }
        [str appendString: title];
    }

    const char *cstr = ns2chars(str);
    [str release];
    CFRelease(windowList);
    return cstr;
}

BOOL findTitle(struct WinInfo *find, struct WinInfo w) {
    // NSLog(@"findTitle:%@ == %@\n", w.title, find->title);
    if ([w.title compare: find->title] == NSOrderedSame) {
        find->wid = w.wid;
        find->pid = w.pid;
        find->rect = w.rect;
        find->scale = w.scale;
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

WinIDInfo WindowGetIDScaleAndRectForTitle(const char *title) {
    struct WinIDInfo got;
    struct WinInfo find;
    find.wid = 0;
    find.scale = 0;
    find.title = [NSString stringWithUTF8String: title];
    got.err = getWindowIDs(&find, NO, findTitle);
    if (got.err != nil && strlen(got.err) > 0) {
        NSLog(@"getwin err: %s\n", got.err);
        return got;
    }
    // NSLog(@"got win: %@ %g %g %g %g\n", find.title, (float)find.rect.origin.x, (float)find.rect.origin.y, (float)find.rect.size.width, (float)find.rect.size.height);
    NSScreen *screen = getBestScreenForBounds(find.rect);
    got.scale = screen.backingScaleFactor;
    CFRelease(screen);

    got.winID = find.wid;
    got.x = find.rect.origin.x;
    got.y = find.rect.origin.y;
    got.w = find.rect.size.width;
    got.h = find.rect.size.height;
    CFRelease(find.title);

    return got;
}

AXUIElementRef getAXElementOfWindowForTitle(const char *title, long pid, BOOL debug) {
    CFArrayRef windowArray = getWindowsForPID(pid);
    if (windowArray == nil) {
        // NSLog(@"getAXElementOfWindowForTitle is nil: %s pid=%ld err=%d\n", title, pid, err);
        return nil;
    }
    NSString *nsTitle = [NSString stringWithUTF8String: title];
    AXUIElementRef matchingWinRef = nil;
    CFIndex nItems = CFArrayGetCount(windowArray);
    removeNonASCIIAndTruncate(&nsTitle);
    for (int i = 0; i < nItems; i++) {
        AXUIElementRef winRef = (AXUIElementRef) CFArrayGetValueAtIndex(windowArray, i);
        NSString *winTitle = nil;
        AXUIElementCopyAttributeValue(winRef, kAXTitleAttribute, (CFTypeRef *)&winTitle);
        if (winTitle == nil) {
            //!!! NSLog(@"Win: <nil-title> # %@\n", nsTitle);
            continue;
        }
        removeNonASCIIAndTruncate(&winTitle);
        if (debug) {
            NSLog(@"Win1: '%@' %d\n", winTitle, (int)[winTitle length]);
            // NSLog(@"Win2: '%@' %d %lu\n", nsTitle, (int)[nsTitle length], strlen(title));
        }
        // NSLog(@"Win1: '%@' '%@' %d\n", winTitle, nsTitle, [winTitle compare:nsTitle] == NSOrderedSame);
        if ([winTitle compare:nsTitle] == NSOrderedSame) {
        //    NSLog(@"Win1: '%@' '%@' %d\n", winTitle, nsTitle, [winTitle compare:nsTitle] == NSOrderedSame);
            matchingWinRef = winRef;
            CFRetain(matchingWinRef);
            CFRelease(winTitle);
            break;
        }
        CFRelease(winTitle);
    }
    CFRelease(windowArray);
    CFRelease(nsTitle);
    return matchingWinRef;
}

int CloseWindowForTitle(const char *title, long pid) {
    AXUIElementRef winRef = getAXElementOfWindowForTitle(title, pid, false);
    // if (winRef == 0) {
    //     getAXElementOfWindowForTitle(title, pid, true);
    // }
    // NSLog(@"CloseWindowForTitle1 %ld %s %p\n", pid, title, winRef);
    if (winRef == nil) {
        return 0;
    }
    // NSLog(@"CloseWindowForTitle2\n");
    CloseWindowForWindowRef(winRef);
    CFRelease(winRef);
    // NSLog(@"CloseWindowForTitle3\n");
    return 1;
}

int ActivateWindowForTitle(const char *title, long pid) {
    NSRunningApplication* app = [NSRunningApplication runningApplicationWithProcessIdentifier: pid];
    [app activateWithOptions: NSApplicationActivateAllWindows];
    AXUIElementRef winRef = getAXElementOfWindowForTitle(title, pid, false);
    if (winRef == nil) {
        // NSLog(@"ActivateWindowForTitle: no window for %s %ld\n", title, pid);
        return 0;
    }
    AXError err = AXUIElementPerformAction(winRef, kAXRaiseAction);
    if (err != 0) {
        // NSLog(@"ActivateWindowForTitle error: %s %d\n", title, err);
        return 0;
    }
    CFRelease(winRef);
    return 1;
}

int SetWindowRectForTitle(const char *title, long pid, int x, int y, int w, int h) {
    // NSLog(@"*******PlaceWindowForTitle %s %ld\n", title, pid);
    AXUIElementRef winRef = getAXElementOfWindowForTitle(title, pid, NO);
    if (winRef == nil) {
        NSLog(@"SetWindowRectForTitle no window for %s %ld\n", title, pid);
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
    err = AXUIElementSetAttributeValue(winRef, (__bridge CFStringRef)NSAccessibilityPositionAttribute, (CFTypeRef *)pos);
    if (err != 0) {
        NSLog(@"SetWindowRectForTitle set pos error: %s %d\n", title, err);
    }
    err = AXUIElementSetAttributeValue(winRef, (__bridge CFStringRef)NSAccessibilitySizeAttribute, (CFTypeRef *)size);
    if (err != 0) {
        NSLog(@"SetWindowRectForTitle set size error: %s %d\n", title, err);
    }
    // NSLog(@"PlaceWindowForTitle %s %ld\n", title, pid);
    CFRelease(winRef);
    CFRelease(size);
    CFRelease(pos);

    return (err == 0) ? 1 : 0;
}

void ShowAlert(char *str) {
    CFOptionFlags cfRes;
    CFStringRef cfstr = CFStringCreateWithCString(NULL, str, kCFStringEncodingUTF8);

    CFUserNotificationDisplayAlert(5, kCFUserNotificationNoteAlertLevel,
        NULL, NULL, NULL,
        cfstr,
        NULL,
        (CFStringRef)@"OK",
        NULL, NULL,
        &cfRes);
    CFRelease(cfstr);
}
