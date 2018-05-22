// Copyright (c) 2018 The Kura Authors.
//
// Copyright (c) 2013-2015 Tommi Virtanen.
// All rights reserved.
//
// Redistribution and use in source and binary forms, with or without
// modification, are permitted provided that the following conditions are
// met:
//
//  * Redistributions of source code must retain the above copyright
//    notice, this list of conditions and the following disclaimer.
//  * Redistributions in binary form must reproduce the above copyright
//    notice, this list of conditions and the following disclaimer in the
//    documentation and/or other materials provided with the distribution.
//  * Neither the name of Google Inc. nor the names of its contributors may be
//    used to endorse or promote products derived from this software without
//    specific prior written permission.
//
// THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS
// "AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT
// LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR
// A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT
// OWNER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL,
// SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT
// LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE,
// DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY
// THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
// (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
// OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.

package xattr

/* This is the source file for xattr_darwin_*.go, to regenerate run

   sh generate

*/

// It's only when dest is set to NULL that the OS X implementations of
// getxattr() and listxattr() return the current sizes of the named attributes.
// An empty byte array is not sufficient. To maintain the same behaviour as the
// linux implementation, we wrap around the system calls and pass in NULL when
// dest is empty.

//sys getxattr(path string, attr string, dest *byte, size int, position uint32, options int) (sz int, err error)

func Getxattr(path string, attr string, dest []byte) (sz int, err error) {
	var destp *byte
	if len(dest) > 0 {
		destp = &dest[0]
	}
	return getxattr(path, attr, destp, len(dest), 0, 0)
}

//sys  listxattr(path string, dest *byte, size int, options int) (sz int, err error)

func Listxattr(path string, dest []byte) (sz int, err error) {
	var destp *byte
	if len(dest) > 0 {
		destp = &dest[0]
	}
	return listxattr(path, destp, len(dest), 0)
}

//sys  setxattr(path string, attr string, data *byte, size int, position uint32, options int) (err error)

func Setxattr(path string, attr string, data []byte, flags int) (err error) {
	// The parameters for the OS X implementation vary slightly compared to the
	// linux system call, specifically the position parameter:
	//
	//  linux:
	//      int setxattr(
	//          const char *path,
	//          const char *name,
	//          const void *value,
	//          size_t size,
	//          int flags
	//      );
	//
	//  darwin:
	//      int setxattr(
	//          const char *path,
	//          const char *name,
	//          void *value,
	//          size_t size,
	//          u_int32_t position,
	//          int options
	//      );
	//
	// position specifies the offset within the extended attribute. In the
	// current implementation, only the resource fork extended attribute makes
	// use of this argument. For all others, position is reserved. We simply
	// default to setting it to zero. If that's needed by the package user, a
	// function with a different name needs to be implemented instead.
	var datap *byte
	if len(data) > 0 {
		datap = &data[0]
	}
	return setxattr(path, attr, datap, len(data), 0, flags)
}

//sys removexattr(path string, attr string, options int) (err error)

func Removexattr(path string, attr string) (err error) {
	return removexattr(path, attr, 0)
}
