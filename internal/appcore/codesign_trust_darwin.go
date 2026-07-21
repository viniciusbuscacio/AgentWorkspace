//go:build darwin

package appcore

/*
#cgo CFLAGS: -x objective-c -fobjc-arc -fblocks
#cgo LDFLAGS: -framework Foundation -framework Security
#import <Foundation/Foundation.h>
#import <Security/Security.h>
#include <stdlib.h>
#include <string.h>

static char* aw_ct_strdup(NSString *s) {
    if (!s) return NULL;
    const char *utf8 = [s UTF8String];
    if (!utf8) return NULL;
    size_t n = strlen(utf8);
    char *out = (char *)malloc(n + 1);
    if (!out) return NULL;
    memcpy(out, utf8, n + 1);
    return out;
}

// aw_self_leaf_cert returns the leaf certificate of THIS app's code signature
// (NULL for ad-hoc/unsigned builds). Caller releases.
static SecCertificateRef aw_self_leaf_cert(void) {
    SecCodeRef code = NULL;
    if (SecCodeCopySelf(kSecCSDefaultFlags, &code) != errSecSuccess || !code) return NULL;
    CFDictionaryRef info = NULL;
    OSStatus st = SecCodeCopySigningInformation((SecStaticCodeRef)code, kSecCSSigningInformation, &info);
    CFRelease(code);
    if (st != errSecSuccess || !info) return NULL;
    CFArrayRef certs = CFDictionaryGetValue(info, kSecCodeInfoCertificates);
    SecCertificateRef leaf = NULL;
    if (certs && CFArrayGetCount(certs) > 0) {
        leaf = (SecCertificateRef)CFArrayGetValueAtIndex(certs, 0);
        CFRetain(leaf);
    }
    CFRelease(info);
    return leaf;
}

// aw_codesign_trust_status: 0 = trusted, 1 = untrusted, 2 = no certificate
// (ad-hoc), 3 = could not inspect.
static int aw_codesign_trust_status(char **nameOut) {
    @autoreleasepool {
        SecCertificateRef leaf = aw_self_leaf_cert();
        if (!leaf) return 2;
        if (nameOut) {
            CFStringRef cn = NULL;
            if (SecCertificateCopyCommonName(leaf, &cn) == errSecSuccess && cn) {
                *nameOut = aw_ct_strdup((__bridge NSString *)cn);
                CFRelease(cn);
            }
        }
        SecPolicyRef policy = SecPolicyCreateWithProperties(kSecPolicyAppleCodeSigning, NULL);
        SecTrustRef trust = NULL;
        int result = 3;
        if (policy && SecTrustCreateWithCertificates(leaf, policy, &trust) == errSecSuccess && trust) {
            CFErrorRef terr = NULL;
            result = SecTrustEvaluateWithError(trust, &terr) ? 0 : 1;
            if (terr) CFRelease(terr);
        }
        if (trust) CFRelease(trust);
        if (policy) CFRelease(policy);
        CFRelease(leaf);
        return result;
    }
}

// aw_codesign_trust_grant marks this app's (self-signed) leaf certificate as
// trusted for CODE SIGNING in the current user's trust settings. macOS shows
// its own authorization dialog; the user confirms with their account password.
static int aw_codesign_trust_grant(char **errOut) {
    @autoreleasepool {
        SecCertificateRef leaf = aw_self_leaf_cert();
        if (!leaf) {
            if (errOut) *errOut = aw_ct_strdup(@"The app has no signing certificate (ad-hoc build).");
            return 0;
        }
        SecPolicyRef policy = SecPolicyCreateWithProperties(kSecPolicyAppleCodeSigning, NULL);
        NSDictionary *settings = @{
            (__bridge id)kSecTrustSettingsPolicy: (__bridge id)policy,
            (__bridge id)kSecTrustSettingsResult: @(kSecTrustSettingsResultTrustRoot),
        };
        OSStatus st = SecTrustSettingsSetTrustSettings(leaf, kSecTrustSettingsDomainUser, (__bridge CFTypeRef)settings);
        if (policy) CFRelease(policy);
        CFRelease(leaf);
        if (st != errSecSuccess) {
            if (errOut) {
                CFStringRef msg = SecCopyErrorMessageString(st, NULL);
                if (msg) {
                    *errOut = aw_ct_strdup((__bridge NSString *)msg);
                    CFRelease(msg);
                } else {
                    *errOut = aw_ct_strdup([NSString stringWithFormat:@"Trust settings error %d", (int)st]);
                }
            }
            return 0;
        }
        return 1;
    }
}
*/
import "C"

import (
	"fmt"
	"unsafe"
)

func codesignTrustStatusNative() (status int, certName string) {
	var name *C.char
	st := int(C.aw_codesign_trust_status(&name))
	if name != nil {
		certName = C.GoString(name)
		C.free(unsafe.Pointer(name))
	}
	return st, certName
}

func codesignTrustGrantNative() error {
	var err *C.char
	if C.aw_codesign_trust_grant(&err) == 1 {
		return nil
	}
	msg := ""
	if err != nil {
		msg = C.GoString(err)
		C.free(unsafe.Pointer(err))
	}
	if msg == "" {
		msg = "could not update the trust settings"
	}
	return fmt.Errorf("%s", msg)
}
