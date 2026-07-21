//go:build darwin

package appcore

/*
#cgo CFLAGS: -x objective-c -fobjc-arc -fblocks
#cgo LDFLAGS: -framework Foundation -framework LocalAuthentication -framework Security
#import <Foundation/Foundation.h>
#import <LocalAuthentication/LocalAuthentication.h>
#import <Security/Security.h>
#include <stdlib.h>
#include <string.h>

static char* aw_strdup(NSString *s) {
    if (!s) return NULL;
    const char *utf8 = [s UTF8String];
    if (!utf8) return NULL;
    size_t n = strlen(utf8);
    char *out = (char *)malloc(n + 1);
    if (!out) return NULL;
    memcpy(out, utf8, n + 1);
    return out;
}

static char* aw_cf_error(OSStatus status) {
    CFStringRef msg = SecCopyErrorMessageString(status, NULL);
    if (msg) {
        char *out = aw_strdup((__bridge NSString *)msg);
        CFRelease(msg);
        if (out) return out;
    }
    NSString *fallback = [NSString stringWithFormat:@"Keychain error %d", (int)status];
    return aw_strdup(fallback);
}

static int aw_touchid_available(char **errOut) {
    @autoreleasepool {
        LAContext *ctx = [[LAContext alloc] init];
        NSError *err = nil;
        BOOL ok = [ctx canEvaluatePolicy:LAPolicyDeviceOwnerAuthenticationWithBiometrics error:&err];
        if (!ok && errOut && err) *errOut = aw_strdup(err.localizedDescription);
        return ok ? 1 : 0;
    }
}

static int aw_touchid_prompt(const char *reason, char **errOut) {
    @autoreleasepool {
        LAContext *ctx = [[LAContext alloc] init];
        NSError *canErr = nil;
        if (![ctx canEvaluatePolicy:LAPolicyDeviceOwnerAuthenticationWithBiometrics error:&canErr]) {
            if (errOut && canErr) *errOut = aw_strdup(canErr.localizedDescription);
            return 0;
        }

        NSString *localizedReason = reason ? [NSString stringWithUTF8String:reason] : @"Authenticate with Touch ID";
        dispatch_semaphore_t sem = dispatch_semaphore_create(0);
        __block BOOL authOK = NO;
        __block NSError *authErr = nil;

        [ctx evaluatePolicy:LAPolicyDeviceOwnerAuthenticationWithBiometrics localizedReason:localizedReason reply:^(BOOL success, NSError *error) {
            authOK = success;
            authErr = error;
            dispatch_semaphore_signal(sem);
        }];
        dispatch_semaphore_wait(sem, DISPATCH_TIME_FOREVER);
        if (!authOK && errOut && authErr) *errOut = aw_strdup(authErr.localizedDescription);
        return authOK ? 1 : 0;
    }
}

static NSMutableDictionary* aw_keychain_query(const char *service, const char *account) {
    NSString *svc = service ? [NSString stringWithUTF8String:service] : @"";
    NSString *acct = account ? [NSString stringWithUTF8String:account] : @"";
    return [@{
        (__bridge id)kSecClass: (__bridge id)kSecClassGenericPassword,
        (__bridge id)kSecAttrService: svc,
        (__bridge id)kSecAttrAccount: acct,
    } mutableCopy];
}

static int aw_keychain_has(const char *service, const char *account) {
    @autoreleasepool {
        NSMutableDictionary *query = aw_keychain_query(service, account);
        OSStatus status = SecItemCopyMatching((__bridge CFDictionaryRef)query, NULL);
        return (status == errSecSuccess) ? 1 : 0;
    }
}

static int aw_keychain_set(const char *service, const char *account, const char *password, char **errOut) {
    @autoreleasepool {
        NSMutableDictionary *query = aw_keychain_query(service, account);
        SecItemDelete((__bridge CFDictionaryRef)query);

        NSString *pw = password ? [NSString stringWithUTF8String:password] : @"";
        NSData *data = [pw dataUsingEncoding:NSUTF8StringEncoding];
        query[(__bridge id)kSecValueData] = data;
        query[(__bridge id)kSecAttrAccessible] = (__bridge id)kSecAttrAccessibleWhenUnlockedThisDeviceOnly;

        OSStatus status = SecItemAdd((__bridge CFDictionaryRef)query, NULL);
        if (status != errSecSuccess) {
            if (errOut) *errOut = aw_cf_error(status);
            return 0;
        }
        return 1;
    }
}

static char* aw_keychain_get(const char *service, const char *account, char **errOut) {
    @autoreleasepool {
        NSMutableDictionary *query = aw_keychain_query(service, account);
        query[(__bridge id)kSecReturnData] = @YES;
        query[(__bridge id)kSecMatchLimit] = (__bridge id)kSecMatchLimitOne;

        CFTypeRef result = NULL;
        OSStatus status = SecItemCopyMatching((__bridge CFDictionaryRef)query, &result);
        if (status != errSecSuccess) {
            if (errOut) *errOut = aw_cf_error(status);
            return NULL;
        }
        NSData *data = (__bridge_transfer NSData *)result;
        NSString *pw = [[NSString alloc] initWithData:data encoding:NSUTF8StringEncoding];
        return aw_strdup(pw);
    }
}

static int aw_keychain_delete(const char *service, const char *account, char **errOut) {
    @autoreleasepool {
        NSMutableDictionary *query = aw_keychain_query(service, account);
        OSStatus status = SecItemDelete((__bridge CFDictionaryRef)query);
        if (status != errSecSuccess && status != errSecItemNotFound) {
            if (errOut) *errOut = aw_cf_error(status);
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

func cErrorString(ptr *C.char) string {
	if ptr == nil {
		return ""
	}
	defer C.free(unsafe.Pointer(ptr))
	return C.GoString(ptr)
}

func touchIDAvailableNative() bool {
	var err *C.char
	ok := C.aw_touchid_available(&err)
	if ok == 1 {
		return true
	}
	_ = cErrorString(err)
	return false
}

func touchIDPromptNative(reason string) error {
	cReason := C.CString(reason)
	defer C.free(unsafe.Pointer(cReason))
	var err *C.char
	if C.aw_touchid_prompt(cReason, &err) == 1 {
		return nil
	}
	msg := cErrorString(err)
	if msg == "" {
		msg = "Touch ID authentication failed"
	}
	return fmt.Errorf("%s", msg)
}

func touchIDKeychainHas(service, account string) bool {
	cService := C.CString(service)
	cAccount := C.CString(account)
	defer C.free(unsafe.Pointer(cService))
	defer C.free(unsafe.Pointer(cAccount))
	return C.aw_keychain_has(cService, cAccount) == 1
}

func touchIDKeychainSet(service, account, password string) error {
	cService := C.CString(service)
	cAccount := C.CString(account)
	cPassword := C.CString(password)
	defer C.free(unsafe.Pointer(cService))
	defer C.free(unsafe.Pointer(cAccount))
	defer C.free(unsafe.Pointer(cPassword))
	var err *C.char
	if C.aw_keychain_set(cService, cAccount, cPassword, &err) == 1 {
		return nil
	}
	msg := cErrorString(err)
	if msg == "" {
		msg = "failed to store Touch ID credential"
	}
	return fmt.Errorf("%s", msg)
}

func touchIDKeychainGet(service, account string) (string, error) {
	cService := C.CString(service)
	cAccount := C.CString(account)
	defer C.free(unsafe.Pointer(cService))
	defer C.free(unsafe.Pointer(cAccount))
	var err *C.char
	pw := C.aw_keychain_get(cService, cAccount, &err)
	if pw == nil {
		msg := cErrorString(err)
		if msg == "" {
			msg = "failed to read Touch ID credential"
		}
		return "", fmt.Errorf("%s", msg)
	}
	defer C.free(unsafe.Pointer(pw))
	return C.GoString(pw), nil
}

func touchIDKeychainDelete(service, account string) error {
	cService := C.CString(service)
	cAccount := C.CString(account)
	defer C.free(unsafe.Pointer(cService))
	defer C.free(unsafe.Pointer(cAccount))
	var err *C.char
	if C.aw_keychain_delete(cService, cAccount, &err) == 1 {
		return nil
	}
	msg := cErrorString(err)
	if msg == "" {
		msg = "failed to remove Touch ID credential"
	}
	return fmt.Errorf("%s", msg)
}
