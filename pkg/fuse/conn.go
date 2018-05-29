package fuse

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

// A Conn represents a connection to a mounted FUSE file system.
type Conn struct {
	// File handle for kernel communication. Only safe to access if
	// rio or wio is held.
	dev *os.File
	wio sync.RWMutex
	rio sync.RWMutex

	// Protocol version negotiated with InitRequest/InitResponse.
	protocol Protocol
}

// Close closes the FUSE connection.
func (c *Conn) Close() error {
	c.wio.Lock()
	defer c.wio.Unlock()
	c.rio.Lock()
	defer c.rio.Unlock()
	return c.dev.Close()
}

// caller must hold wio or rio
func (c *Conn) fd() int {
	return int(c.dev.Fd())
}

func (c *Conn) Protocol() Protocol {
	return c.protocol
}

// ReadRequest returns the next FUSE request from the kernel.
//
// Caller must call either Request.Respond or Request.RespondError in
// a reasonable time. Caller must not retain Request after that call.
func (c *Conn) ReadRequest() (Request, error) {
	m := getMessage(c)
loop:
	c.rio.RLock()
	n, err := syscall.Read(c.fd(), m.buf)
	c.rio.RUnlock()
	if err == syscall.EINTR {
		// OSXFUSE sends EINTR to userspace when a request interrupt
		// completed before it got sent to userspace?
		goto loop
	}
	if err != nil && err != syscall.ENODEV {
		putMessage(m)
		return nil, err
	}
	if n <= 0 {
		putMessage(m)
		return nil, io.EOF
	}
	m.buf = m.buf[:n]

	if n < inHeaderSize {
		putMessage(m)
		return nil, errors.New("fuse: message too short")
	}

	// FreeBSD FUSE sends a short length in the header
	// for FUSE_INIT even though the actual read length is correct.
	if n == inHeaderSize+initInSize && m.hdr.Opcode == opInit && m.hdr.Len < uint32(n) {
		m.hdr.Len = uint32(n)
	}

	// OSXFUSE sometimes sends the wrong m.hdr.Len in a FUSE_WRITE message.
	if m.hdr.Len < uint32(n) && m.hdr.Len >= uint32(unsafe.Sizeof(writeIn{})) && m.hdr.Opcode == opWrite {
		m.hdr.Len = uint32(n)
	}

	if m.hdr.Len != uint32(n) {
		// prepare error message before returning m to pool
		err := fmt.Errorf("fuse: read %d opcode %d but expected %d", n, m.hdr.Opcode, m.hdr.Len)
		putMessage(m)
		return nil, err
	}

	m.off = inHeaderSize

	// Convert to data structures.
	// Do not trust kernel to hand us well-formed data.
	var req Request
	switch m.hdr.Opcode {
	default:
		Debug(noOpcode{Opcode: m.hdr.Opcode})
		goto unrecognized

	case opLookup:
		buf := m.bytes()
		n := len(buf)
		if n == 0 || buf[n-1] != '\x00' {
			goto corrupt
		}
		req = &LookupRequest{
			Header: m.Header(),
			Name:   string(buf[:n-1]),
		}

	case opForget:
		in := (*forgetIn)(m.data())
		if m.len() < unsafe.Sizeof(*in) {
			goto corrupt
		}
		req = &ForgetRequest{
			Header: m.Header(),
			N:      in.Nlookup,
		}

	case opGetattr:
		switch {
		case c.protocol.LT(Protocol{7, 9}):
			req = &GetattrRequest{
				Header: m.Header(),
			}

		default:
			in := (*getattrIn)(m.data())
			if m.len() < unsafe.Sizeof(*in) {
				goto corrupt
			}
			req = &GetattrRequest{
				Header: m.Header(),
				Flags:  GetattrFlags(in.GetattrFlags),
				Handle: HandleID(in.Fh),
			}
		}

	case opSetattr:
		in := (*setattrIn)(m.data())
		if m.len() < unsafe.Sizeof(*in) {
			goto corrupt
		}
		req = &SetattrRequest{
			Header:   m.Header(),
			Valid:    SetattrValid(in.Valid),
			Handle:   HandleID(in.Fh),
			Size:     in.Size,
			Atime:    time.Unix(int64(in.Atime), int64(in.AtimeNsec)),
			Mtime:    time.Unix(int64(in.Mtime), int64(in.MtimeNsec)),
			Mode:     fileMode(in.Mode),
			Uid:      in.Uid,
			Gid:      in.Gid,
			Bkuptime: in.BkupTime(),
			Chgtime:  in.Chgtime(),
			Flags:    in.Flags(),
		}

	case opReadlink:
		if len(m.bytes()) > 0 {
			goto corrupt
		}
		req = &ReadlinkRequest{
			Header: m.Header(),
		}

	case opSymlink:
		// m.bytes() is "newName\0target\0"
		names := m.bytes()
		if len(names) == 0 || names[len(names)-1] != 0 {
			goto corrupt
		}
		i := bytes.IndexByte(names, '\x00')
		if i < 0 {
			goto corrupt
		}
		newName, target := names[0:i], names[i+1:len(names)-1]
		req = &SymlinkRequest{
			Header:  m.Header(),
			NewName: string(newName),
			Target:  string(target),
		}

	case opLink:
		in := (*linkIn)(m.data())
		if m.len() < unsafe.Sizeof(*in) {
			goto corrupt
		}
		newName := m.bytes()[unsafe.Sizeof(*in):]
		if len(newName) < 2 || newName[len(newName)-1] != 0 {
			goto corrupt
		}
		newName = newName[:len(newName)-1]
		req = &LinkRequest{
			Header:  m.Header(),
			OldNode: NodeID(in.Oldnodeid),
			NewName: string(newName),
		}

	case opMknod:
		size := mknodInSize(c.protocol)
		if m.len() < size {
			goto corrupt
		}
		in := (*mknodIn)(m.data())
		name := m.bytes()[size:]
		if len(name) < 2 || name[len(name)-1] != '\x00' {
			goto corrupt
		}
		name = name[:len(name)-1]
		r := &MknodRequest{
			Header: m.Header(),
			Mode:   fileMode(in.Mode),
			Rdev:   in.Rdev,
			Name:   string(name),
		}
		if c.protocol.GE(Protocol{7, 12}) {
			r.Umask = fileMode(in.Umask) & os.ModePerm
		}
		req = r

	case opMkdir:
		size := mkdirInSize(c.protocol)
		if m.len() < size {
			goto corrupt
		}
		in := (*mkdirIn)(m.data())
		name := m.bytes()[size:]
		i := bytes.IndexByte(name, '\x00')
		if i < 0 {
			goto corrupt
		}
		r := &MkdirRequest{
			Header: m.Header(),
			Name:   string(name[:i]),
			// observed on Linux: mkdirIn.Mode & syscall.S_IFMT == 0,
			// and this causes fileMode to go into it's "no idea"
			// code branch; enforce type to directory
			Mode: fileMode((in.Mode &^ syscall.S_IFMT) | syscall.S_IFDIR),
		}
		if c.protocol.GE(Protocol{7, 12}) {
			r.Umask = fileMode(in.Umask) & os.ModePerm
		}
		req = r

	case opUnlink, opRmdir:
		buf := m.bytes()
		n := len(buf)
		if n == 0 || buf[n-1] != '\x00' {
			goto corrupt
		}
		req = &RemoveRequest{
			Header: m.Header(),
			Name:   string(buf[:n-1]),
			Dir:    m.hdr.Opcode == opRmdir,
		}

	case opRename:
		in := (*renameIn)(m.data())
		if m.len() < unsafe.Sizeof(*in) {
			goto corrupt
		}
		newDirNodeID := NodeID(in.Newdir)
		oldNew := m.bytes()[unsafe.Sizeof(*in):]
		// oldNew should be "old\x00new\x00"
		if len(oldNew) < 4 {
			goto corrupt
		}
		if oldNew[len(oldNew)-1] != '\x00' {
			goto corrupt
		}
		i := bytes.IndexByte(oldNew, '\x00')
		if i < 0 {
			goto corrupt
		}
		oldName, newName := string(oldNew[:i]), string(oldNew[i+1:len(oldNew)-1])
		req = &RenameRequest{
			Header:  m.Header(),
			NewDir:  newDirNodeID,
			OldName: oldName,
			NewName: newName,
		}

	case opOpendir, opOpen:
		in := (*openIn)(m.data())
		if m.len() < unsafe.Sizeof(*in) {
			goto corrupt
		}
		req = &OpenRequest{
			Header: m.Header(),
			Dir:    m.hdr.Opcode == opOpendir,
			Flags:  openFlags(in.Flags),
		}

	case opRead, opReaddir:
		in := (*readIn)(m.data())
		if m.len() < readInSize(c.protocol) {
			goto corrupt
		}
		r := &ReadRequest{
			Header: m.Header(),
			Dir:    m.hdr.Opcode == opReaddir,
			Handle: HandleID(in.Fh),
			Offset: int64(in.Offset),
			Size:   int(in.Size),
		}
		if c.protocol.GE(Protocol{7, 9}) {
			r.Flags = ReadFlags(in.ReadFlags)
			r.LockOwner = in.LockOwner
			r.FileFlags = openFlags(in.Flags)
		}
		req = r

	case opWrite:
		in := (*writeIn)(m.data())
		if m.len() < writeInSize(c.protocol) {
			goto corrupt
		}
		r := &WriteRequest{
			Header: m.Header(),
			Handle: HandleID(in.Fh),
			Offset: int64(in.Offset),
			Flags:  WriteFlags(in.WriteFlags),
		}
		if c.protocol.GE(Protocol{7, 9}) {
			r.LockOwner = in.LockOwner
			r.FileFlags = openFlags(in.Flags)
		}
		buf := m.bytes()[writeInSize(c.protocol):]
		if uint32(len(buf)) < in.Size {
			goto corrupt
		}
		r.Data = buf
		req = r

	case opStatfs:
		req = &StatfsRequest{
			Header: m.Header(),
		}

	case opRelease, opReleasedir:
		in := (*releaseIn)(m.data())
		if m.len() < unsafe.Sizeof(*in) {
			goto corrupt
		}
		req = &ReleaseRequest{
			Header:       m.Header(),
			Dir:          m.hdr.Opcode == opReleasedir,
			Handle:       HandleID(in.Fh),
			Flags:        openFlags(in.Flags),
			ReleaseFlags: ReleaseFlags(in.ReleaseFlags),
			LockOwner:    in.LockOwner,
		}

	case opFsync, opFsyncdir:
		in := (*fsyncIn)(m.data())
		if m.len() < unsafe.Sizeof(*in) {
			goto corrupt
		}
		req = &FsyncRequest{
			Dir:    m.hdr.Opcode == opFsyncdir,
			Header: m.Header(),
			Handle: HandleID(in.Fh),
			Flags:  in.FsyncFlags,
		}

	case opSetxattr:
		in := (*setxattrIn)(m.data())
		if m.len() < unsafe.Sizeof(*in) {
			goto corrupt
		}
		m.off += int(unsafe.Sizeof(*in))
		name := m.bytes()
		i := bytes.IndexByte(name, '\x00')
		if i < 0 {
			goto corrupt
		}
		xattr := name[i+1:]
		if uint32(len(xattr)) < in.Size {
			goto corrupt
		}
		xattr = xattr[:in.Size]
		req = &SetxattrRequest{
			Header:   m.Header(),
			Flags:    in.Flags,
			Position: in.position(),
			Name:     string(name[:i]),
			Xattr:    xattr,
		}

	case opGetxattr:
		in := (*getxattrIn)(m.data())
		if m.len() < unsafe.Sizeof(*in) {
			goto corrupt
		}
		name := m.bytes()[unsafe.Sizeof(*in):]
		i := bytes.IndexByte(name, '\x00')
		if i < 0 {
			goto corrupt
		}
		req = &GetxattrRequest{
			Header:   m.Header(),
			Name:     string(name[:i]),
			Size:     in.Size,
			Position: in.position(),
		}

	case opListxattr:
		in := (*getxattrIn)(m.data())
		if m.len() < unsafe.Sizeof(*in) {
			goto corrupt
		}
		req = &ListxattrRequest{
			Header:   m.Header(),
			Size:     in.Size,
			Position: in.position(),
		}

	case opRemovexattr:
		buf := m.bytes()
		n := len(buf)
		if n == 0 || buf[n-1] != '\x00' {
			goto corrupt
		}
		req = &RemovexattrRequest{
			Header: m.Header(),
			Name:   string(buf[:n-1]),
		}

	case opFlush:
		in := (*flushIn)(m.data())
		if m.len() < unsafe.Sizeof(*in) {
			goto corrupt
		}
		req = &FlushRequest{
			Header:    m.Header(),
			Handle:    HandleID(in.Fh),
			Flags:     in.FlushFlags,
			LockOwner: in.LockOwner,
		}

	case opInit:
		in := (*initIn)(m.data())
		if m.len() < unsafe.Sizeof(*in) {
			goto corrupt
		}
		req = &InitRequest{
			Header:       m.Header(),
			Kernel:       Protocol{in.Major, in.Minor},
			MaxReadahead: in.MaxReadahead,
			Flags:        InitFlags(in.Flags),
		}

	case opGetlk:
		panic("opGetlk")
	case opSetlk:
		panic("opSetlk")
	case opSetlkw:
		panic("opSetlkw")

	case opAccess:
		in := (*accessIn)(m.data())
		if m.len() < unsafe.Sizeof(*in) {
			goto corrupt
		}
		req = &AccessRequest{
			Header: m.Header(),
			Mask:   in.Mask,
		}

	case opCreate:
		size := createInSize(c.protocol)
		if m.len() < size {
			goto corrupt
		}
		in := (*createIn)(m.data())
		name := m.bytes()[size:]
		i := bytes.IndexByte(name, '\x00')
		if i < 0 {
			goto corrupt
		}
		r := &CreateRequest{
			Header: m.Header(),
			Flags:  openFlags(in.Flags),
			Mode:   fileMode(in.Mode),
			Name:   string(name[:i]),
		}
		if c.protocol.GE(Protocol{7, 12}) {
			r.Umask = fileMode(in.Umask) & os.ModePerm
		}
		req = r

	case opInterrupt:
		in := (*interruptIn)(m.data())
		if m.len() < unsafe.Sizeof(*in) {
			goto corrupt
		}
		req = &InterruptRequest{
			Header: m.Header(),
			IntrID: RequestID(in.Unique),
		}

	case opBmap:
		panic("opBmap")

	case opDestroy:
		req = &DestroyRequest{
			Header: m.Header(),
		}

		// OS X
	case opSetvolname:
		panic("opSetvolname")
	case opGetxtimes:
		panic("opGetxtimes")
	case opExchange:
		in := (*exchangeIn)(m.data())
		if m.len() < unsafe.Sizeof(*in) {
			goto corrupt
		}
		oldDirNodeID := NodeID(in.Olddir)
		newDirNodeID := NodeID(in.Newdir)
		oldNew := m.bytes()[unsafe.Sizeof(*in):]
		// oldNew should be "oldname\x00newname\x00"
		if len(oldNew) < 4 {
			goto corrupt
		}
		if oldNew[len(oldNew)-1] != '\x00' {
			goto corrupt
		}
		i := bytes.IndexByte(oldNew, '\x00')
		if i < 0 {
			goto corrupt
		}
		oldName, newName := string(oldNew[:i]), string(oldNew[i+1:len(oldNew)-1])
		req = &ExchangeDataRequest{
			Header:  m.Header(),
			OldDir:  oldDirNodeID,
			NewDir:  newDirNodeID,
			OldName: oldName,
			NewName: newName,
			// TODO options
		}
	}

	return req, nil

corrupt:
	Debug(malformedMessage{})
	putMessage(m)
	return nil, fmt.Errorf("fuse: malformed message")

unrecognized:
	// Unrecognized message.
	// Assume higher-level code will send a "no idea what you mean" error.
	h := m.Header()
	return &h, nil
}

