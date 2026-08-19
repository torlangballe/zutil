#import <Foundation/Foundation.h>
#import <CoreServices/CoreServices.h>
#import <ScreenCaptureKit/ScreenCaptureKit.h>
#import <Cocoa/Cocoa.h>
#import <VideoToolbox/VideoToolbox.h>
#import <CoreMedia/CoreMedia.h>
#import <AudioToolbox/AudioToolbox.h>
#import <unistd.h>
#import <string.h>
#import <stdlib.h>
#import <math.h>
#import <sys/types.h>
#import <sys/sysctl.h>
#import <unistd.h>
#import "zdesktop_windowstream_darwin.h"

void EnsureCocoaGraphicsInitialized() {
    static dispatch_once_t onceToken;
    dispatch_once(&onceToken, ^{
        // In CLI/server processes there is no app run loop driving the main queue,
        // so dispatch_sync(dispatch_get_main_queue(), ...) can deadlock.
        [NSApplication sharedApplication];
        [NSScreen screens];
        CGMainDisplayID();
    });
}

@interface WindowCaptureStream : NSObject <SCStreamOutput>
@property (nonatomic, retain) SCStream *stream;
@property (nonatomic, retain) dispatch_queue_t outputQueue;
@property (nonatomic, retain) NSLock *lock;
@property void *latestH264Bytes;
@property int latestH264Length;
@property void *latestAudioPCM16Bytes;
@property int latestAudioPCM16Length;
@property int latestAudioPCM16SampleRate;
@property int latestAudioPCM16Channels;
@property int64_t latestAudioPCM16PTSNS;
@property void *latestAudioOpusBytes;
@property int latestAudioOpusLength;
@property int latestAudioSampleRate;
@property int latestAudioChannels;
@property int64_t latestVideoPTSNS;
@property int64_t latestAudioPTSNS;
@property int64_t latestAudioDurationNS;
@property BOOL running;
@property VTCompressionSessionRef vtSession;
@property AudioConverterRef opusConverter;
@property int opusInputSampleRate;
@property int opusInputChannels;
@property int videoWidth;
@property int videoHeight;
@property int captureAudio;
@end

typedef struct {
    const uint8_t *data;
    UInt32 dataByteSize;
    UInt32 channels;
    BOOL consumed;
} OpusConverterInputState;

static OSStatus opusConverterInputDataProc(AudioConverterRef inAudioConverter,
                                           UInt32 *ioNumberDataPackets,
                                           AudioBufferList *ioData,
                                           AudioStreamPacketDescription **outDataPacketDescription,
                                           void *inUserData) {
    OpusConverterInputState *state = (OpusConverterInputState *)inUserData;
    if (state == NULL || state->consumed || state->data == NULL || state->dataByteSize == 0 || ioNumberDataPackets == NULL || ioData == NULL) {
        if (ioNumberDataPackets != NULL) {
            *ioNumberDataPackets = 0;
        }
        return noErr;
    }
    UInt32 bytesPerFrame = state->channels * (UInt32)sizeof(int16_t);
    if (bytesPerFrame == 0) {
        *ioNumberDataPackets = 0;
        return noErr;
    }
    ioData->mNumberBuffers = 1;
    ioData->mBuffers[0].mData = (void *)state->data;
    ioData->mBuffers[0].mDataByteSize = state->dataByteSize;
    ioData->mBuffers[0].mNumberChannels = state->channels;
    *ioNumberDataPackets = state->dataByteSize / bytesPerFrame;
    if (outDataPacketDescription != NULL) {
        *outDataPacketDescription = NULL;
    }
    state->consumed = YES;
    return noErr;
}

static void onEncodedFrame(void *outputCallbackRefCon,
                           void *sourceFrameRefCon,
                           OSStatus status,
                           VTEncodeInfoFlags infoFlags,
                           CMSampleBufferRef sampleBuffer);

static double rmsPCM16ForData(NSData *data) {
    if (data == nil) {
        return 0.0;
    }
    NSUInteger byteLen = [data length];
    NSUInteger sampleCount = byteLen / sizeof(int16_t);
    if (sampleCount == 0) {
        return 0.0;
    }
    const int16_t *samples = (const int16_t *)[data bytes];
    double sumSquares = 0.0;
    for (NSUInteger i = 0; i < sampleCount; i++) {
        double v = (double)samples[i];
        sumSquares += v * v;
    }
    return sqrt(sumSquares / (double)sampleCount);
}

@implementation WindowCaptureStream

- (instancetype)init {
    self = [super init];
    if (self) {
        _outputQueue = dispatch_queue_create("zdesktop.window.stream.queue", DISPATCH_QUEUE_SERIAL);
        _lock = [[NSLock alloc] init];
        _latestH264Bytes = NULL;
        _latestH264Length = 0;
        _latestAudioPCM16Bytes = NULL;
        _latestAudioPCM16Length = 0;
        _latestAudioPCM16SampleRate = 0;
        _latestAudioPCM16Channels = 0;
        _latestAudioPCM16PTSNS = 0;
        _latestAudioOpusBytes = NULL;
        _latestAudioOpusLength = 0;
        _latestAudioSampleRate = 0;
        _latestAudioChannels = 0;
        _latestVideoPTSNS = 0;
        _latestAudioPTSNS = 0;
        _latestAudioDurationNS = 0;
        _running = NO;
        _vtSession = nil;
        _opusConverter = NULL;
        _opusInputSampleRate = 0;
        _opusInputChannels = 0;
        _videoWidth = 0;
        _videoHeight = 0;
        _captureAudio = 0;
    }
    return self;
}

