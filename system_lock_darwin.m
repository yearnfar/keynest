#import <AppKit/AppKit.h>
#import <stdatomic.h>

static atomic_bool keynestSystemLockPending = ATOMIC_VAR_INIT(false);

@interface KeynestSystemLockMonitor : NSObject
- (void)lockVault:(NSNotification *)notification;
@end

@implementation KeynestSystemLockMonitor
- (void)lockVault:(NSNotification *)notification {
    atomic_store_explicit(&keynestSystemLockPending, true, memory_order_release);
}
@end

static KeynestSystemLockMonitor *keynestSystemLockMonitor;

void keynest_start_system_lock_monitor(void) {
    @synchronized([KeynestSystemLockMonitor class]) {
        atomic_store_explicit(&keynestSystemLockPending, false, memory_order_release);
        if (keynestSystemLockMonitor != nil) {
            return;
        }

        keynestSystemLockMonitor = [[KeynestSystemLockMonitor alloc] init];
        NSNotificationCenter *workspaceCenter = [[NSWorkspace sharedWorkspace] notificationCenter];
        [workspaceCenter addObserver:keynestSystemLockMonitor
                           selector:@selector(lockVault:)
                               name:NSWorkspaceWillSleepNotification
                             object:nil];
        [workspaceCenter addObserver:keynestSystemLockMonitor
                           selector:@selector(lockVault:)
                               name:NSWorkspaceDidWakeNotification
                             object:nil];
        [workspaceCenter addObserver:keynestSystemLockMonitor
                           selector:@selector(lockVault:)
                               name:NSWorkspaceSessionDidResignActiveNotification
                             object:nil];
        [[NSDistributedNotificationCenter defaultCenter]
            addObserver:keynestSystemLockMonitor
               selector:@selector(lockVault:)
                   name:@"com.apple.screenIsLocked"
                 object:nil];
    }
}

void keynest_stop_system_lock_monitor(void) {
    @synchronized([KeynestSystemLockMonitor class]) {
        if (keynestSystemLockMonitor == nil) {
            return;
        }

        [[[NSWorkspace sharedWorkspace] notificationCenter] removeObserver:keynestSystemLockMonitor];
        [[NSDistributedNotificationCenter defaultCenter] removeObserver:keynestSystemLockMonitor];
        [keynestSystemLockMonitor release];
        keynestSystemLockMonitor = nil;
        atomic_store_explicit(&keynestSystemLockPending, false, memory_order_release);
    }
}

int keynest_take_system_lock_event(void) {
    return atomic_exchange_explicit(&keynestSystemLockPending, false, memory_order_acq_rel) ? 1 : 0;
}
