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
#import "zdesktop_windowstream_darwin.h"

// Initializes minimal Cocoa/graphics state needed by ScreenCaptureKit in non-app processes.
void EnsureCocoaGraphicsInitialized() {
    static dispatch_once_t onceToken;
    dispatch_once(&onceToken, ^{
        // In CLI/server processes there is no app run loop driving the main queue,
        // so dispatch_sync(dispatch_get_main_queue(), ...) can deadlock.
        // [NSApplication sharedApplication];
        // [NSScreen screens];
        CGMainDisplayID();
    });
}

@interface WindowCaptureStream : NSObject <SCStreamOutput>
@property (nonatomic, retain) SCStream *stream;
@property (nonatomic, retain) dispatch_queue_t outputQueue;
@property (nonatomic, retain) dispatch_queue_t audioOutputQueue;
@property (nonatomic, retain) NSLock *lock;
@property void *latestH264Bytes;
@property int latestH264Length;
@property void *latestAudioPCM16Bytes;
@property int latestAudioPCM16Length;
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

// Supplies one PCM16 input buffer to the AudioConverter when encoding Opus.
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

// Converts AVCC-length-prefixed H264 NAL units into Annex-B start-code format.
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

// Appends H264 SPS/PPS parameter sets (if available) in Annex-B format.
static void appendParameterSets(NSMutableData *dst, CMFormatDescriptionRef formatDesc) {
    static const uint8_t startCode[] = {0x00, 0x00, 0x00, 0x01};
    for (int i = 0; i < 2; i++) {
        const uint8_t *ps = NULL;
        size_t psSize = 0;
        size_t psCount = 0;
        if (CMVideoFormatDescriptionGetH264ParameterSetAtIndex(formatDesc, i, &ps, &psSize, &psCount, NULL) == noErr && psSize > 0) {
            [dst appendBytes:startCode length:4];
            [dst appendBytes:ps length:psSize];
        }
    }
}

// VideoToolbox callback that receives encoded H264 samples and stores the latest frame.
static void onEncodedFrame(void *outputCallbackRefCon, void *sourceFrameRefCon, OSStatus status, VTEncodeInfoFlags infoFlags, CMSampleBufferRef sampleBuffer);

@implementation WindowCaptureStream