- (void)dealloc {
    [self stop];
    [super dealloc];
}

- (BOOL)setupEncoderForWidth:(int)width height:(int)height {
    if (_vtSession != nil && _videoWidth == width && _videoHeight == height) {
        return YES;
    }
    if (_vtSession != nil) {
        VTCompressionSessionCompleteFrames(_vtSession, kCMTimeInvalid);
        VTCompressionSessionInvalidate(_vtSession);
        CFRelease(_vtSession);
        _vtSession = nil;
    }
    _videoWidth = width;
    _videoHeight = height;
    OSStatus err = VTCompressionSessionCreate(kCFAllocatorDefault,
                                              width,
                                              height,
                                              kCMVideoCodecType_H264,
                                              NULL,
                                              NULL,
                                              NULL,
                                              onEncodedFrame,
                                              self,
                                              &_vtSession);
    if (err != noErr || _vtSession == nil) {
        NSLog(@"VTCompressionSessionCreate failed: %d", (int)err);
        return NO;
    }
    VTSessionSetProperty(_vtSession, kVTCompressionPropertyKey_RealTime, kCFBooleanTrue);
    VTSessionSetProperty(_vtSession, kVTCompressionPropertyKey_AllowFrameReordering, kCFBooleanFalse);
    VTSessionSetProperty(_vtSession, kVTCompressionPropertyKey_ProfileLevel, kVTProfileLevel_H264_Baseline_AutoLevel);
    int32_t fps = 30;
    CFNumberRef fpsNum = CFNumberCreate(kCFAllocatorDefault, kCFNumberSInt32Type, &fps);
    VTSessionSetProperty(_vtSession, kVTCompressionPropertyKey_ExpectedFrameRate, fpsNum);
    CFRelease(fpsNum);
    VTCompressionSessionPrepareToEncodeFrames(_vtSession);
    return YES;
}

- (SCContentFilter *)filterFromShareableContent:(SCShareableContent *)shareableContent
                                       winTitle:(NSString *)winTitle
                                     appBundleID:(NSString *)appBundleID
                                           error:(NSString **)errorOut {
    SCRunningApplication *foundApp = nil;
    SCWindow *foundWin = nil;
    for (SCRunningApplication *app in shareableContent.applications) {
        if (appBundleID != nil && [app.bundleIdentifier isEqualToString:appBundleID]) {
            foundApp = app;
            break;
        }
    }
    if (appBundleID != nil && [appBundleID length] != 0 && foundApp == nil) {
        *errorOut = @"App not Found";
        return nil;
    }
    if (winTitle != nil && [winTitle length] != 0) {
        for (SCWindow *win in shareableContent.windows) {
            if ([win.title isEqualToString:winTitle]) {
                foundWin = win;
                break;
            }
        }
        if (foundWin == nil) {
            *errorOut = @"Window not Found for Capture";
            return nil;
        }
        return [[[SCContentFilter alloc] initWithDesktopIndependentWindow:foundWin] autorelease];
    }
    SCDisplay *display = nil;
    for (SCDisplay *d in shareableContent.displays) {
        display = d;
    }
    if (display == nil) {
        *errorOut = @"Display not Found for Capture";
        return nil;
    }
    return [[[SCContentFilter alloc] initWithDisplay:display excludingWindows:@[]] autorelease];
}

BOOL isChildProcessOfPID(pid_t childPID, pid_t parentPID) {
    struct kinfo_proc info;
    size_t length = sizeof(info);
    int mib[4] = { CTL_KERN, KERN_PROC, KERN_PROC_PID, childPID };
    
    if (sysctl(mib, 4, &info, &length, NULL, 0) == 0 && length > 0) {
        return info.kp_eproc.e_ppid == parentPID;
    }
    return NO;
}

