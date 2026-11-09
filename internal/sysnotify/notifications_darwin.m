#import <AppKit/AppKit.h>
#import <UserNotifications/UserNotifications.h>
#import "notifications_darwin.h"
#include <string.h>

extern void kazeNotificationActivated(uint64_t, const char *);
extern void kazeNotificationStatus(uint64_t, const char *);

@interface KZNotificationState : NSObject
@property uint64_t identifier;
@property (weak) NSWindow *window;
@property BOOL asking;
@property BOOL requested;
@property UNAuthorizationStatus authorization;
@property NSMutableDictionary<NSString *, UNMutableNotificationContent *> *desired;
@end
@implementation KZNotificationState
@end

@interface KZNotificationDelegate : NSObject <UNUserNotificationCenterDelegate>
@end

// All native state belongs to AppKit's main queue. No callback calls Gio inline.
static NSMutableDictionary<NSNumber *, KZNotificationState *> *kzStates;
static KZNotificationDelegate *kzDelegate;

static NSString *kzKey(NSString *activation) {
    NSArray<NSString *> *parts = [activation componentsSeparatedByString:@"-"];
    if (parts.count != 3) return nil;
    NSCharacterSet *bad = [[NSCharacterSet characterSetWithCharactersInString:@"0123456789abcdef"] invertedSet];
    for (NSString *part in parts) {
        if (part.length == 0 || part.length > 16 || [part rangeOfCharacterFromSet:bad].location != NSNotFound) return nil;
    }
    return [NSString stringWithFormat:@"%@-%@", parts[0], parts[1]];
}

int kzNotificationSupportsBundle(const char *path, const char *identifier) {
    @autoreleasepool {
    NSString *bundlePath = [NSString stringWithUTF8String:path];
    NSString *bundleID = [NSString stringWithUTF8String:identifier];
    return [bundlePath hasSuffix:@".app"] && [bundleID isEqual:@"com.patrickgogo.kaze"];
    }
}
char *kzNotificationRequestKey(const char *activation) {
    @autoreleasepool {
    NSString *key = kzKey([NSString stringWithUTF8String:activation]);
    return key == nil ? NULL : strdup(key.UTF8String);
    }
}

static void kzStatus(KZNotificationState *s, NSString *message) {
    if (kzStates[@(s.identifier)] == s) {
        kazeNotificationStatus(s.identifier, message.UTF8String);
    }
}

static void kzSubmit(KZNotificationState *s, NSString *key) {
    UNMutableNotificationContent *content = s.desired[key];
    if (content == nil || kzStates[@(s.identifier)] != s) return;
    if ([NSApp isActive]) { // permission completed after the owner returned
        [s.desired removeObjectForKey:key];
        return;
    }
    if (s.authorization != UNAuthorizationStatusAuthorized && s.authorization != UNAuthorizationStatusProvisional) return;
    UNUserNotificationCenter *center = [UNUserNotificationCenter currentNotificationCenter];
    [center removePendingNotificationRequestsWithIdentifiers:@[key]];
    [center removeDeliveredNotificationsWithIdentifiers:@[key]];
    UNNotificationRequest *request = [UNNotificationRequest requestWithIdentifier:key content:content trigger:nil];
    [center addNotificationRequest:request withCompletionHandler:^(NSError *error) {
        if (error != nil) {
            dispatch_async(dispatch_get_main_queue(), ^{
                if (s.desired[key] == content) {
                    kzStatus(s, [NSString stringWithFormat:@"Notifications unavailable: %@", error.localizedDescription]);
                }
            });
        }
    }];
}

