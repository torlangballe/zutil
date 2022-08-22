#import <Foundation/Foundation.h>

#include <string.h>

char *getString(NSString *nsStr, int max) {
    char *c = malloc(max);

    if(![nsStr getCString:c maxLength:max encoding:NSUTF8StringEncoding]) {
        NSLog(@"getString error: %d\n", max);
        return nil;
    }
    return c;
}


// char *GetOSVersion() {
//     NSProcessInfo *pInfo = [NSProcessInfo processInfo];
//     NSString *version = [pInfo operatingSystemVersionString];
//     return getString(version, 100);
// }