- (BOOL)startWithWindowTitle:(NSString *)winTitle
                  appBundleID:(NSString *)appBundleID
                 captureAudio:(int)captureAudio
            cropRect:(CGRect)cropRect
            destSize:(CGSize)destSize
                        error:(NSString **)errorOut {
    __block BOOL done = NO;
    __block BOOL ok = NO;
    __block NSString *startErr = nil;
    _captureAudio = captureAudio;
    [SCShareableContent getShareableContentExcludingDesktopWindows:true
                                               onScreenWindowsOnly:true
                                                 completionHandler:^(SCShareableContent * _Nullable shareableContent, NSError * _Nullable error) {
        if (error != nil) {
            startErr = [error localizedDescription];
            done = YES;
            return;
        }
        NSString *ferr = nil;
        __block SCWindow *targetWindow = nil;
        // 2. Locate the specific window by its title string
        for (SCWindow *window in shareableContent.windows) {
            if ([window.title containsString:@"Your Shaka Player Title"]) {
                targetWindow = window;
                break;
            }
        }
        if (!targetWindow) {
            startErr = [NSString stringWithFormat:@"Target Chrome window not found."];
            done = YES;
            return;
        }
        // 3. Identify the parent PID of the specific target window
        pid_t targetPID = targetWindow.owningApplication.processID;
        NSMutableArray<SCRunningApplication *> *isolatedApps = [NSMutableArray array];

        // 4. Collect only Chrome processes that belong to this browser instance tree
        for (SCRunningApplication *app in shareableContent.applications) {
            BOOL isMainTargetBrowser = (app.processID == targetPID);
            BOOL isAssociatedHelper = [app.applicationName containsString:@"Google Chrome Helper"] && isChildProcessOfPID(app.processID, targetPID);
            if (isMainTargetBrowser || isAssociatedHelper) {
                [isolatedApps addObject:app];
            }
        }
        // 5. Build the ScreenCaptureKit content filter
        SCDisplay *activeDisplay = shareableContent.displays.firstObject; // Default fallback
        CGRect windowFrame = targetWindow.frame;
        CGPoint windowCenter = CGPointMake(CGRectGetMidX(windowFrame), CGRectGetMidY(windowFrame));

        for (SCDisplay *display in shareableContent.displays) {
            CGRect displayFrame = display.frame;
            if (CGRectContainsPoint(displayFrame, windowCenter)) {
                activeDisplay = display;
                break;
            }
        }
        SCContentFilter *filter = [[SCContentFilter alloc] initWithDisplay:activeDisplay
                                                 includingApplications:isolatedApps
                                                      exceptingWindows:@[]];

        if (filter == nil) {
            startErr = ferr;
            done = YES;
            return;
        }
        SCStreamConfiguration *config = [[[SCStreamConfiguration alloc] init] autorelease];
        config.capturesAudio = true; //(captureAudio != 0);
        // config.excludesCurrentProcessAudio = YES;
        config.showsCursor = NO;
        config.preservesAspectRatio = YES;
        config.captureResolution = SCCaptureResolutionBest;
        BOOL hasCrop = cropRect.size.width > 0 && cropRect.size.height > 0;
        BOOL hasDestSize = destSize.width > 0 && destSize.height > 0;
        if (hasCrop) {
            config.sourceRect = cropRect;
        }
        if (hasDestSize) {
            config.width = (size_t)destSize.width;
            config.height = (size_t)destSize.height;
        } else if (hasCrop) {
            config.width = NSWidth(cropRect) * filter.pointPixelScale;
            config.height = NSHeight(cropRect) * filter.pointPixelScale;
        } else {
            config.width = NSWidth(filter.contentRect) * filter.pointPixelScale;
            config.height = NSHeight(filter.contentRect) * filter.pointPixelScale;
        }
        config.pixelFormat = kCVPixelFormatType_32BGRA;
        config.minimumFrameInterval = CMTimeMake(1, 30);
        config.sampleRate = 48000;
        config.channelCount = 2;

        NSError *addOutErr = nil;
        if (![self.stream addStreamOutput:self type:SCStreamOutputTypeScreen sampleHandlerQueue:self.outputQueue error:&addOutErr]) {
            startErr = [addOutErr localizedDescription];
            done = YES;
            return;
        }
        if (captureAudio != 0) {
            NSError *addAudioErr = nil;
            if (![self.stream addStreamOutput:self type:SCStreamOutputTypeAudio sampleHandlerQueue:self.outputQueue error:&addAudioErr]) {
                startErr = [addAudioErr localizedDescription];
                done = YES;
                return;
            }
        }

        [self.stream startCaptureWithCompletionHandler:^(NSError * _Nullable serr) {
            if (serr != nil) {
                startErr = [serr localizedDescription];
                done = YES;
                return;
            }
            self.running = YES;
            ok = YES;
            done = YES;
        }];
    }];

    int waited = 0;
    while (!done && waited < 2500) {
        usleep(1000);
        waited++;
    }
    if (!done) {
        *errorOut = @"timeout waiting for stream start";
        return NO;
    }
    if (!ok) {
        *errorOut = startErr;
        return NO;
    }
    return YES;
}
/*
- (BOOL)startWithWindowTitle:(NSString *)winTitle
                  appBundleID:(NSString *)appBundleID
                 captureAudio:(int)captureAudio
            cropRect:(CGRect)cropRect
            destSize:(CGSize)destSize
                        error:(NSString **)errorOut {
    __block BOOL done = NO;
    __block BOOL ok = NO;
    __block NSString *startErr = nil;
    _captureAudio = captureAudio;
    [SCShareableContent getShareableContentExcludingDesktopWindows:true
                                               onScreenWindowsOnly:true
                                                 completionHandler:^(SCShareableContent * _Nullable shareableContent, NSError * _Nullable error) {
        if (error != nil) {
            startErr = [error localizedDescription];
            done = YES;
            return;
        }
        NSString *ferr = nil;
        SCContentFilter *filter = [self filterFromShareableContent:shareableContent
                                                          winTitle:winTitle
                                                        appBundleID:appBundleID
                                                              error:&ferr];
        if (filter == nil) {
            startErr = ferr;
            done = YES;
            return;
        }
        SCStreamConfiguration *config = [[[SCStreamConfiguration alloc] init] autorelease];
        config.capturesAudio = true; //(captureAudio != 0);
        // config.excludesCurrentProcessAudio = YES;
        config.showsCursor = NO;
        config.preservesAspectRatio = YES;
        config.captureResolution = SCCaptureResolutionBest;
        BOOL hasCrop = cropRect.size.width > 0 && cropRect.size.height > 0;
        BOOL hasDestSize = destSize.width > 0 && destSize.height > 0;
        if (hasCrop) {
            config.sourceRect = cropRect;
        }
        if (hasDestSize) {
            config.width = (size_t)destSize.width;
            config.height = (size_t)destSize.height;
        } else if (hasCrop) {
            config.width = NSWidth(cropRect) * filter.pointPixelScale;
            config.height = NSHeight(cropRect) * filter.pointPixelScale;
        } else {
            config.width = NSWidth(filter.contentRect) * filter.pointPixelScale;
            config.height = NSHeight(filter.contentRect) * filter.pointPixelScale;
        }
        config.pixelFormat = kCVPixelFormatType_32BGRA;
        config.minimumFrameInterval = CMTimeMake(1, 30);
        config.sampleRate = 48000;
        config.channelCount = 2;

        self.stream = [[[SCStream alloc] initWithFilter:filter configuration:config delegate:nil] autorelease];
        NSError *addOutErr = nil;
        if (![self.stream addStreamOutput:self type:SCStreamOutputTypeScreen sampleHandlerQueue:self.outputQueue error:&addOutErr]) {
            startErr = [addOutErr localizedDescription];
            done = YES;
            return;
        }
        if (captureAudio != 0) {
            NSError *addAudioErr = nil;
            if (![self.stream addStreamOutput:self type:SCStreamOutputTypeAudio sampleHandlerQueue:self.outputQueue error:&addAudioErr]) {
                startErr = [addAudioErr localizedDescription];
                done = YES;
                return;
            }
        }

        [self.stream startCaptureWithCompletionHandler:^(NSError * _Nullable serr) {
            if (serr != nil) {
                startErr = [serr localizedDescription];
                done = YES;
                return;
            }
            self.running = YES;
            ok = YES;
            done = YES;
        }];
    }];

    int waited = 0;
    while (!done && waited < 2500) {
        usleep(1000);
        waited++;
    }
    if (!done) {
        *errorOut = @"timeout waiting for stream start";
        return NO;
    }
    if (!ok) {
        *errorOut = startErr;
        return NO;
    }
    return YES;
}
*/
- (void)stop {
    if (!_running && _stream == nil && _vtSession == nil && _opusConverter == NULL) {
        return;
    }
    _running = NO;
    if (_stream != nil) {
        [ _stream stopCaptureWithCompletionHandler:^(NSError * _Nullable error) {
            if (error != nil) {
                NSLog(@"stopCapture error: %@", error.localizedDescription);
            }
        }];
        _stream = nil;
    }
    if (_vtSession != nil) {
        VTCompressionSessionCompleteFrames(_vtSession, kCMTimeInvalid);
        VTCompressionSessionInvalidate(_vtSession);
        CFRelease(_vtSession);
        _vtSession = nil;
    }
    if (_opusConverter != NULL) {
        AudioConverterDispose(_opusConverter);
        _opusConverter = NULL;
        _opusInputSampleRate = 0;
        _opusInputChannels = 0;
    }
    [_lock lock];
    void *oldH264 = _latestH264Bytes;
    void *oldPCM = _latestAudioPCM16Bytes;
    void *oldOpus = _latestAudioOpusBytes;
    _latestH264Bytes = NULL;
    _latestH264Length = 0;
    _latestAudioPCM16Bytes = NULL;
    _latestAudioPCM16Length = 0;
    _latestAudioPCM16SampleRate = 0;
    _latestAudioPCM16Channels = 0;
    _latestAudioPCM16PTSNS = 0;
    _latestAudioOpusBytes = NULL;
    _latestAudioOpusLength = 0;
    _latestAudioSampleRate = 0;
    _latestAudioChannels = 0;
    _latestVideoPTSNS = 0;
    _latestAudioPTSNS = 0;
    _latestAudioDurationNS = 0;
    [_lock unlock];
    if (oldH264 != NULL) {
        free(oldH264);
    }
    if (oldPCM != NULL) {
        free(oldPCM);
    }
    if (oldOpus != NULL) {
        free(oldOpus);
    }
}