// Creates a capture stream object and initializes runtime state.
- (instancetype)init {
    self = [super init];
    if (self) {
        _outputQueue = dispatch_queue_create("zdesktop.window.stream.queue", DISPATCH_QUEUE_SERIAL);
        _audioOutputQueue = dispatch_queue_create("zdesktop.window.audio.stream.queue", DISPATCH_QUEUE_SERIAL);
        _lock = [[NSLock alloc] init];
        _latestH264Bytes = NULL;
        _latestH264Length = 0;
        _latestAudioPCM16Bytes = NULL;
        _latestAudioPCM16Length = 0;
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

// Ensures capture and encoder resources are released on object teardown.
- (void)dealloc {
    [self stop];
    [super dealloc];
}

// Creates or recreates the H264 encoder for the provided frame dimensions.
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
    OSStatus err = VTCompressionSessionCreate(kCFAllocatorDefault, width, height, kCMVideoCodecType_H264, NULL, NULL, NULL, onEncodedFrame, self, &_vtSession);
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
    int32_t keyIntervalFrames = fps * 2;
    CFNumberRef keyFramesNum = CFNumberCreate(kCFAllocatorDefault, kCFNumberSInt32Type, &keyIntervalFrames);
    VTSessionSetProperty(_vtSession, kVTCompressionPropertyKey_MaxKeyFrameInterval, keyFramesNum);
    CFRelease(keyFramesNum);
    int32_t keyIntervalSeconds = 2;
    CFNumberRef keySecondsNum = CFNumberCreate(kCFAllocatorDefault, kCFNumberSInt32Type, &keyIntervalSeconds);
    VTSessionSetProperty(_vtSession, kVTCompressionPropertyKey_MaxKeyFrameIntervalDuration, keySecondsNum);
    CFRelease(keySecondsNum);
    CFRelease(fpsNum);
    VTCompressionSessionPrepareToEncodeFrames(_vtSession);
    return YES;
}

// Resolves target window/display into an SCContentFilter used for capture.
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

// startWithWindowTitle starts a ScreenCaptureKit stream for the requested target and optional audio.
- (BOOL)startWithWindowTitle:(NSString *)winTitle
                  appBundleID:(NSString *)appBundleID
                 captureAudio:(int)captureAudio
                                     srcOffsetX:(int)srcOffsetX
                                     srcOffsetY:(int)srcOffsetY
                                         srcWidth:(int)srcWidth
                                        srcHeight:(int)srcHeight
                                        destWidth:(int)destWidth
                                     destHeight:(int)destHeight
                        error:(NSString **)errorOut {
    if (_stream != nil || _vtSession != nil || _opusConverter != NULL) {
        [self stop];
    }
    dispatch_semaphore_t started = dispatch_semaphore_create(0);
    __block BOOL ok = NO;
    __block NSString *startErr = nil;
    _captureAudio = captureAudio;
    [SCShareableContent getShareableContentExcludingDesktopWindows:true
                                               onScreenWindowsOnly:true
                                                 completionHandler:^(SCShareableContent * _Nullable shareableContent, NSError * _Nullable error) {
        if (error != nil) {
            startErr = [error localizedDescription];
            dispatch_semaphore_signal(started);
            return;
        }
        NSString *ferr = nil;
        SCContentFilter *filter = [self filterFromShareableContent:shareableContent
                                                          winTitle:winTitle
                                                        appBundleID:appBundleID
                                                              error:&ferr];
        if (filter == nil) {
            startErr = ferr;
            dispatch_semaphore_signal(started);
            return;
        }
        SCStreamConfiguration *config = [[[SCStreamConfiguration alloc] init] autorelease];
        config.capturesAudio = (captureAudio != 0);
        config.excludesCurrentProcessAudio = YES;
        config.showsCursor = NO;
        config.captureResolution = SCCaptureResolutionBest;
        CGRect captureRectPts = filter.contentRect;
        if (srcWidth > 0 && srcHeight > 0) {
            // Match screenshot behavior: inset crop rect directly in content coordinate space.
            CGRect contentRectPts = filter.contentRect;
            CGRect requested = CGRectMake((CGFloat)srcOffsetX,
                                          (CGFloat)srcOffsetY,
                                          (CGFloat)srcWidth,
                                          (CGFloat)srcHeight);
            captureRectPts = requested;
            config.sourceRect = requested;
        }
        config.width = destWidth;
        config.height = destHeight;
        config.pixelFormat = kCVPixelFormatType_32BGRA;
        config.minimumFrameInterval = CMTimeMake(1, 30);
        config.sampleRate = 48000;
        config.channelCount = 2;
        NSLog(@"Stream config: captureRectPts x: %f y: %f w: %f h: %f, dest: %dx%d, pixelFormat: %u",
              captureRectPts.origin.x, captureRectPts.origin.y, captureRectPts.size.width, captureRectPts.size.height,
              (int)destWidth, (int)destHeight, (unsigned int)config.pixelFormat);
        self.stream = [[[SCStream alloc] initWithFilter:filter configuration:config delegate:nil] autorelease];
        NSError *addOutErr = nil;
        if (![self.stream addStreamOutput:self type:SCStreamOutputTypeScreen sampleHandlerQueue:self.outputQueue error:&addOutErr]) {
            startErr = [addOutErr localizedDescription];
            dispatch_semaphore_signal(started);
            return;
        }
        if (captureAudio != 0) {
            NSError *addAudioErr = nil;
            if (![self.stream addStreamOutput:self type:SCStreamOutputTypeAudio sampleHandlerQueue:self.audioOutputQueue error:&addAudioErr]) {
                startErr = [addAudioErr localizedDescription];
                dispatch_semaphore_signal(started);
                return;
            }
        }

        [self.stream startCaptureWithCompletionHandler:^(NSError * _Nullable serr) {
            if (serr != nil) {
                startErr = [serr localizedDescription];
                dispatch_semaphore_signal(started);
                return;
            }
            self.running = YES;
            ok = YES;
            dispatch_semaphore_signal(started);
        }];
    }];

    long waitResult = dispatch_semaphore_wait(started, dispatch_time(DISPATCH_TIME_NOW, (int64_t)(10 * NSEC_PER_SEC)));
    if (waitResult != 0) {
        *errorOut = @"timeout waiting for stream start";
        return NO;
    }
    if (!ok) {
        [startErr retain];
        *errorOut = startErr;
        NSLog(@"getShareableContent6 error: %@", *errorOut);
        return NO;
    }
    return YES;
}

// Stops capture and frees all active buffers and encoder resources.
- (void)stop {
    if (!_running && _stream == nil && _vtSession == nil && _opusConverter == NULL) {
        return;
    }
    _running = NO;
    _captureAudio = 0;
    if (_stream != nil) {
        NSError *removeErr = nil;
        [_stream removeStreamOutput:self type:SCStreamOutputTypeScreen error:&removeErr];
        if (removeErr != nil) {
            NSLog(@"removeStreamOutput(screen) error: %@", removeErr.localizedDescription);
        }
        removeErr = nil;
        [_stream removeStreamOutput:self type:SCStreamOutputTypeAudio error:&removeErr];
        if (removeErr != nil) {
            NSLog(@"removeStreamOutput(audio) error: %@", removeErr.localizedDescription);
        }
        dispatch_semaphore_t stopped = dispatch_semaphore_create(0);
        [_stream stopCaptureWithCompletionHandler:^(NSError * _Nullable error) {
            if (error != nil) {
                NSLog(@"stopCapture error: %@", error.localizedDescription);
            }
            dispatch_semaphore_signal(stopped);
        }];
        long stopWait = dispatch_semaphore_wait(stopped, dispatch_time(DISPATCH_TIME_NOW, (int64_t)(2 * NSEC_PER_SEC)));
        if (stopWait != 0) {
            NSLog(@"stopCapture timed out waiting for completion");
        }
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

// Replaces the latest encoded H264 frame with the newly produced one.
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

// Replaces the latest PCM16 audio buffer and metadata.
- (void)storePCM16Audio:(NSData *)data sampleRate:(int)sampleRate channels:(int)channels pts:(int64_t)ptsNS {
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
    void *old = _latestAudioPCM16Bytes;
    _latestAudioPCM16Bytes = newBuf;
    _latestAudioPCM16Length = len;
    _latestAudioSampleRate = sampleRate;
    _latestAudioChannels = channels;
    _latestAudioPTSNS = ptsNS;
    [_lock unlock];
    if (old != NULL) {
        free(old);
    }
}

// Replaces the latest Opus packet and associated timing metadata.
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

// Creates or reuses an AudioConverter configured for PCM16-to-Opus encoding.
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
                        break;
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

// Encodes one PCM16 audio buffer into Opus and stores the resulting packet.
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

// Submits one captured video sample to VideoToolbox for H264 encoding.
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

// Converts captured audio samples to PCM16, stores them, and also encodes Opus.
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

// SCStreamOutput callback that dispatches screen and audio sample processing.
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

// SCStream delegate callback that marks stream state when capture stops unexpectedly.
- (void)stream:(SCStream *)stream didStopWithError:(NSError *)error {
    if (error != nil) {
        NSLog(@"WindowCaptureStream didStopWithError: %@", error.localizedDescription);
    }
    _running = NO;
}

@end

static void onEncodedFrame(void *outputCallbackRefCon, void *sourceFrameRefCon, OSStatus status, VTEncodeInfoFlags infoFlags, CMSampleBufferRef sampleBuffer) {
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

// Returns whether the native stream object is currently marked as running.
int isWindowCaptureStreamRunning(void *stream) {
    WindowCaptureStream *wcs = (WindowCaptureStream *)stream;
    if (wcs == nil) {
        return 0;
    }
    return wcs.running ? 1 : 0;
}

// C entry point that creates and starts a window capture stream instance with optional crop+scale.
void *startWindowCaptureStreamWithRegion(const char *winTitle, const char *appBundleID, int captureAudio, int srcOffsetX, int srcOffsetY, int srcWidth, int srcHeight, int destWidth, int destHeight, char *err, int errLen) {
    NSString *sTitle = nil;
    NSString *sAppID = nil;
    if (winTitle != NULL && strlen(winTitle) > 0) {
        sTitle = [NSString stringWithUTF8String:winTitle];
    }
    if (appBundleID != NULL && strlen(appBundleID) > 0) {
        sAppID = [NSString stringWithUTF8String:appBundleID];
    }
    WindowCaptureStream *wcs = [[WindowCaptureStream alloc] init];
            NSLog(@"startWindowCaptureStream9\n");
    NSString *startErr = nil;
    if (![wcs startWithWindowTitle:sTitle
                        appBundleID:sAppID
                       captureAudio:captureAudio
                         srcOffsetX:srcOffsetX
                         srcOffsetY:srcOffsetY
                           srcWidth:srcWidth
                          srcHeight:srcHeight
                          destWidth:destWidth
                         destHeight:destHeight
                              error:&startErr]) {
        if (startErr != nil && err != NULL && errLen > 0) {
            [startErr getCString:err maxLength:errLen encoding:NSUTF8StringEncoding];
            NSLog(@"startWindowCaptureStream error: %@\n", startErr);
        }
        [wcs release];
        return nil;
    }
    return (void *)wcs;
}

// C entry point that stops and releases a previously created capture stream.
void stopWindowCaptureStream(void *stream) {
    WindowCaptureStream *wcs = (WindowCaptureStream *)stream;
    if (wcs == nil) {
        return;
    }
    [wcs stop];
    [wcs release];
}

// Copies and clears the latest encoded H264 frame from the stream state.
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

// Copies and clears the latest PCM16 audio buffer from the stream state.
int copyLatestWindowStreamAudioPCM16(void *stream, short **data, int *sampleCount, int *sampleRate, int *channels, int64_t *ptsNS) {
    WindowCaptureStream *wcs = (WindowCaptureStream *)stream;
    if (wcs == nil || data == NULL || sampleCount == NULL || sampleRate == NULL || channels == NULL || ptsNS == NULL) {
        return 0;
    }
    if (wcs.lock == nil) {
        return 0;
    }
    [wcs.lock lock];
    void *pcm = wcs.latestAudioPCM16Bytes;
    int len = wcs.latestAudioPCM16Length;
    int sr = wcs.latestAudioSampleRate;
    int ch = wcs.latestAudioChannels;
    int64_t pts = wcs.latestAudioPTSNS;
    wcs.latestAudioPCM16Bytes = NULL;
    wcs.latestAudioPCM16Length = 0;
    wcs.latestAudioSampleRate = 0;
    wcs.latestAudioChannels = 0;
    wcs.latestAudioPTSNS = 0;
    [wcs.lock unlock];
    if (pcm == NULL || len <= 0 || sr == 0 || ch == 0) {
        return 0;
    }
    *data = (short *)pcm;
    *sampleCount = len / (int)sizeof(short);
    *sampleRate = sr;
    *channels = ch;
    *ptsNS = pts;
    return 1;
}

// Copies and clears the latest Opus packet and timing metadata from the stream state.
int copyLatestWindowStreamAudioOpus(void *stream, unsigned char **data, int *length, int *sampleRate, int *channels, int64_t *durationNS, int64_t *ptsNS) {
    WindowCaptureStream *wcs = (WindowCaptureStream *)stream;
    if (wcs == nil || data == NULL || length == NULL || sampleRate == NULL || channels == NULL || durationNS == NULL || ptsNS == NULL) {
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
    int64_t duration = wcs.latestAudioDurationNS;
    int64_t pts = wcs.latestAudioPTSNS;
    wcs.latestAudioOpusBytes = NULL;
    wcs.latestAudioOpusLength = 0;
    wcs.latestAudioDurationNS = 0;
    wcs.latestAudioPTSNS = 0;
    [wcs.lock unlock];
    if (opus == NULL || len <= 0 || sr == 0 || ch == 0) {
        return 0;
    }
    *data = (unsigned char *)opus;
    *length = len;
    *sampleRate = sr;
    *channels = ch;
    *durationNS = duration;
    *ptsNS = pts;
    return 1;
}
