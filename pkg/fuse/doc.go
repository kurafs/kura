// See the file LICENSE for copyright and licensing information.

// Adapted from Plan 9 from User Space's src/cmd/9pfuse/fuse.c,
// which carries this notice:
//
// The files in this directory are subject to the following license.
//
// The author of this software is Russ Cox.
//
//         Copyright (c) 2006 Russ Cox
//
// Permission to use, copy, modify, and distribute this software for any
// purpose without fee is hereby granted, provided that this entire notice
// is included in all copies of any software which is or includes a copy
// or modification of this software and in all copies of the supporting
// documentation for such software.
//
// THIS SOFTWARE IS BEING PROVIDED "AS IS", WITHOUT ANY EXPRESS OR IMPLIED
// WARRANTY.  IN PARTICULAR, THE AUTHOR MAKES NO REPRESENTATION OR WARRANTY
// OF ANY KIND CONCERNING THE MERCHANTABILITY OF THIS SOFTWARE OR ITS
// FITNESS FOR ANY PARTICULAR PURPOSE.

// Package fuse enables writing FUSE file systems on Linux, OS X, and FreeBSD.
//
// On OS X, it requires OSXFUSE (http://osxfuse.github.com/).
//
// There are two approaches to writing a FUSE file system.  The first is to speak
// the low-level message protocol, reading from a Conn using ReadRequest and
// writing using the various Respond methods.  This approach is closest to
// the actual interaction with the kernel and can be the simplest one in contexts
// such as protocol translators.
//
// Servers of synthesized file systems tend to share common
// bookkeeping abstracted away by the second approach, which is to
// call fs.Serve to serve the FUSE protocol using an implementation of
// the service methods in the interfaces FS* (file system), Node* (file
// or directory), and Handle* (opened file or directory).
// There are a daunting number of such methods that can be written,
// but few are required.
// The specific methods are described in the documentation for those interfaces.
//
// The hellofs subdirectory contains a simple illustration of the fs.Serve approach.
//
// Service Methods
//
// The required and optional methods for the FS, Node, and Handle interfaces
// have the general form
//
//	Op(ctx context.Context, req *OpRequest, resp *OpResponse) error
//
// where Op is the name of a FUSE operation. Op reads request
// parameters from req and writes results to resp. An operation whose
// only result is the error result omits the resp parameter.
//
// Multiple goroutines may call service methods simultaneously; the
// methods being called are responsible for appropriate
// synchronization.
//
// The operation must not hold on to the request or response,
// including any []byte fields such as WriteRequest.Data or
// SetxattrRequest.Xattr.
//
// Errors
//
// Operations can return errors. The FUSE interface can only
// communicate POSIX errno error numbers to file system clients, the
// message is not visible to file system clients. The returned error
// can implement ErrorNumber to control the errno returned. Without
// ErrorNumber, a generic errno (EIO) is returned.
//
// Error messages will be visible in the debug log as part of the
// response.
//
// Interrupted Operations
//
// In some file systems, some operations
// may take an undetermined amount of time.  For example, a Read waiting for
// a network message or a matching Write might wait indefinitely.  If the request
// is cancelled and no longer needed, the context will be cancelled.
// Blocking operations should select on a receive from ctx.Done() and attempt to
// abort the operation early if the receive succeeds (meaning the channel is closed).
// To indicate that the operation failed because it was aborted, return fuse.EINTR.
//
// If an operation does not block for an indefinite amount of time, supporting
// cancellation is not necessary.
//
// Authentication
//
// All requests types embed a Header, meaning that the method can
// inspect req.Pid, req.Uid, and req.Gid as necessary to implement
// permission checking. The kernel FUSE layer normally prevents other
// users from accessing the FUSE file system (to change this, see
// AllowOther, AllowRoot), but does not enforce access modes (to
// change this, see DefaultPermissions).
//
// Mount Options
//
// Behavior and metadata of the mounted file system can be changed by
// passing MountOption values to Mount.
package fuse