- (void)storeEncodedVideo:(NSData *)data pts:(int64_t)ptsNS {
    if (data == nil || [data length] == 0) {
        return;
    }
    int len = (int)[data length];
    void *newBuf = malloc(len);
    if (newBuf == NULL) {
        return;
    }
    memcpy(newBuf, [data bytes], len);
    [_lock lock];
    void *old = _latestH264Bytes;
    _latestH264Bytes = newBuf;
    _latestH264Length = len;
    _latestVideoPTSNS = ptsNS;
    [_lock unlock];
    if (old != NULL) {
        free(old);
    }
}

- (void)storePCM16Audio:(NSData *)data sampleRate:(int)sampleRate channels:(int)channels pts:(int64_t)ptsNS {
    if (data == nil || [data length] == 0) {
        return;
    }
    double rmsPCM16 = rmsPCM16ForData(data);
    NSLog(@"storePCM16Audio rms=%f bytes=%lu rate=%d channels=%d ptsNS=%lld", rmsPCM16, (unsigned long)[data length], sampleRate, channels, ptsNS);
    int len = (int)[data length];
    void *newBuf = malloc(len);
    if (newBuf == NULL) {
        return;
    }
    memcpy(newBuf, [data bytes], len);
    [_lock lock];
    void *old = _latestAudioPCM16Bytes;
    _latestAudioPCM16Bytes = newBuf;
    _latestAudioPCM16Length = len;
    _latestAudioPCM16SampleRate = sampleRate;
    _latestAudioPCM16Channels = channels;
    _latestAudioPCM16PTSNS = ptsNS;
    _latestAudioSampleRate = sampleRate;
    _latestAudioChannels = channels;
    _latestAudioPTSNS = ptsNS;
    [_lock unlock];
    if (old != NULL) {
        free(old);
    }
}

- (void)storeOpusAudio:(NSData *)data sampleRate:(int)sampleRate channels:(int)channels durationNS:(int64_t)durationNS pts:(int64_t)ptsNS {
    if (data == nil || [data length] == 0) {
        return;
    }
    int len = (int)[data length];
    void *newBuf = malloc(len);
    if (newBuf == NULL) {
        return;
    }
    memcpy(newBuf, [data bytes], len);
    [_lock lock];
    void *old = _latestAudioOpusBytes;
    _latestAudioOpusBytes = newBuf;
    _latestAudioOpusLength = len;
    _latestAudioSampleRate = sampleRate;
    _latestAudioChannels = channels;
    _latestAudioDurationNS = durationNS;
    _latestAudioPTSNS = ptsNS;
    [_lock unlock];
    if (old != NULL) {
        free(old);
    }
}

