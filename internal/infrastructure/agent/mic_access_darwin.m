#import <AVFoundation/AVFoundation.h>

// Requests microphone access AS THE APP. TCC attributes the request to the
// bundle (which carries NSMicrophoneUsageDescription), so the system prompt
// actually appears; ffmpeg alone is a bare binary that macOS denies without
// ever prompting. Once the app holds the grant, spawned children inherit it.
// Blocks while the system prompt is on screen (first run only).
// Returns 1 granted, 0 denied/restricted.
int aw_request_mic_access(void) {
    AVAuthorizationStatus st = [AVCaptureDevice authorizationStatusForMediaType:AVMediaTypeAudio];
    if (st == AVAuthorizationStatusAuthorized) return 1;
    if (st == AVAuthorizationStatusDenied || st == AVAuthorizationStatusRestricted) return 0;
    dispatch_semaphore_t sem = dispatch_semaphore_create(0);
    __block int granted = 0;
    [AVCaptureDevice requestAccessForMediaType:AVMediaTypeAudio completionHandler:^(BOOL ok) {
        granted = ok ? 1 : 0;
        dispatch_semaphore_signal(sem);
    }];
    dispatch_semaphore_wait(sem, DISPATCH_TIME_FOREVER);
    return granted;
}