static void kzCheck(KZNotificationState *s) {
    if (s == nil || s.asking) return;
    UNUserNotificationCenter *center = [UNUserNotificationCenter currentNotificationCenter];
    [center getNotificationSettingsWithCompletionHandler:^(UNNotificationSettings *settings) {
        dispatch_async(dispatch_get_main_queue(), ^{
            if (kzStates[@(s.identifier)] != s) return;
            s.authorization = settings.authorizationStatus;
            if (settings.authorizationStatus == UNAuthorizationStatusNotDetermined) {
                if (s.asking || s.requested) return;
                s.asking = YES;
                s.requested = YES;
                kzStatus(s, @"Waiting for notification permission");
                [center requestAuthorizationWithOptions:UNAuthorizationOptionAlert completionHandler:^(BOOL allowed, NSError *error) {
                    dispatch_async(dispatch_get_main_queue(), ^{
                        if (kzStates[@(s.identifier)] != s) return;
                        s.asking = NO;
                        s.authorization = allowed ? UNAuthorizationStatusAuthorized : UNAuthorizationStatusDenied;
                        if (error != nil) {
                            kzStatus(s, [NSString stringWithFormat:@"Notifications unavailable: %@", error.localizedDescription]);
                        } else {
                            kzStatus(s, allowed ? @"" : @"Blocked in macOS notification settings");
                        }
                        if (allowed) for (NSString *key in [s.desired.allKeys copy]) {
                            UNMutableNotificationContent *content = s.desired[key];
                            NSMutableDictionary *info = [content.userInfo mutableCopy];
                            info[@"pending"] = @NO;
                            content.userInfo = info;
                            kzSubmit(s, key);
                        }
                        else [s.desired removeAllObjects];
                    });
                }];
            } else {
                BOOL allowed = settings.authorizationStatus == UNAuthorizationStatusAuthorized || settings.authorizationStatus == UNAuthorizationStatusProvisional;
                kzStatus(s, allowed ? @"" : @"Blocked in macOS notification settings");
                // These were deferred for authorization, never periodic replay.
                if (allowed) for (NSString *key in [s.desired.allKeys copy]) {
                    UNMutableNotificationContent *c = s.desired[key];
                    if ([c.userInfo[@"pending"] boolValue]) {
                        NSMutableDictionary *info = [c.userInfo mutableCopy];
                        info[@"pending"] = @NO;
                        c.userInfo = info;
                        kzSubmit(s, key);
                    }
                }
            }
        });
    }];
}

@implementation KZNotificationDelegate
- (void)userNotificationCenter:(UNUserNotificationCenter *)center willPresentNotification:(UNNotification *)notification withCompletionHandler:(void (^)(UNNotificationPresentationOptions))completion {
    dispatch_async(dispatch_get_main_queue(), ^{
        NSDictionary *info = notification.request.content.userInfo;
        KZNotificationState *s = kzStates[info[@"backend"]];
        NSString *activation = info[@"activation"];
        BOOL current = s != nil && [s.desired[notification.request.identifier].userInfo[@"activation"] isEqual:activation];
        if ([NSApp isActive] || !current) {
            completion(UNNotificationPresentationOptionNone);
        } else if (@available(macOS 11.0, *)) {
            completion(UNNotificationPresentationOptionBanner | UNNotificationPresentationOptionList);
        } else {
            completion(UNNotificationPresentationOptionAlert);
        }
    });
}
- (void)userNotificationCenter:(UNUserNotificationCenter *)center didReceiveNotificationResponse:(UNNotificationResponse *)response withCompletionHandler:(void (^)(void))completion {
    dispatch_async(dispatch_get_main_queue(), ^{
        NSDictionary *info = response.notification.request.content.userInfo;
        KZNotificationState *s = kzStates[info[@"backend"]];
        NSString *activation = info[@"activation"];
        NSString *key = response.notification.request.identifier;
        if ([response.actionIdentifier isEqual:UNNotificationDefaultActionIdentifier] && s != nil && [s.desired[key].userInfo[@"activation"] isEqual:activation]) {
            kazeNotificationActivated(s.identifier, activation.UTF8String);
        }
        completion();
    });
}
@end

void kzNotificationStart(uint64_t identifier) {
    @autoreleasepool {
    dispatch_async(dispatch_get_main_queue(), ^{
        NSBundle *bundle = [NSBundle mainBundle];
        // Calling currentNotificationCenter without an application bundle can
        // abort. Tests and ordinary `go run` binaries must never reach it.
        if (!kzNotificationSupportsBundle(bundle.bundlePath.UTF8String ?: "", bundle.bundleIdentifier.UTF8String ?: "")) {
            kazeNotificationStatus(identifier, "Available in the Kaze app bundle");
            return;
        }
        if (kzStates == nil) kzStates = [NSMutableDictionary dictionary];
        KZNotificationState *s = [KZNotificationState new];
        s.identifier = identifier;
        s.authorization = UNAuthorizationStatusNotDetermined;
        s.desired = [NSMutableDictionary dictionary];
        kzStates[@(identifier)] = s;
        if (kzDelegate == nil) kzDelegate = [KZNotificationDelegate new];
        [UNUserNotificationCenter currentNotificationCenter].delegate = kzDelegate;
    });
    }
}