- (BOOL)setupOpusEncoderForSampleRate:(int)sampleRate channels:(int)channels {
    if (sampleRate <= 0 || channels <= 0) {
        return NO;
    }
    if (_opusConverter != NULL && _opusInputSampleRate == sampleRate && _opusInputChannels == channels) {
        return YES;
    }
    if (_opusConverter != NULL) {
        AudioConverterDispose(_opusConverter);
        _opusConverter = NULL;
    }

    AudioStreamBasicDescription inASBD;
    memset(&inASBD, 0, sizeof(inASBD));
    inASBD.mSampleRate = sampleRate;
    inASBD.mFormatID = kAudioFormatLinearPCM;
    inASBD.mFormatFlags = kAudioFormatFlagIsSignedInteger | kAudioFormatFlagIsPacked;
    inASBD.mBytesPerPacket = channels * (UInt32)sizeof(int16_t);
    inASBD.mFramesPerPacket = 1;
    inASBD.mBytesPerFrame = channels * (UInt32)sizeof(int16_t);
    inASBD.mChannelsPerFrame = channels;
    inASBD.mBitsPerChannel = 16;

    AudioStreamBasicDescription outASBD;
    memset(&outASBD, 0, sizeof(outASBD));
    outASBD.mSampleRate = 48000;
    outASBD.mFormatID = kAudioFormatOpus;
    outASBD.mChannelsPerFrame = channels;

    OSStatus err = noErr;
    UInt32 subtype = kAudioFormatOpus;
    UInt32 size = 0;
    AudioClassDescription chosen;
    BOOL haveChosen = NO;
    if (AudioFormatGetPropertyInfo(kAudioFormatProperty_Encoders, sizeof(subtype), &subtype, &size) == noErr && size >= sizeof(AudioClassDescription)) {
        UInt32 count = size / (UInt32)sizeof(AudioClassDescription);
        AudioClassDescription *descs = (AudioClassDescription *)malloc(size);
        if (descs != NULL) {
            if (AudioFormatGetProperty(kAudioFormatProperty_Encoders, sizeof(subtype), &subtype, &size, descs) == noErr) {
                for (UInt32 i = 0; i < count; i++) {
                    if (descs[i].mSubType == kAudioFormatOpus) {
                        chosen = descs[i];
                        haveChosen = YES;
                        // if (descs[i].mManufacturer == kAppleSoftwareAudioCodecManufacturer) { // only on iPhone...
                        if (descs[i].mManufacturer == (UInt32)'appl') {
                            break;
                        }
                    }
                }
            }
            free(descs);
        }
    }
    if (haveChosen) {
        err = AudioConverterNewSpecific(&inASBD, &outASBD, 1, &chosen, &_opusConverter);
    } else {
        err = AudioConverterNew(&inASBD, &outASBD, &_opusConverter);
    }
    if (err != noErr || _opusConverter == NULL) {
        NSLog(@"AudioConverterNew(Opus) failed: %d", (int)err);
        _opusConverter = NULL;
        return NO;
    }

    UInt32 bitRate = channels >= 2 ? 96000 : 64000;
    AudioConverterSetProperty(_opusConverter, kAudioConverterEncodeBitRate, sizeof(bitRate), &bitRate);
    _opusInputSampleRate = sampleRate;
    _opusInputChannels = channels;
    return YES;
}

- (void)encodePCM16ToOpus:(NSData *)pcmData sampleRate:(int)sampleRate channels:(int)channels pts:(int64_t)ptsNS {
    if (pcmData == nil || [pcmData length] == 0 || sampleRate <= 0 || channels <= 0) {
        return;
    }
    if (![self setupOpusEncoderForSampleRate:sampleRate channels:channels]) {
        return;
    }
    UInt32 maxPacketSize = 1500;
    UInt32 maxPacketSizeLen = sizeof(maxPacketSize);
    if (AudioConverterGetProperty(_opusConverter, kAudioConverterPropertyMaximumOutputPacketSize, &maxPacketSizeLen, &maxPacketSize) != noErr || maxPacketSize == 0) {
        maxPacketSize = 1500;
    }
    NSMutableData *outData = [NSMutableData dataWithLength:maxPacketSize];
    AudioBufferList outABL;
    memset(&outABL, 0, sizeof(outABL));
    outABL.mNumberBuffers = 1;
    outABL.mBuffers[0].mData = [outData mutableBytes];
    outABL.mBuffers[0].mDataByteSize = maxPacketSize;
    outABL.mBuffers[0].mNumberChannels = channels;

    OpusConverterInputState state;
    state.data = (const uint8_t *)[pcmData bytes];
    state.dataByteSize = (UInt32)[pcmData length];
    state.channels = channels;
    state.consumed = NO;

    UInt32 ioOutputPackets = 1;
    OSStatus err = AudioConverterFillComplexBuffer(_opusConverter,
                                                   opusConverterInputDataProc,
                                                   &state,
                                                   &ioOutputPackets,
                                                   &outABL,
                                                   NULL);
    if (err != noErr || ioOutputPackets == 0 || outABL.mBuffers[0].mDataByteSize == 0) {
        return;
    }
    NSData *packet = [NSData dataWithBytes:outABL.mBuffers[0].mData length:outABL.mBuffers[0].mDataByteSize];
    int frames = (int)([pcmData length] / ((size_t)channels * sizeof(int16_t)));
    int64_t durationNS = (int64_t)((double)frames / (double)sampleRate * 1000000000.0);
    [self storeOpusAudio:packet sampleRate:48000 channels:channels durationNS:durationNS pts:ptsNS];
}

