#import <Foundation/Foundation.h>
#import <CoreServices/CoreServices.h>
#import <AVFoundation/AVFoundation.h>
#import <Cocoa/Cocoa.h>

@interface CameraCapture : NSObject <AVCaptureVideoDataOutputSampleBufferDelegate>
- (void)start;
- (void)stop;
@end


@interface CameraCapture ()
@property BOOL wantImage;
@property CGImageRef gotImage;
@property (nonatomic, strong) AVCaptureSession *captureSession;
@property (nonatomic, strong) AVCaptureVideoDataOutput *videoOutput;
@property (nonatomic, strong) dispatch_queue_t sessionQueue;
@end

@implementation CameraCapture
- (instancetype)init {
    self = [super init];
    if (self) {
        _captureSession = [[AVCaptureSession alloc] init];
        _videoOutput = [[AVCaptureVideoDataOutput alloc] init];
        _sessionQueue = dispatch_queue_create("camera.session.queue", DISPATCH_QUEUE_SERIAL);
        [self setupSession];
    }
    return self;
}

- (void)setupSession {
    // AVCaptureDevice *videoDevice = [AVCaptureDevice defaultDeviceWithMediaType:AVMediaTypeVideo];
    AVCaptureDevice *videoDevice = [AVCaptureDevice defaultDeviceWithDeviceType:AVCaptureDeviceTypeExternal mediaType:AVMediaTypeVideo position: AVCaptureDevicePositionUnspecified];
    if (!videoDevice) {
        NSLog(@"No video device found");
        return;
    }
    NSError *error = nil;
    AVCaptureDeviceInput *videoInput = [AVCaptureDeviceInput deviceInputWithDevice:videoDevice error:&error];
    if (error) {
        NSLog(@"Error getting video input: %@", error.localizedDescription);
        return;
    }
    [self.captureSession beginConfiguration];
    if ([self.captureSession canSetSessionPreset:AVCaptureSessionPreset1280x720]) {
    NSLog(@"Setting video input 1270");
        self.captureSession.sessionPreset = AVCaptureSessionPreset1280x720;
    }
    if ([self.captureSession canAddInput:videoInput]) {
        [self.captureSession addInput:videoInput];
    } else {
        NSLog(@"Cannot add video input");
        return;
    }
    NSDictionary *settings = @{ (NSString *)kCVPixelBufferPixelFormatTypeKey : @(kCVPixelFormatType_32BGRA) };
    self.videoOutput.videoSettings = settings;
    [self.videoOutput setAlwaysDiscardsLateVideoFrames:YES];
    [self.videoOutput setSampleBufferDelegate:self queue:self.sessionQueue];

    if ([self.captureSession canAddOutput:self.videoOutput]) {
        [self.captureSession addOutput:self.videoOutput];
    } else {
        NSLog(@"Cannot add video output");
        return;
    }
    [self.captureSession commitConfiguration];
}

- (void)start {
    dispatch_async(self.sessionQueue, ^{
        if (!self.captureSession.isRunning) {
            [self.captureSession startRunning];
        }
    });
}

- (void)stop {
    dispatch_async(self.sessionQueue, ^{
        if (self.captureSession.isRunning) {
            [self.captureSession stopRunning];
        }
    });
}

int count = 0;

- (void)captureOutput:(AVCaptureOutput *)output didOutputSampleBuffer:(CMSampleBufferRef)sampleBuffer fromConnection:(AVCaptureConnection *)connection {
    if (!self.wantImage) {
        return;
    }
    CVImageBufferRef pixelBuffer = CMSampleBufferGetImageBuffer(sampleBuffer);
    CIImage *ciImage = [CIImage imageWithCVPixelBuffer:pixelBuffer];
    CIContext *context = [CIContext contextWithOptions:nil];

    self.gotImage = [context createCGImage:ciImage fromRect:[ciImage extent]];
    count++;
    self.wantImage = false;
}
@end

int isCameraCaptureStreamRunning(void *stream) {
    CameraCapture *cc = (CameraCapture *)stream;
    if (cc == nil) {
        return 0;
    }
    if (cc.captureSession.isRunning) {
        return 1;
    }
    return 0;
}


void *startCameraCaptureStream(void *stream) {
    CameraCapture *cc;
    if (stream == nil) {
        cc = [[CameraCapture alloc] init];
    } else {
        cc = (CameraCapture *)stream;
    }
    [ cc start ];
    return (void *)cc;
}

void stopCameraCaptureStream(void *stream) {
    CameraCapture *cc = (CameraCapture *)stream;
    [ cc stop ];
}

 int snapImageFromCaptureStream(void *stream, CGImageRef *image)  {
    CameraCapture *cc = (CameraCapture *)stream;
    // NSLog(@"snapImageFromCaptureStream: %p %d %d", cc, cc.wantImage, cc.gotImage != nil);
    if (cc.gotImage == nil) {
        cc.wantImage = true;
        return 0;
    }
    *image = cc.gotImage;
    cc.gotImage = nil;
    cc.wantImage = false;
    return 1;
}
