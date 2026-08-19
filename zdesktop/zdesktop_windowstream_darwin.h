#ifndef ZDESKTOP_WINDOWSTREAM_DARWIN_H
#define ZDESKTOP_WINDOWSTREAM_DARWIN_H

#include <stdint.h>
#include <CoreGraphics/CoreGraphics.h>

void EnsureCocoaGraphicsInitialized(void);
void *startWindowCaptureStream(const char *winTitle, const char *appBundleID, int captureAudio, char *err, int errLen);
void *startWindowCaptureStreamWithCropAndDestSize(const char *winTitle,
												  const char *appBundleID,
												  int captureAudio,
												  CGRect cropRect,
												  CGSize destSize,
												  char *err,
												  int errLen);
void stopWindowCaptureStream(void *stream);
int isWindowCaptureStreamRunning(void *stream);
int copyLatestWindowStreamH264(void *stream, unsigned char **data, int *length, int64_t *ptsNS);
int copyLatestWindowStreamAudioOpus(void *stream, unsigned char **data, int *byteLength, int *sampleRate, int *channels, int64_t *ptsNS);

#endif