- (void)encodeVideoSampleBuffer:(CMSampleBufferRef)sampleBuffer {
    CVImageBufferRef imageBuffer = CMSampleBufferGetImageBuffer(sampleBuffer);
    if (imageBuffer == nil) {
        return;
    }
    int width = (int)CVPixelBufferGetWidth(imageBuffer);
    int height = (int)CVPixelBufferGetHeight(imageBuffer);
    if (![self setupEncoderForWidth:width height:height]) {
        return;
    }
    CMTime pts = CMSampleBufferGetPresentationTimeStamp(sampleBuffer);
    VTEncodeInfoFlags flags = 0;
    OSStatus err = VTCompressionSessionEncodeFrame(_vtSession,
                                                   imageBuffer,
                                                   pts,
                                                   kCMTimeInvalid,
                                                   NULL,
                                                   NULL,
                                                   &flags);
    if (err != noErr) {
        NSLog(@"VTCompressionSessionEncodeFrame failed: %d", (int)err);
    }
}

- (void)consumeAudioSampleBuffer:(CMSampleBufferRef)sampleBuffer {
    if (_captureAudio == 0) {
        return;
    }
    CMFormatDescriptionRef fmt = CMSampleBufferGetFormatDescription(sampleBuffer);
    if (fmt == nil) {
        return;
    }
    const AudioStreamBasicDescription *asbd = CMAudioFormatDescriptionGetStreamBasicDescription(fmt);
    if (asbd == nil) {
        return;
    }

    size_t ablSize = 0;
    CMBlockBufferRef blockBuffer = nil;
    OSStatus status = CMSampleBufferGetAudioBufferListWithRetainedBlockBuffer(sampleBuffer,
                                                                               &ablSize,
                                                                               NULL,
                                                                               0,
                                                                               NULL,
                                                                               NULL,
                                                                               kCMSampleBufferFlag_AudioBufferList_Assure16ByteAlignment,
                                                                               &blockBuffer);
    if (status != noErr && status != kCMSampleBufferError_ArrayTooSmall) {
        if (blockBuffer != nil) {
            CFRelease(blockBuffer);
        }
        return;
    }
    if (ablSize == 0) {
        if (blockBuffer != nil) {
            CFRelease(blockBuffer);
        }
        return;
    }
    AudioBufferList *audioBufferList = (AudioBufferList *)malloc(ablSize);
    if (audioBufferList == NULL) {
        if (blockBuffer != nil) {
            CFRelease(blockBuffer);
        }
        return;
    }
    status = CMSampleBufferGetAudioBufferListWithRetainedBlockBuffer(sampleBuffer,
                                                                      &ablSize,
                                                                      audioBufferList,
                                                                      ablSize,
                                                                      NULL,
                                                                      NULL,
                                                                      kCMSampleBufferFlag_AudioBufferList_Assure16ByteAlignment,
                                                                      &blockBuffer);
    if (status != noErr) {
        free(audioBufferList);
        if (blockBuffer != nil) {
            CFRelease(blockBuffer);
        }
        return;
    }

    int channels = (int)MAX(1, asbd->mChannelsPerFrame);
    int targetChannels = channels >= 2 ? 2 : 1;
    int numFrames = (int)CMSampleBufferGetNumSamples(sampleBuffer);
    if (numFrames <= 0) {
        free(audioBufferList);
        if (blockBuffer != nil) {
            CFRelease(blockBuffer);
        }
        return;
    }
    NSMutableData *pcmData = [NSMutableData dataWithLength:(size_t)numFrames * (size_t)targetChannels * sizeof(int16_t)];
    int16_t *dst = (int16_t *)[pcmData mutableBytes];

    BOOL converted = NO;
    if (asbd->mFormatID == kAudioFormatLinearPCM && (asbd->mFormatFlags & kAudioFormatFlagIsFloat) != 0) {
        if (audioBufferList->mNumberBuffers == 1) {
            AudioBuffer b = audioBufferList->mBuffers[0];
            float *src = (float *)b.mData;
            size_t needBytes = (size_t)numFrames * (size_t)channels * sizeof(float);
            if (src != nil && b.mDataByteSize >= needBytes) {
                for (int i = 0; i < numFrames; i++) {
                    for (int c = 0; c < targetChannels; c++) {
                        int srcChan = c < channels ? c : 0;
                        float v = src[i * channels + srcChan];
                        if (v > 1.0f) v = 1.0f;
                        if (v < -1.0f) v = -1.0f;
                        dst[i * targetChannels + c] = (int16_t)(v * 32767.0f);
                    }
                }
                converted = YES;
            }
        } else if ((int)audioBufferList->mNumberBuffers >= channels) {
            for (int i = 0; i < numFrames; i++) {
                BOOL okFrame = YES;
                for (int c = 0; c < targetChannels; c++) {
                    int srcChan = c < channels ? c : 0;
                    AudioBuffer b = audioBufferList->mBuffers[srcChan];
                    float *src = (float *)b.mData;
                    size_t needBytes = (size_t)(i + 1) * sizeof(float);
                    if (src == nil || b.mDataByteSize < needBytes) {
                        okFrame = NO;
                        break;
                    }
                    float v = src[i];
                    if (v > 1.0f) v = 1.0f;
                    if (v < -1.0f) v = -1.0f;
                    dst[i * targetChannels + c] = (int16_t)(v * 32767.0f);
                }
                if (!okFrame) {
                    break;
                }
            }
            converted = YES;
        }
    } else if (asbd->mFormatID == kAudioFormatLinearPCM && (asbd->mFormatFlags & kAudioFormatFlagIsSignedInteger) != 0 && asbd->mBitsPerChannel == 16) {
        if (audioBufferList->mNumberBuffers == 1) {
            AudioBuffer b = audioBufferList->mBuffers[0];
            int16_t *src = (int16_t *)b.mData;
            size_t needBytes = (size_t)numFrames * (size_t)channels * sizeof(int16_t);
            if (src != nil && b.mDataByteSize >= needBytes) {
                for (int i = 0; i < numFrames; i++) {
                    for (int c = 0; c < targetChannels; c++) {
                        int srcChan = c < channels ? c : 0;
                        dst[i * targetChannels + c] = src[i * channels + srcChan];
                    }
                }
                converted = YES;
            }
        } else if ((int)audioBufferList->mNumberBuffers >= channels) {
            for (int i = 0; i < numFrames; i++) {
                BOOL okFrame = YES;
                for (int c = 0; c < targetChannels; c++) {
                    int srcChan = c < channels ? c : 0;
                    AudioBuffer b = audioBufferList->mBuffers[srcChan];
                    int16_t *src = (int16_t *)b.mData;
                    size_t needBytes = (size_t)(i + 1) * sizeof(int16_t);
                    if (src == nil || b.mDataByteSize < needBytes) {
                        okFrame = NO;
                        break;
                    }
                    dst[i * targetChannels + c] = src[i];
                }
                if (!okFrame) {
                    break;
                }
            }
            converted = YES;
        }
    }

    free(audioBufferList);
    if (!converted) {
        if (blockBuffer != nil) {
            CFRelease(blockBuffer);
        }
        return;
    }
    CMTime pts = CMSampleBufferGetPresentationTimeStamp(sampleBuffer);
    int64_t ptsNS = CMTimeGetSeconds(pts) * 1000000000.0;
    [self storePCM16Audio:pcmData sampleRate:(int)asbd->mSampleRate channels:targetChannels pts:ptsNS];
    [self encodePCM16ToOpus:pcmData sampleRate:(int)asbd->mSampleRate channels:targetChannels pts:ptsNS];
    if (blockBuffer != nil) {
        CFRelease(blockBuffer);
    }
}

