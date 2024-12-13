#import <Foundation/Foundation.h>
#import <CoreServices/CoreServices.h>
#import <ScreenCaptureKit/ScreenCaptureKit.h>

CGRect croppedRect(CGRect rect, CGRect with)  {
    CGRect out;
    out.origin.x = rect.origin.x + with.origin.x;
    out.origin.y = rect.origin.y + with.origin.y;
    out.size.width = rect.size.width + with.size.width;
    out.size.height = rect.size.height + with.size.height;
    return out;
}

void imageOfWindow(NSString *winTitle, NSString *appBundleID, CGRect insetRect, void(^got)(CGImageRef image, NSString *err)) {
    [SCShareableContent getShareableContentExcludingDesktopWindows: true
                                               onScreenWindowsOnly: true
                                                 completionHandler: ^(SCShareableContent * _Nullable shareableContent, NSError * _Nullable error) {
        SCRunningApplication *foundApp = nil;
        SCWindow *foundWin = nil;
        if (error) {
            got(nil, error.localizedDescription);
            return;
        }
        for (SCRunningApplication *app in shareableContent.applications) {
            if ([app.bundleIdentifier isEqualToString: appBundleID]) {
                foundApp = app;
            }
        }
        if (appBundleID != nil && foundApp == nil) {
            got(nil, @"app not found");
            return;
        }
        for (SCWindow *win in shareableContent.windows) {
            if ([ win.title isEqualToString: winTitle]) {
                foundWin = win;
                break;
            }
        }
        if (foundWin == nil) {
            got(nil, @"window not found for capture");
            return;
        }
        SCContentFilter *filter = [[SCContentFilter alloc] initWithDesktopIndependentWindow:foundWin];
        SCStreamConfiguration *configuration = [[SCStreamConfiguration alloc] init];
        configuration.capturesAudio = NO;
        configuration.excludesCurrentProcessAudio = YES;
        configuration.preservesAspectRatio = YES;
        configuration.showsCursor = NO;
        configuration.captureResolution = SCCaptureResolutionBest;
        if (insetRect.size.width != 0) {
            CGRect frame = foundWin.frame;
            frame.origin = CGPointMake(0, 0);
            configuration.sourceRect = insetRect;
            configuration.width = NSWidth(insetRect) * filter.pointPixelScale;
            configuration.height = NSHeight(insetRect) * filter.pointPixelScale;
        } else {
            configuration.width = NSWidth(filter.contentRect) * filter.pointPixelScale;
            configuration.height = NSHeight(filter.contentRect) * filter.pointPixelScale;
        }
        [SCScreenshotManager captureImageWithFilter:filter configuration:configuration completionHandler:^(CGImageRef  _Nullable cgImage, NSError * _Nullable error) {
            if (error) {
                got(nil, [error localizedDescription]);
                return;
            }
            got(cgImage, nil);
        }];
    }];
}


char cerr[1024];
const char *ImageOfWindow(char *winTitle, char *appBundleID, CGRect insetRect, CGImageRef *cgImagePtr) {
    const int timeoutSecs = 2; // it should only take 10ms ish for smaller sizes, so lets keep it small to avoid congestion
    NSString *nstitle = [NSString stringWithUTF8String:winTitle];
    NSString *nsapp = [NSString stringWithUTF8String:appBundleID];
    __block NSString *snapErr = nil;
    __block dispatch_semaphore_t sem = dispatch_semaphore_create(0);
    __block bool timedOut = false;
    imageOfWindow(nstitle, nsapp, insetRect, ^(CGImageRef image, NSString *err) {
        if (timedOut) {
            NSLog(@"ImageOfWindow That Timed out finally finished: %s\n", winTitle);
        }
        [nstitle release];
        [nsapp release];
        if (err != nil) {
            snapErr = err;
        } else {
            CFRetain(image);
        }
        *cgImagePtr = image;
        dispatch_semaphore_signal(sem);
    });
   int64_t timeoutAt = dispatch_time(DISPATCH_TIME_NOW, (int64_t)timeoutSecs * NSEC_PER_SEC);
    if (dispatch_semaphore_wait(sem, timeoutAt)) {
    // if (dispatch_semaphore_wait(sem, DISPATCH_TIME_FOREVER)) {
        timedOut = true;
        return "timed out";
    } else if (snapErr != nil) {
            NSLog(@"ImageOfWindow Error: %s %@\n", winTitle, snapErr);
            [snapErr getCString:cerr maxLength:1024 encoding:NSUTF8StringEncoding];
            return cerr;
    }
    return "";
}
