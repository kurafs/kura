// See the file LICENSE for copyright and licensing information.
package fuse

import (
	"fmt"
	"io"
	"os"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

// Mount mounts a new FUSE connection on the named directory
// and returns a connection for reading and writing FUSE messages.
//
// After a successful return, caller must call Close to free
// resources.
//
// Even on successful return, the new mount is not guaranteed to be
// visible until after Conn.Ready is closed. See Conn.MountError for
// possible errors. Incoming requests on Conn must be served to make
// progress.
// TODO: Comment. Blocks until mounted.
func Mount(dir string, options ...MountOption) (*Conn, error) {
	conf := mountConfig{
		options: make(map[string]string),
	}
	for _, option := range options {
		if err := option(&conf); err != nil {
			return nil, err
		}
	}

	dev, err := mount(dir, &conf)
	if err != nil {
		return nil, err
	}
	conn := &Conn{dev: dev}

	if err := initialize(conn, &conf); err != nil {
		conn.Close()
		return nil, err
	}

	return conn, nil
}

func initialize(c *Conn, conf *mountConfig) error {
	req, err := c.ReadRequest()
	if err != nil {
		if err == io.EOF {
			return ErrClosedWithoutInit
		}
		return err
	}
	r, ok := req.(*InitRequest)
	if !ok {
		return fmt.Errorf("missing init, got: %T", req)
	}

	min := Protocol{protoVersionMinMajor, protoVersionMinMinor}
	if r.Kernel.LT(min) {
		req.RespondError(Errno(syscall.EPROTO))
		c.Close()
		return &OldVersionError{
			Kernel:     r.Kernel,
			LibraryMin: min,
		}
	}

	protocol := Protocol{protoVersionMaxMajor, protoVersionMaxMinor}
	if r.Kernel.LT(protocol) {
		// Kernel doesn't support the latest version we have.
		protocol = r.Kernel
	}
	c.protocol = protocol

	s := &InitResponse{
		Library:      protocol,
		MaxReadahead: conf.maxReadahead,
		MaxWrite:     maxWrite,
		Flags:        InitBigWrites | conf.initFlags,
	}
	r.Respond(s)
	return nil
}

// A Request represents a single FUSE request received from the kernel.
// Use a type switch to determine the specific kind.
// A request of unrecognized type will have concrete type *Header.
type Request interface {
	// Hdr returns the Header associated with this request.
	Hdr() *Header

	// RespondError responds to the request with the given error.
	RespondError(error)

	String() string
}

// A RequestID identifies an active FUSE request.
type RequestID uint64

func (r RequestID) String() string {
	return fmt.Sprintf("%#x", uint64(r))
}

// A NodeID is a number identifying a directory or file.
// It must be unique among IDs returned in LookupResponses
// that have not yet been forgotten by ForgetRequests.
type NodeID uint64

func (n NodeID) String() string {
	return fmt.Sprintf("%#x", uint64(n))
}

// A HandleID is a number identifying an open directory or file.
// It only needs to be unique while the directory or file is open.
type HandleID uint64

func (h HandleID) String() string {
	return fmt.Sprintf("%#x", uint64(h))
}

// The RootID identifies the root directory of a FUSE file system.
const RootID NodeID = rootID

// A Header describes the basic information sent in every request.
type Header struct {
	Conn *Conn     `json:"-"` // Connection this request was received on.
	ID   RequestID // Unique ID for request.
	Node NodeID    // File or directory the request is about.
	Uid  uint32    // User ID of process making request.
	Gid  uint32    // Group ID of process making request.
	Pid  uint32    // Process ID of process making request.

	// for returning to reqPool
	msg *message
}

func (h *Header) String() string {
	return fmt.Sprintf("ID=%v Node=%v Uid=%d Gid=%d Pid=%d", h.ID, h.Node, h.Uid, h.Gid, h.Pid)
}

func (h *Header) Hdr() *Header {
	return h
}

func (h *Header) noResponse() {
	putMessage(h.msg)
}

func (h *Header) respond(msg []byte) {
	out := (*outHeader)(unsafe.Pointer(&msg[0]))
	out.Unique = uint64(h.ID)
	h.Conn.respond(msg)
	putMessage(h.msg)
}

// An ErrorNumber is an error with a specific error number.
//
// Operations may return an error value that implements ErrorNumber to
// control what specific error number (errno) to return.
type ErrorNumber interface {
	// Errno returns the the error number (errno) for this error.
	Errno() Errno
}

const (
	// ENOSYS indicates that the call is not supported.
	ENOSYS = Errno(syscall.ENOSYS)

	// ESTALE is used by Serve to respond to violations of the FUSE protocol.
	ESTALE = Errno(syscall.ESTALE)

	// EINTR indicates request was interrupted by an InterruptRequest.
	// See also fs.Intr.
	EINTR = Errno(syscall.EINTR)

	EEXIST  = Errno(syscall.EEXIST)
	ENOTSUP = Errno(syscall.ENOTSUP)
	ERANGE  = Errno(syscall.ERANGE)
	EIO     = Errno(syscall.EIO)
	ENOENT  = Errno(syscall.ENOENT)
	EPERM   = Errno(syscall.EPERM)
)

// DefaultErrno is the errno used when error returned does not
// implement ErrorNumber.
const DefaultErrno = EIO

var errnoNames = map[Errno]string{
	ENOSYS: "ENOSYS",
	ESTALE: "ESTALE",
	ENOENT: "ENOENT",
	EIO:    "EIO",
	EPERM:  "EPERM",
	EINTR:  "EINTR",
	EEXIST: "EEXIST",
}

// Errno implements Error and ErrorNumber using a syscall.Errno.
type Errno syscall.Errno

var _ = ErrorNumber(Errno(0))
var _ = error(Errno(0))

func (e Errno) Errno() Errno {
	return e
}

func (e Errno) String() string {
	return syscall.Errno(e).Error()
}

func (e Errno) Error() string {
	return syscall.Errno(e).Error()
}

// ErrnoName returns the short non-numeric identifier for this errno.
// For example, "EIO".
func (e Errno) ErrnoName() string {
	s := errnoNames[e]
	if s == "" {
		s = fmt.Sprint(e.Errno())
	}
	return s
}

func (e Errno) MarshalText() ([]byte, error) {
	s := e.ErrnoName()
	return []byte(s), nil
}

func (h *Header) RespondError(err error) {
	errno := DefaultErrno
	if ferr, ok := err.(ErrorNumber); ok {
		errno = ferr.Errno()
	}
	// FUSE uses negative errors!
	// TODO: File bug report against OSXFUSE: positive error causes kernel panic.
	buf := newBuffer(0)
	hOut := (*outHeader)(unsafe.Pointer(&buf[0]))
	hOut.Error = -int32(errno)
	h.respond(buf)
}

// All requests read from the kernel, without data, are shorter than
// this.
var maxRequestSize = syscall.Getpagesize()
var bufSize = maxRequestSize + maxWrite

// a message represents the bytes of a single FUSE message
type message struct {
	conn *Conn
	buf  []byte    // all bytes
	hdr  *inHeader // header
	off  int       // offset for reading additional fields
}

func (m *message) len() uintptr {
	return uintptr(len(m.buf) - m.off)
}

func (m *message) data() unsafe.Pointer {
	var p unsafe.Pointer
	if m.off < len(m.buf) {
		p = unsafe.Pointer(&m.buf[m.off])
	}
	return p
}

func (m *message) bytes() []byte {
	return m.buf[m.off:]
}

func (m *message) Header() Header {
	h := m.hdr
	return Header{
		Conn: m.conn,
		ID:   RequestID(h.Unique),
		Node: NodeID(h.Nodeid),
		Uid:  h.Uid,
		Gid:  h.Gid,
		Pid:  h.Pid,

		msg: m,
	}
}

// reqPool is a pool of messages.
//
// Lifetime of a logical message is from getMessage to putMessage.
// getMessage is called by ReadRequest. putMessage is called by
// Conn.ReadRequest, Request.Respond, or Request.RespondError.
//
// Messages in the pool are guaranteed to have conn and off zeroed,
// buf allocated and len==bufSize, and hdr set.
var reqPool = sync.Pool{
	New: allocMessage,
}

func allocMessage() interface{} {
	m := &message{buf: make([]byte, bufSize)}
	m.hdr = (*inHeader)(unsafe.Pointer(&m.buf[0]))
	return m
}

func getMessage(c *Conn) *message {
	m := reqPool.Get().(*message)
	m.conn = c
	return m
}

func putMessage(m *message) {
	m.buf = m.buf[:bufSize]
	m.conn = nil
	m.off = 0
	reqPool.Put(m)
}

// fileMode returns a Go os.FileMode from a Unix mode.
func fileMode(unixMode uint32) os.FileMode {
	mode := os.FileMode(unixMode & 0777)
	switch unixMode & syscall.S_IFMT {
	case syscall.S_IFREG:
		// nothing
	case syscall.S_IFDIR:
		mode |= os.ModeDir
	case syscall.S_IFCHR:
		mode |= os.ModeCharDevice | os.ModeDevice
	case syscall.S_IFBLK:
		mode |= os.ModeDevice
	case syscall.S_IFIFO:
		mode |= os.ModeNamedPipe
	case syscall.S_IFLNK:
		mode |= os.ModeSymlink
	case syscall.S_IFSOCK:
		mode |= os.ModeSocket
	default:
		// no idea
		mode |= os.ModeDevice
	}
	if unixMode&syscall.S_ISUID != 0 {
		mode |= os.ModeSetuid
	}
	if unixMode&syscall.S_ISGID != 0 {
		mode |= os.ModeSetgid
	}
	return mode
}

type noOpcode struct {
	Opcode uint32
}

func (m noOpcode) String() string {
	return fmt.Sprintf("No opcode %v", m.Opcode)
}

type malformedMessage struct {
}

func (malformedMessage) String() string {
	return "malformed message"
}

type bugShortKernelWrite struct {
	Written int64
	Length  int64
	Error   string
	Stack   string
}

func (b bugShortKernelWrite) String() string {
	return fmt.Sprintf("short kernel write: written=%d/%d error=%q stack=\n%s", b.Written, b.Length, b.Error, b.Stack)
}

// An Attr is the metadata for a single file or directory.
type Attr struct {
	Valid time.Duration // how long Attr can be cached

	Inode     uint64      // inode number
	Size      uint64      // size in bytes
	Blocks    uint64      // size in 512-byte units
	Atime     time.Time   // time of last access
	Mtime     time.Time   // time of last modification
	Ctime     time.Time   // time of last inode change
	Crtime    time.Time   // time of creation (OS X only)
	Mode      os.FileMode // file mode
	Nlink     uint32      // number of links (usually 1)
	Uid       uint32      // owner uid
	Gid       uint32      // group gid
	Rdev      uint32      // device numbers
	Flags     uint32      // chflags(2) flags (OS X only)
	BlockSize uint32      // preferred blocksize for filesystem I/O
}

func (a Attr) String() string {
	return fmt.Sprintf("valid=%v ino=%v size=%d mode=%v", a.Valid, a.Inode, a.Size, a.Mode)
}

func unix(t time.Time) (sec uint64, nsec uint32) {
	nano := t.UnixNano()
	return uint64(nano / 1e9), uint32(nano % 1e9)
}

func (a *Attr) attr(out *attr, proto Protocol) {
	out.Ino = a.Inode
	out.Size = a.Size
	out.Blocks = a.Blocks
	out.Atime, out.AtimeNsec = unix(a.Atime)
	out.Mtime, out.MtimeNsec = unix(a.Mtime)
	out.Ctime, out.CtimeNsec = unix(a.Ctime)
	out.SetCrtime(unix(a.Crtime))
	out.Mode = uint32(a.Mode) & 0777
	switch {
	default:
		out.Mode |= syscall.S_IFREG
	case a.Mode&os.ModeDir != 0:
		out.Mode |= syscall.S_IFDIR
	case a.Mode&os.ModeDevice != 0:
		if a.Mode&os.ModeCharDevice != 0 {
			out.Mode |= syscall.S_IFCHR
		} else {
			out.Mode |= syscall.S_IFBLK
		}
	case a.Mode&os.ModeNamedPipe != 0:
		out.Mode |= syscall.S_IFIFO
	case a.Mode&os.ModeSymlink != 0:
		out.Mode |= syscall.S_IFLNK
	case a.Mode&os.ModeSocket != 0:
		out.Mode |= syscall.S_IFSOCK
	}
	if a.Mode&os.ModeSetuid != 0 {
		out.Mode |= syscall.S_ISUID
	}
	if a.Mode&os.ModeSetgid != 0 {
		out.Mode |= syscall.S_ISGID
	}
	out.Nlink = a.Nlink
	out.Uid = a.Uid
	out.Gid = a.Gid
	out.Rdev = a.Rdev
	out.SetFlags(a.Flags)
	if proto.GE(Protocol{7, 9}) {
		out.Blksize = a.BlockSize
	}

	return
}

func trunc(b []byte, max int) ([]byte, string) {
	if len(b) > max {
		return b[:max], "..."
	}
	return b, ""
}

// A Dirent represents a single directory entry.
type Dirent struct {
	// Inode this entry names.
	Inode uint64

	// Type of the entry, for example DT_File.
	//
	// Setting this is optional. The zero value (DT_Unknown) means
	// callers will just need to do a Getattr when the type is
	// needed. Providing a type can speed up operations
	// significantly.
	Type DirentType

	// Name of the entry
	Name string
}

// Type of an entry in a directory listing.
type DirentType uint32

const (
	// These don't quite match os.FileMode; especially there's an
	// explicit unknown, instead of zero value meaning file. They
	// are also not quite syscall.DT_*; nothing says the FUSE
	// protocol follows those, and even if they were, we don't
	// want each fs to fiddle with syscall.

	// The shift by 12 is hardcoded in the FUSE userspace
	// low-level C library, so it's safe here.

	DT_Unknown DirentType = 0
	DT_Socket  DirentType = syscall.S_IFSOCK >> 12
	DT_Link    DirentType = syscall.S_IFLNK >> 12
	DT_File    DirentType = syscall.S_IFREG >> 12
	DT_Block   DirentType = syscall.S_IFBLK >> 12
	DT_Dir     DirentType = syscall.S_IFDIR >> 12
	DT_Char    DirentType = syscall.S_IFCHR >> 12
	DT_FIFO    DirentType = syscall.S_IFIFO >> 12
)

func (t DirentType) String() string {
	switch t {
	case DT_Unknown:
		return "unknown"
	case DT_Socket:
		return "socket"
	case DT_Link:
		return "link"
	case DT_File:
		return "file"
	case DT_Block:
		return "block"
	case DT_Dir:
		return "dir"
	case DT_Char:
		return "char"
	case DT_FIFO:
		return "fifo"
	}
	return "invalid"
}

// AppendDirent appends the encoded form of a directory entry to data
// and returns the resulting slice.
func AppendDirent(data []byte, dir Dirent) []byte {
	de := dirent{
		Ino:     dir.Inode,
		Namelen: uint32(len(dir.Name)),
		Type:    uint32(dir.Type),
	}
	de.Off = uint64(len(data) + direntSize + (len(dir.Name)+7)&^7)
	data = append(data, (*[direntSize]byte)(unsafe.Pointer(&de))[:]...)
	data = append(data, dir.Name...)
	n := direntSize + uintptr(len(dir.Name))
	if n%8 != 0 {
		var pad [8]byte
		data = append(data, pad[:8-n%8]...)
	}
	return data
}