- (void)stream:(SCStream *)stream didOutputSampleBuffer:(CMSampleBufferRef)sampleBuffer ofType:(SCStreamOutputType)type {
    if (!CMSampleBufferIsValid(sampleBuffer)) {
        return;
    }
    if (type == SCStreamOutputTypeScreen) {
        [self encodeVideoSampleBuffer:sampleBuffer];
    } else if (type == SCStreamOutputTypeAudio) {
        [self consumeAudioSampleBuffer:sampleBuffer];
    }
}

- (void)stream:(SCStream *)stream didStopWithError:(NSError *)error {
    if (error != nil) {
        NSLog(@"WindowCaptureStream didStopWithError: %@", error.localizedDescription);
    }
    _running = NO;
}

@end

static void appendAVCCNALUs(NSMutableData *dst, CMBlockBufferRef blockBuffer) {
    size_t lengthAtOffset = 0;
    size_t totalLength = 0;
    char *dataPointer = NULL;
    OSStatus status = CMBlockBufferGetDataPointer(blockBuffer, 0, &lengthAtOffset, &totalLength, &dataPointer);
    if (status != noErr || dataPointer == NULL) {
        return;
    }
    size_t offset = 0;
    while (offset + 4 <= totalLength) {
        uint32_t naluLen = 0;
        memcpy(&naluLen, dataPointer + offset, 4);
        naluLen = CFSwapInt32BigToHost(naluLen);
        offset += 4;
        if (naluLen == 0 || offset + naluLen > totalLength) {
            break;
        }
        static const uint8_t startCode[] = {0x00, 0x00, 0x00, 0x01};
        [dst appendBytes:startCode length:4];
        [dst appendBytes:(dataPointer + offset) length:naluLen];
        offset += naluLen;
    }
}

static void appendParameterSets(NSMutableData *dst, CMFormatDescriptionRef formatDesc) {
    const uint8_t *sps = NULL;
    size_t spsSize = 0;
    size_t spsCount = 0;
    const uint8_t *pps = NULL;
    size_t ppsSize = 0;
    size_t ppsCount = 0;
    if (CMVideoFormatDescriptionGetH264ParameterSetAtIndex(formatDesc, 0, &sps, &spsSize, &spsCount, NULL) == noErr && spsSize > 0) {
        static const uint8_t startCode[] = {0x00, 0x00, 0x00, 0x01};
        [dst appendBytes:startCode length:4];
        [dst appendBytes:sps length:spsSize];
    }
    if (CMVideoFormatDescriptionGetH264ParameterSetAtIndex(formatDesc, 1, &pps, &ppsSize, &ppsCount, NULL) == noErr && ppsSize > 0) {
        static const uint8_t startCode[] = {0x00, 0x00, 0x00, 0x01};
        [dst appendBytes:startCode length:4];
        [dst appendBytes:pps length:ppsSize];
    }
}

