#ifndef ZDESKTOP_WINDOWSTREAM_DARWIN_H
#define ZDESKTOP_WINDOWSTREAM_DARWIN_H

#include <stdint.h>

void EnsureCocoaGraphicsInitialized(void);
void *startWindowCaptureStreamWithRegion(const char *winTitle, const char *appBundleID, int captureAudio, int srcOffsetX, int srcOffsetY, int srcWidth, int srcHeight, int destWidth, int destHeight, char *err, int errLen);
void stopWindowCaptureStream(void *stream);
int isWindowCaptureStreamRunning(void *stream);
int copyLatestWindowStreamH264(void *stream, unsigned char **data, int *length, int64_t *ptsNS);
int copyLatestWindowStreamAudioPCM16(void *stream, short **data, int *sampleCount, int *sampleRate, int *channels, int64_t *ptsNS);
int copyLatestWindowStreamAudioOpus(void *stream, unsigned char **data, int *length, int *sampleRate, int *channels, int64_t *durationNS, int64_t *ptsNS);

#endif
