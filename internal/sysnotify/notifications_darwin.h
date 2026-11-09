#include <stdint.h>
int kzNotificationSupportsBundle(const char*,const char*);
char *kzNotificationRequestKey(const char*);
void kzNotificationStart(uint64_t);
void kzNotificationPost(uint64_t,const char*,const char*,const char*,const char*);
void kzNotificationRemove(uint64_t,const char*);
void kzNotificationCheck(uint64_t);
void kzNotificationRetry(uint64_t);
void kzNotificationWindow(uint64_t,uintptr_t);
void kzNotificationRaise(uint64_t);
void kzNotificationClose(uint64_t);
