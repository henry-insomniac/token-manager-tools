#import <Cocoa/Cocoa.h>
#import <dispatch/dispatch.h>
#include <stdlib.h>
#include <string.h>

extern void tokenManagerDesktopStatusItemAction(char *action);

@interface TMStatusItemTarget : NSObject
@end

@implementation TMStatusItemTarget
- (void)handleMenuItem:(id)sender {
	NSString *action = [sender representedObject];
	if (action == nil) {
		return;
	}
	tokenManagerDesktopStatusItemAction((char *)[action UTF8String]);
}
@end

static NSStatusItem *tmStatusItem = nil;
static TMStatusItemTarget *tmStatusItemTarget = nil;

static NSArray *TMParseMenuItemsJSON(const char *menuJSON) {
	if (menuJSON == NULL) {
		return @[];
	}
	NSData *data = [NSData dataWithBytes:menuJSON length:strlen(menuJSON)];
	if (data == nil) {
		return @[];
	}
	NSError *error = nil;
	id value = [NSJSONSerialization JSONObjectWithData:data options:0 error:&error];
	if (error != nil || ![value isKindOfClass:[NSArray class]]) {
		return @[];
	}
	return (NSArray *)value;
}

static NSMenu *TMBuildMenu(NSArray *items);

static NSMenuItem *TMBuildMenuItem(NSDictionary *item) {
	if ([[item objectForKey:@"separator"] boolValue]) {
		return [NSMenuItem separatorItem];
	}

	NSString *title = [item objectForKey:@"title"];
	if (title == nil) {
		title = @"";
	}
	NSMenuItem *menuItem = [[NSMenuItem alloc] initWithTitle:title action:nil keyEquivalent:@""];
	[menuItem setEnabled:![[item objectForKey:@"disabled"] boolValue]];

	NSArray *children = [item objectForKey:@"children"];
	if ([children isKindOfClass:[NSArray class]] && [children count] > 0) {
		[menuItem setSubmenu:TMBuildMenu(children)];
		return menuItem;
	}

	NSString *action = [item objectForKey:@"action"];
	if (action != nil && [action length] > 0) {
		[menuItem setTarget:tmStatusItemTarget];
		[menuItem setAction:@selector(handleMenuItem:)];
		[menuItem setRepresentedObject:action];
	}
	return menuItem;
}

static NSMenu *TMBuildMenu(NSArray *items) {
	NSMenu *menu = [[NSMenu alloc] initWithTitle:@""];
	for (id rawItem in items) {
		if (![rawItem isKindOfClass:[NSDictionary class]]) {
			continue;
		}
		[menu addItem:TMBuildMenuItem((NSDictionary *)rawItem)];
	}
	return menu;
}

static void TMRunOnMainThreadSync(dispatch_block_t block) {
	if ([NSThread isMainThread]) {
		block();
		return;
	}
	dispatch_sync(dispatch_get_main_queue(), block);
}

static void TMEnsureStatusItem(void) {
	if (tmStatusItemTarget == nil) {
		tmStatusItemTarget = [TMStatusItemTarget new];
		NSLog(@"[TokenManagerStatusItem] created target");
	}
	if (tmStatusItem == nil) {
		tmStatusItem = [[NSStatusBar systemStatusBar] statusItemWithLength:NSVariableStatusItemLength];
		if ([tmStatusItem respondsToSelector:@selector(setVisible:)]) {
			[tmStatusItem setVisible:YES];
		}
		NSLog(@"[TokenManagerStatusItem] created status item: %@", tmStatusItem);
	}
}

void TMApplyStatusItem(const char *title, const char *tooltip, const char *menuJSON) {
	TMRunOnMainThreadSync(^{
		TMEnsureStatusItem();
		NSString *titleString = (title != NULL && strlen(title) > 0) ? [NSString stringWithUTF8String:title] : @"TM";
		NSString *tooltipString = (tooltip != NULL) ? [NSString stringWithUTF8String:tooltip] : @"";
		NSArray *items = TMParseMenuItemsJSON(menuJSON);
		NSMenu *menu = TMBuildMenu(items);
		NSLog(@"[TokenManagerStatusItem] applying title=%@ items=%lu", titleString, (unsigned long)[items count]);
		[tmStatusItem setLength:NSVariableStatusItemLength];
		if ([tmStatusItem respondsToSelector:@selector(setVisible:)]) {
			[tmStatusItem setVisible:YES];
		}
		if (tmStatusItem.button != nil) {
			[tmStatusItem.button setTitle:titleString];
			[tmStatusItem.button setToolTip:tooltipString];
			NSLog(@"[TokenManagerStatusItem] button=%@ title=%@ visible=%d", tmStatusItem.button, tmStatusItem.button.title, [tmStatusItem isVisible]);
		} else {
			NSLog(@"[TokenManagerStatusItem] button is nil");
		}
		[tmStatusItem setMenu:menu];
	});
}

void TMRemoveStatusItem(void) {
	TMRunOnMainThreadSync(^{
		if (tmStatusItem != nil) {
			[[NSStatusBar systemStatusBar] removeStatusItem:tmStatusItem];
			tmStatusItem = nil;
		}
	});
}

char *TMStatusItemDebugState(void) {
	__block NSString *state = nil;
	TMRunOnMainThreadSync(^{
		NSString *buttonTitle = tmStatusItem.button != nil ? tmStatusItem.button.title : @"<nil>";
		NSUInteger menuCount = tmStatusItem.menu != nil ? [tmStatusItem.menu.itemArray count] : 0;
		state = [NSString stringWithFormat:@"item=%@ button=%@ title=%@ visible=%d menuItems=%lu", tmStatusItem, tmStatusItem.button, buttonTitle, tmStatusItem != nil, (unsigned long)menuCount];
	});
	if (state == nil) {
		return strdup("<nil>");
	}
	return strdup([state UTF8String]);
}