void kzNotificationPost(uint64_t identifier, const char *id, const char *title, const char *context, const char *body) {
    @autoreleasepool {
    // Copy all C strings before returning to Go and freeing their memory.
    NSString *activation = [NSString stringWithUTF8String:id];
    NSString *key = kzKey(activation);
    if (key == nil) return;
    UNMutableNotificationContent *content = [UNMutableNotificationContent new];
    content.title = [NSString stringWithUTF8String:title] ?: @"Kaze";
    content.subtitle = [NSString stringWithUTF8String:context] ?: @"";
    content.body = [NSString stringWithUTF8String:body] ?: @"";
    content.sound = nil;
    content.userInfo = @{@"backend": @(identifier), @"activation": activation, @"pending": @YES};
    dispatch_async(dispatch_get_main_queue(), ^{
        KZNotificationState *s = kzStates[@(identifier)];
        if (s == nil || [NSApp isActive]) return;
        // Core owns the main bound; retain a defensive bound at the native edge.
        if (s.desired[key] == nil && s.desired.count >= 256) {
            kazeNotificationStatus(identifier, "Too many pending notifications");
            return;
        }
        s.desired[key] = content;
        if (s.authorization == UNAuthorizationStatusNotDetermined) return;
        NSMutableDictionary *info = [content.userInfo mutableCopy];
        info[@"pending"] = @NO;
        content.userInfo = info;
        kzSubmit(s, key);
    });
    }
}

void kzNotificationRemove(uint64_t identifier, const char *id) {
    @autoreleasepool {
    NSString *activation = [NSString stringWithUTF8String:id];
    NSString *key = kzKey(activation);
    if (key == nil) return;
    dispatch_async(dispatch_get_main_queue(), ^{
        KZNotificationState *s = kzStates[@(identifier)];
        if (s == nil || ![s.desired[key].userInfo[@"activation"] isEqual:activation]) return;
        [s.desired removeObjectForKey:key];
        UNUserNotificationCenter *center = [UNUserNotificationCenter currentNotificationCenter];
        [center removePendingNotificationRequestsWithIdentifiers:@[key]];
        [center removeDeliveredNotificationsWithIdentifiers:@[key]];
    });
    }
}

void kzNotificationCheck(uint64_t identifier) {
    @autoreleasepool {
    dispatch_async(dispatch_get_main_queue(), ^{ kzCheck(kzStates[@(identifier)]); });
    }
}
void kzNotificationRetry(uint64_t identifier) {
    @autoreleasepool {
        dispatch_async(dispatch_get_main_queue(), ^{
            KZNotificationState *s = kzStates[@(identifier)];
            if (s == nil || s.asking) return;
            s.requested = NO;
            kzCheck(s);
        });
    }
}
void kzNotificationWindow(uint64_t identifier, uintptr_t handle) {
    @autoreleasepool {
    // Retain the valid Gio view through dispatch, then keep only its weak window.
    NSView *view = handle == 0 ? nil : (__bridge NSView *)(void *)handle;
    dispatch_async(dispatch_get_main_queue(), ^{ kzStates[@(identifier)].window = view.window; });
    }
}
void kzNotificationRaise(uint64_t identifier) {
    @autoreleasepool {
    dispatch_async(dispatch_get_main_queue(), ^{
        NSWindow *window = kzStates[@(identifier)].window;
        if (window == nil) return;
        [window deminiaturize:nil];
        [NSApp activateIgnoringOtherApps:YES];
        [window makeKeyAndOrderFront:nil];
    });
    }
}
void kzNotificationClose(uint64_t identifier) {
    @autoreleasepool {
    void (^cleanup)(void) = ^{
        KZNotificationState *s = kzStates[@(identifier)];
        if (s == nil) return;
        NSArray<NSString *> *keys = s.desired.allKeys;
        [kzStates removeObjectForKey:@(identifier)];
        UNUserNotificationCenter *center = [UNUserNotificationCenter currentNotificationCenter];
        [center removePendingNotificationRequestsWithIdentifiers:keys];
        [center removeDeliveredNotificationsWithIdentifiers:keys];
        if (kzStates.count == 0 && center.delegate == kzDelegate) center.delegate = nil;
    };
    if ([NSThread isMainThread]) {
        cleanup();
    } else {
        // Run returns straight to os.Exit. Give AppKit cleanup a bounded
        // completion window so normal shutdown doesn't strand notifications.
        dispatch_semaphore_t done = dispatch_semaphore_create(0);
        dispatch_async(dispatch_get_main_queue(), ^{ cleanup(); dispatch_semaphore_signal(done); });
        dispatch_semaphore_wait(done, dispatch_time(DISPATCH_TIME_NOW, NSEC_PER_SEC));
    }
    }
}