static void onEncodedFrame(void *outputCallbackRefCon,
                           void *sourceFrameRefCon,
                           OSStatus status,
                           VTEncodeInfoFlags infoFlags,
                           CMSampleBufferRef sampleBuffer) {
    if (status != noErr || sampleBuffer == nil || !CMSampleBufferDataIsReady(sampleBuffer)) {
        return;
    }
    WindowCaptureStream *wcs = (WindowCaptureStream *)outputCallbackRefCon;
    CFArrayRef attachments = CMSampleBufferGetSampleAttachmentsArray(sampleBuffer, false);
    BOOL isKeyFrame = NO;
    if (attachments != NULL && CFArrayGetCount(attachments) > 0) {
        CFDictionaryRef attachment = (CFDictionaryRef)CFArrayGetValueAtIndex(attachments, 0);
        BOOL notSync = CFDictionaryContainsKey(attachment, kCMSampleAttachmentKey_NotSync);
        isKeyFrame = !notSync;
    }
    NSMutableData *encoded = [NSMutableData data];
    if (isKeyFrame) {
        CMFormatDescriptionRef formatDesc = CMSampleBufferGetFormatDescription(sampleBuffer);
        appendParameterSets(encoded, formatDesc);
    }
    CMBlockBufferRef blockBuffer = CMSampleBufferGetDataBuffer(sampleBuffer);
    if (blockBuffer == nil) {
        return;
    }
    appendAVCCNALUs(encoded, blockBuffer);
    if ([encoded length] == 0) {
        return;
    }
    CMTime pts = CMSampleBufferGetPresentationTimeStamp(sampleBuffer);
    int64_t ptsNS = CMTimeGetSeconds(pts) * 1000000000.0;
    [wcs storeEncodedVideo:encoded pts:ptsNS];
}

int isWindowCaptureStreamRunning(void *stream) {
    WindowCaptureStream *wcs = (WindowCaptureStream *)stream;
    if (wcs == nil) {
        return 0;
    }
    return wcs.running ? 1 : 0;
}

void *startWindowCaptureStream(const char *winTitle, const char *appBundleID, int captureAudio, char *err, int errLen) {
    return startWindowCaptureStreamWithCropAndDestSize(winTitle,
                                                       appBundleID,
                                                       captureAudio,
                                                       CGRectZero,
                                                       CGSizeZero,
                                                       err,
                                                       errLen);
}

void *startWindowCaptureStreamWithCropAndDestSize(const char *winTitle,
                                                  const char *appBundleID,
                                                  int captureAudio,
                                                  CGRect cropRect,
                                                  CGSize destSize,
                                                  char *err,
                                                  int errLen) {
    NSString *sTitle = nil;
    NSString *sAppID = nil;
    if (winTitle != NULL && strlen(winTitle) > 0) {
        sTitle = [NSString stringWithUTF8String:winTitle];
    }
    if (appBundleID != NULL && strlen(appBundleID) > 0) {
        sAppID = [NSString stringWithUTF8String:appBundleID];
    }
    WindowCaptureStream *wcs = [[WindowCaptureStream alloc] init];
    NSString *startErr = nil;
    if (![wcs startWithWindowTitle:sTitle
                        appBundleID:sAppID
                       captureAudio:captureAudio
                          cropRect:cropRect
                          destSize:destSize
                              error:&startErr]) {
        if (startErr != nil && err != NULL && errLen > 0) {
            [startErr getCString:err maxLength:errLen encoding:NSUTF8StringEncoding];
        }
        [wcs release];
        return nil;
    }
    return (void *)wcs;
}

void stopWindowCaptureStream(void *stream) {
    WindowCaptureStream *wcs = (WindowCaptureStream *)stream;
    if (wcs == nil) {
        return;
    }
    [wcs stop];
    [wcs release];
}

int copyLatestWindowStreamH264(void *stream, unsigned char **data, int *length, int64_t *ptsNS) {
    WindowCaptureStream *wcs = (WindowCaptureStream *)stream;
    if (wcs == nil || data == NULL || length == NULL || ptsNS == NULL) {
        return 0;
    }
    if (wcs.lock == nil) {
        return 0;
    }
    [wcs.lock lock];
    void *h264 = wcs.latestH264Bytes;
    int len = wcs.latestH264Length;
    int64_t pts = wcs.latestVideoPTSNS;
    wcs.latestH264Bytes = NULL;
    wcs.latestH264Length = 0;
    wcs.latestVideoPTSNS = 0;
    [wcs.lock unlock];
    if (h264 == NULL || len <= 0) {
        return 0;
    }
    *data = (unsigned char *)h264;
    *length = len;
    *ptsNS = pts;
    return 1;
}

int copyLatestWindowStreamAudioOpus(void *stream, unsigned char **data, int *byteLength, int *sampleRate, int *channels, int64_t *ptsNS) {
    WindowCaptureStream *wcs = (WindowCaptureStream *)stream;
    if (wcs == nil || data == NULL || byteLength == NULL || sampleRate == NULL || channels == NULL || ptsNS == NULL) {
        return 0;
    }
    if (wcs.lock == nil) {
        return 0;
    }
    [wcs.lock lock];
    void *opus = wcs.latestAudioOpusBytes;
    int len = wcs.latestAudioOpusLength;
    int sr = wcs.latestAudioSampleRate;
    int ch = wcs.latestAudioChannels;
    int64_t pts = wcs.latestAudioPTSNS;
    wcs.latestAudioOpusBytes = NULL;
    wcs.latestAudioOpusLength = 0;
    wcs.latestAudioSampleRate = 0;
    wcs.latestAudioChannels = 0;
    wcs.latestAudioPTSNS = 0;
    [wcs.lock unlock];
    if (opus == NULL || len <= 0 || sr == 0 || ch == 0) {
        return 0;
    }
    *data = (unsigned char *)opus;
    *byteLength = len;
    *sampleRate = sr;
    *channels = ch;
    *ptsNS = pts;
    return 1;
}

