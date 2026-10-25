//go:build linux && cgo

#include <X11/Xlib.h>
#include <X11/Xutil.h>

void blockX11Window(unsigned long window, int blocked) {
    Display *display = XOpenDisplay(NULL);
    if (!display) return;
    XWMHints *hints = XGetWMHints(display, window);
    if (!hints) hints = XAllocWMHints();
    if (hints) {
        hints->flags |= InputHint;
        hints->input = !blocked;
        XSetWMHints(display, window, hints);
        XFree(hints);
    }
    XCloseDisplay(display);
}

void setX11ModalParent(unsigned long parent, unsigned long child) {
    Display *display = XOpenDisplay(NULL);
    if (!display) return;
    XSetTransientForHint(display, child, parent);
    Atom state = XInternAtom(display, "_NET_WM_STATE", False);
    Atom modal = XInternAtom(display, "_NET_WM_STATE_MODAL", False);
    // Preserve any other state flags, including maximized/fullscreen.
    XEvent event = {0};
    event.xclient.type = ClientMessage;
    event.xclient.window = child;
    event.xclient.message_type = state;
    event.xclient.format = 32;
    event.xclient.data.l[0] = 1; // _NET_WM_STATE_ADD
    event.xclient.data.l[1] = modal;
    event.xclient.data.l[3] = 1; // application
    XSendEvent(display, DefaultRootWindow(display), False,
               SubstructureRedirectMask | SubstructureNotifyMask, &event);
    XCloseDisplay(display);
}