#import <Cocoa/Cocoa.h>
#include <stdlib.h>

extern void tokenManagerDesktopTitlebarAction(char *action);

@interface TMTitlebarAccessoryTarget : NSObject
@end

@implementation TMTitlebarAccessoryTarget
- (void)handleAction:(id)sender {
	NSString *action = nil;
	if ([sender respondsToSelector:@selector(identifier)]) {
		action = [sender identifier];
	}
	if (action == nil) {
		return;
	}
	tokenManagerDesktopTitlebarAction((char *)[action UTF8String]);
}
@end

static NSTitlebarAccessoryViewController *tmTitlebarController = nil;
static TMTitlebarAccessoryTarget *tmTitlebarTarget = nil;
static NSTextField *tmTitlebarLabel = nil;
static NSWindow *tmTitlebarWindow = nil;

static NSWindow *TMCurrentWindow(void) {
	NSWindow *window = [NSApp keyWindow];
	if (window != nil) {
		return window;
	}
	window = [NSApp mainWindow];
	if (window != nil) {
		return window;
	}
	NSArray *windows = [NSApp windows];
	if ([windows count] > 0) {
		return [windows objectAtIndex:0];
	}
	return nil;
}

static NSButton *TMToolbarButton(NSString *title, NSString *action) {
	NSButton *button = [NSButton buttonWithTitle:title target:tmTitlebarTarget action:@selector(handleAction:)];
	[button setBezelStyle:NSBezelStyleRounded];
	[button setControlSize:NSControlSizeSmall];
	if ([button respondsToSelector:@selector(setIdentifier:)]) {
		[button setIdentifier:action];
	}
	return button;
}

static void TMEnsureTitlebarAccessory(void) {
	NSWindow *window = TMCurrentWindow();
	if (window == nil) {
		return;
	}
	if (tmTitlebarTarget == nil) {
		tmTitlebarTarget = [TMTitlebarAccessoryTarget new];
	}
	if (tmTitlebarController == nil) {
		tmTitlebarController = [NSTitlebarAccessoryViewController new];
		tmTitlebarController.layoutAttribute = NSLayoutAttributeRight;

		NSStackView *stack = [NSStackView stackViewWithViews:@[]];
		[stack setOrientation:NSUserInterfaceLayoutOrientationHorizontal];
		[stack setSpacing:8];
		[stack setAlignment:NSLayoutAttributeCenterY];
		[stack setEdgeInsets:NSEdgeInsetsMake(0, 0, 0, 8)];

		tmTitlebarLabel = [NSTextField labelWithString:@"当前：未加载"];
		[tmTitlebarLabel setTextColor:[NSColor secondaryLabelColor]];
		[tmTitlebarLabel setFont:[NSFont systemFontOfSize:12 weight:NSFontWeightMedium]];
		[tmTitlebarLabel setLineBreakMode:NSLineBreakByTruncatingTail];
		[tmTitlebarLabel setMaximumNumberOfLines:1];
		[tmTitlebarLabel setPreferredMaxLayoutWidth:320];

		[stack addArrangedSubview:tmTitlebarLabel];
		[stack addArrangedSubview:TMToolbarButton(@"查看额度", @"titlebar_show_quota")];
		[stack addArrangedSubview:TMToolbarButton(@"刷新额度", @"titlebar_refresh_usage")];
		[stack addArrangedSubview:TMToolbarButton(@"自动检查", @"titlebar_run_auto_switch")];
		tmTitlebarController.view = stack;
	}

	if (tmTitlebarWindow != window) {
		[window addTitlebarAccessoryViewController:tmTitlebarController];
		tmTitlebarWindow = window;
	}
}

void TMInstallTitlebarAccessory(const char *summary) {
	dispatch_async(dispatch_get_main_queue(), ^{
		TMEnsureTitlebarAccessory();
		if (summary != NULL && tmTitlebarLabel != nil) {
			[tmTitlebarLabel setStringValue:[NSString stringWithUTF8String:summary]];
		}
	});
}

void TMUpdateTitlebarAccessory(const char *summary) {
	dispatch_async(dispatch_get_main_queue(), ^{
		TMEnsureTitlebarAccessory();
		if (summary != NULL && tmTitlebarLabel != nil) {
			[tmTitlebarLabel setStringValue:[NSString stringWithUTF8String:summary]];
		}
	});
}

void TMRemoveTitlebarAccessory(void) {
	dispatch_async(dispatch_get_main_queue(), ^{
		tmTitlebarWindow = nil;
		tmTitlebarController = nil;
		tmTitlebarLabel = nil;
	});
}
