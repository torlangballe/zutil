#import <Foundation/Foundation.h>
#import <CoreVideo/CoreVideo.h>
#import <AppKit/AppKit.h>

BOOL forceScreenRecording = true;

struct WinInfo {
    long wid;
    long pid;
    NSString *title;
    NSDictionary *boundsDict;
    int scale;
};

BOOL canRecordScreen() {
    if (@available(macOS 10.15, *)) {
        CGDisplayStreamRef stream = CGDisplayStreamCreate(CGMainDisplayID(), 1, 1, kCVPixelFormatType_32BGRA, nil, ^(CGDisplayStreamFrameStatus status, uint64_t displayTime, IOSurfaceRef frameSurface, CGDisplayStreamUpdateRef updateRef) {
         ;
    });
    BOOL canRecord = stream != NULL;
    if (stream) {
        CFRelease(stream);
    }
        return canRecord;
    } else {
        return YES;
    }
}


const char *getWindowIDs(void *data, BOOL debug, BOOL(*gotWin)(void *data, struct WinInfo w)) {
    if (forceScreenRecording) {
        if (!canRecordScreen()) {
            return "can't record screen";
        }
        forceScreenRecording = false;
    }
    CFArrayRef windowList = CGWindowListCopyWindowInfo(kCGWindowListOptionOnScreenOnly, kCGNullWindowID);
    for (NSMutableDictionary* entry in (__bridge NSArray*)windowList)
    {
        struct WinInfo w;
        w.title = [entry objectForKey:(id)kCGWindowName];
        if (debug) {
            NSLog(@"Win: %@\n", w.title);
        }
        w.pid = (long)[[entry objectForKey:(id)kCGWindowOwnerPID] integerValue];
        w.wid = (long)[[entry objectForKey:(id)kCGWindowNumber] integerValue];
        w.boundsDict = [entry objectForKey:(id)kCGWindowBounds];
        if (!gotWin(data, w)) {
            return "";
        }
    }
    CFRelease(windowList);
    return "window not found";
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

BOOL findTitle(void *data, struct WinInfo w) {
    struct WinInfo *find = (struct WinInfo *)data;
    CGRect bounds;
    if (w.pid == find->pid && [w.title compare: find->title] == NSOrderedSame) {
        find->wid = w.wid;
        bounds.origin.x = (CGFloat)[(NSNumber *)w.boundsDict[@"X"] floatValue];
        bounds.origin.y = (CGFloat)[(NSNumber *)w.boundsDict[@"Y"] floatValue];
        bounds.size.width = (CGFloat)[(NSNumber *)w.boundsDict[@"Width"] floatValue];
        bounds.size.height = (CGFloat)[(NSNumber *)w.boundsDict[@"Height"] floatValue];
        find->scale = getBestScreenForBounds(bounds).backingScaleFactor;
        // NSLog(@"scale:%d bounds: %f,%f %fx%f\n", find->scale, bounds.origin.x, bounds.origin.y, bounds.size.width, bounds.size.height);
        return NO;
    }
    return YES;
}

typedef struct WinIDInfo {
    long       winID;
    int        scale;
    const char *err;
} WinIDInfo;

WinIDInfo WindowGetIDAndScaleForTitle(const char *title, long pid) {
    // NSLog(@"WindowGetIDAndScaleForTitle1\n");
    struct WinInfo find;
    struct WinIDInfo got;    
    find.wid = 0;
    find.scale = 0;
    find.title = [NSString stringWithUTF8String: title];
    find.pid = pid;
    // NSLog(@"WindowGetIDAndScaleForTitle2\n");
    got.err = getWindowIDs(&find, NO, findTitle);
    // NSLog(@"WindowGetIDAndScaleForTitle3: %p\n", got.err);
    // if (strlen(got.err) != 0) {
    //     getWindowIDs(&find, YES, findTitle);
    // }
    got.winID = find.wid;
    got.scale = find.scale;
    CFRelease(find.title);
    return got;
}

// NSArray *getRunAppsForName(const char *app) {
//     NSArray *apps = [[NSWorkspace sharedWorkspace] runningApplications];
//     NSMutableArray *mine = [NSMutableArray arrayWithCapacity: 1];
//     NSString *nsapp = [NSString stringWithUTF8String: app];
//     for (NSRunningApplication *a in apps) {
//         NSString *loc = [a localizedName];
//         NSLog(@"getRunAppsForName: %@\n", loc);
//         if ([loc compare:nsapp] == NSOrderedSame) {
//             // NSLog(@"App: %@\n", loc);
//             [mine addObject: a];
//             CFRelease(loc);
//             continue;      
//         }
//     }
//     CFRelease(apps);
//     CFRelease(nsapp);
//     return mine;
// }

AXUIElementRef getAXElementOfWindowForTitle(const char *title, long pid, BOOL debug) {
    //  NSLog(@"getAXElementOfWindowForTitle: %s %ld\n", title, pid);
    NSString *nsTitle = [NSString stringWithUTF8String: title];
    AXUIElementRef appElementRef = AXUIElementCreateApplication(pid);
    CFArrayRef windowArray = nil;
    AXUIElementCopyAttributeValue(appElementRef, kAXWindowsAttribute, (CFTypeRef*)&windowArray);
    if (windowArray == nil) {
        CFRelease(appElementRef);
        return nil;
    }
    AXUIElementRef matchinWinRef = nil;
    CFIndex nItems = CFArrayGetCount(windowArray);
    for (int i = 0; i < nItems; i++) {
        AXUIElementRef winRef = (AXUIElementRef) CFArrayGetValueAtIndex(windowArray, i);
        NSString *winTitle = nil;
        AXUIElementCopyAttributeValue(winRef, kAXTitleAttribute, (CFTypeRef *)&winTitle);
        if (winTitle == nil) {
            continue;
        }
        if (debug) {
            NSLog(@"Win: %@ # %@\n", winTitle, nsTitle);
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
    if (winRef == 0) {
        getAXElementOfWindowForTitle(title, pid, true);
    }
    // NSLog(@"CloseWindowForTitle1 %s %p\n", title, winRef);
    if (winRef == nil) {
        return 0;
    }
    AXUIElementRef buttonRef = nil;
    AXUIElementCopyAttributeValue(winRef, kAXCloseButtonAttribute, (CFTypeRef*)&buttonRef);
    AXUIElementPerformAction(buttonRef, kAXPressAction);
    CFRelease(buttonRef);
    return 1;
}

int SetWindowSizeForTitle(const char *title, long pid, int w, int h) {
    // NSLog(@"PlaceWindowForTitle %s %s\n", title, app);
    AXUIElementRef winRef = getAXElementOfWindowForTitle(title, pid, NO);
    if (winRef == nil) {
        return 0;
    }
    NSSize winSize;
    winSize.width = w;
    winSize.height = h;
    CFTypeRef size = (CFTypeRef)(AXValueCreate(kAXValueCGSizeType, (const void *)&winSize));
    AXUIElementSetAttributeValue(winRef, (__bridge CFStringRef)NSAccessibilitySizeAttribute, (CFTypeRef *)size);
    CFRelease(size);

    return 1;
}