func (c *Conn) writeToKernel(msg []byte) error {
	out := (*outHeader)(unsafe.Pointer(&msg[0]))
	out.Len = uint32(len(msg))

	c.wio.RLock()
	defer c.wio.RUnlock()
	nn, err := syscall.Write(c.fd(), msg)
	if err == nil && nn != len(msg) {
		Debug(bugShortKernelWrite{
			Written: int64(nn),
			Length:  int64(len(msg)),
			Error:   errorString(err),
			Stack:   stack(),
		})
	}
	return err
}

func (c *Conn) respond(msg []byte) {
	if err := c.writeToKernel(msg); err != nil {
		Debug(bugKernelWriteError{
			Error: errorString(err),
			Stack: stack(),
		})
	}
}

// sendInvalidate sends an invalidate notification to kernel.
//
// A returned ENOENT is translated to a friendlier error.
func (c *Conn) sendInvalidate(msg []byte) error {
	switch err := c.writeToKernel(msg); err {
	case syscall.ENOENT:
		return ErrNotCached
	default:
		return err
	}
}

// InvalidateNode invalidates the kernel cache of the attributes and a
// range of the data of a node.
//
// Giving offset 0 and size -1 means all data. To invalidate just the
// attributes, give offset 0 and size 0.
//
// Returns ErrNotCached if the kernel is not currently caching the
// node.
func (c *Conn) InvalidateNode(nodeID NodeID, off int64, size int64) error {
	buf := newBuffer(unsafe.Sizeof(notifyInvalInodeOut{}))
	h := (*outHeader)(unsafe.Pointer(&buf[0]))
	// h.Unique is 0
	h.Error = notifyCodeInvalInode
	out := (*notifyInvalInodeOut)(buf.alloc(unsafe.Sizeof(notifyInvalInodeOut{})))
	out.Ino = uint64(nodeID)
	out.Off = off
	out.Len = size
	return c.sendInvalidate(buf)
}

// InvalidateEntry invalidates the kernel cache of the directory entry
// identified by parent directory node ID and entry basename.
//
// Kernel may or may not cache directory listings. To invalidate
// those, use InvalidateNode to invalidate all of the data for a
// directory. (As of 2015-06, Linux FUSE does not cache directory
// listings.)
//
// Returns ErrNotCached if the kernel is not currently caching the
// node.
func (c *Conn) InvalidateEntry(parent NodeID, name string) error {
	const maxUint32 = ^uint32(0)
	if uint64(len(name)) > uint64(maxUint32) {
		// very unlikely, but we don't want to silently truncate
		return syscall.ENAMETOOLONG
	}
	buf := newBuffer(unsafe.Sizeof(notifyInvalEntryOut{}) + uintptr(len(name)) + 1)
	h := (*outHeader)(unsafe.Pointer(&buf[0]))
	// h.Unique is 0
	h.Error = notifyCodeInvalEntry
	out := (*notifyInvalEntryOut)(buf.alloc(unsafe.Sizeof(notifyInvalEntryOut{})))
	out.Parent = uint64(parent)
	out.Namelen = uint32(len(name))
	buf = append(buf, name...)
	buf = append(buf, '\x00')
	return c.sendInvalidate(buf)
}
