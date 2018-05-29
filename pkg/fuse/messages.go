package fuse

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"time"
	"unsafe"
)

// An InitRequest is the first request sent on a FUSE file system.
type InitRequest struct {
	Header `json:"-"`
	Kernel Protocol
	// Maximum readahead in bytes that the kernel plans to use.
	MaxReadahead uint32
	Flags        InitFlags
}

var _ = Request(&InitRequest{})

func (r *InitRequest) String() string {
	return fmt.Sprintf("Init [%v] %v ra=%d fl=%v", &r.Header, r.Kernel, r.MaxReadahead, r.Flags)
}

// An InitResponse is the response to an InitRequest.
type InitResponse struct {
	Library Protocol
	// Maximum readahead in bytes that the kernel can use. Ignored if
	// greater than InitRequest.MaxReadahead.
	MaxReadahead uint32
	Flags        InitFlags
	// Maximum size of a single write operation.
	// Linux enforces a minimum of 4 KiB.
	MaxWrite uint32
}

func (r *InitResponse) String() string {
	return fmt.Sprintf("Init %v ra=%d fl=%v w=%d", r.Library, r.MaxReadahead, r.Flags, r.MaxWrite)
}

// Respond replies to the request with the given response.
func (r *InitRequest) Respond(resp *InitResponse) {
	buf := newBuffer(unsafe.Sizeof(initOut{}))
	out := (*initOut)(buf.alloc(unsafe.Sizeof(initOut{})))
	out.Major = resp.Library.Major
	out.Minor = resp.Library.Minor
	out.MaxReadahead = resp.MaxReadahead
	out.Flags = uint32(resp.Flags)
	out.MaxWrite = resp.MaxWrite

	// MaxWrite larger than our receive buffer would just lead to
	// errors on large writes.
	if out.MaxWrite > maxWrite {
		out.MaxWrite = maxWrite
	}
	r.respond(buf)
}

// A StatfsRequest requests information about the mounted file system.
type StatfsRequest struct {
	Header `json:"-"`
}

var _ = Request(&StatfsRequest{})

func (r *StatfsRequest) String() string {
	return fmt.Sprintf("Statfs [%s]", &r.Header)
}

// Respond replies to the request with the given response.
func (r *StatfsRequest) Respond(resp *StatfsResponse) {
	buf := newBuffer(unsafe.Sizeof(statfsOut{}))
	out := (*statfsOut)(buf.alloc(unsafe.Sizeof(statfsOut{})))
	out.St = kstatfs{
		Blocks:  resp.Blocks,
		Bfree:   resp.Bfree,
		Bavail:  resp.Bavail,
		Files:   resp.Files,
		Ffree:   resp.Ffree,
		Bsize:   resp.Bsize,
		Namelen: resp.Namelen,
		Frsize:  resp.Frsize,
	}
	r.respond(buf)
}

// A StatfsResponse is the response to a StatfsRequest.
type StatfsResponse struct {
	Blocks  uint64 // Total data blocks in file system.
	Bfree   uint64 // Free blocks in file system.
	Bavail  uint64 // Free blocks in file system if you're not root.
	Files   uint64 // Total files in file system.
	Ffree   uint64 // Free files in file system.
	Bsize   uint32 // Block size
	Namelen uint32 // Maximum file name length?
	Frsize  uint32 // Fragment size, smallest addressable data size in the file system.
}

func (r *StatfsResponse) String() string {
	return fmt.Sprintf("Statfs blocks=%d/%d/%d files=%d/%d bsize=%d frsize=%d namelen=%d",
		r.Bavail, r.Bfree, r.Blocks,
		r.Ffree, r.Files,
		r.Bsize,
		r.Frsize,
		r.Namelen,
	)
}

// An AccessRequest asks whether the file can be accessed
// for the purpose specified by the mask.
type AccessRequest struct {
	Header `json:"-"`
	Mask   uint32
}

var _ = Request(&AccessRequest{})

func (r *AccessRequest) String() string {
	return fmt.Sprintf("Access [%s] mask=%#x", &r.Header, r.Mask)
}

// Respond replies to the request indicating that access is allowed.
// To deny access, use RespondError.
func (r *AccessRequest) Respond() {
	buf := newBuffer(0)
	r.respond(buf)
}

// A GetattrRequest asks for the metadata for the file denoted by r.Node.
type GetattrRequest struct {
	Header `json:"-"`
	Flags  GetattrFlags
	Handle HandleID
}

var _ = Request(&GetattrRequest{})

func (r *GetattrRequest) String() string {
	return fmt.Sprintf("Getattr [%s] %v fl=%v", &r.Header, r.Handle, r.Flags)
}

// Respond replies to the request with the given response.
func (r *GetattrRequest) Respond(resp *GetattrResponse) {
	size := attrOutSize(r.Header.Conn.protocol)
	buf := newBuffer(size)
	out := (*attrOut)(buf.alloc(size))
	out.AttrValid = uint64(resp.Attr.Valid / time.Second)
	out.AttrValidNsec = uint32(resp.Attr.Valid % time.Second / time.Nanosecond)
	resp.Attr.attr(&out.Attr, r.Header.Conn.protocol)
	r.respond(buf)
}

// A GetattrResponse is the response to a GetattrRequest.
type GetattrResponse struct {
	Attr Attr // file attributes
}

func (r *GetattrResponse) String() string {
	return fmt.Sprintf("Getattr %v", r.Attr)
}

// A GetxattrRequest asks for the extended attributes associated with r.Node.
type GetxattrRequest struct {
	Header `json:"-"`

	// Maximum size to return.
	Size uint32

	// Name of the attribute requested.
	Name string

	// Offset within extended attributes.
	//
	// Only valid for OS X, and then only with the resource fork
	// attribute.
	Position uint32
}

var _ = Request(&GetxattrRequest{})

func (r *GetxattrRequest) String() string {
	return fmt.Sprintf("Getxattr [%s] %q %d @%d", &r.Header, r.Name, r.Size, r.Position)
}

// Respond replies to the request with the given response.
func (r *GetxattrRequest) Respond(resp *GetxattrResponse) {
	if r.Size == 0 {
		buf := newBuffer(unsafe.Sizeof(getxattrOut{}))
		out := (*getxattrOut)(buf.alloc(unsafe.Sizeof(getxattrOut{})))
		out.Size = uint32(len(resp.Xattr))
		r.respond(buf)
	} else {
		buf := newBuffer(uintptr(len(resp.Xattr)))
		buf = append(buf, resp.Xattr...)
		r.respond(buf)
	}
}

// A GetxattrResponse is the response to a GetxattrRequest.
type GetxattrResponse struct {
	Xattr []byte
}

func (r *GetxattrResponse) String() string {
	return fmt.Sprintf("Getxattr %x", r.Xattr)
}

// A ListxattrRequest asks to list the extended attributes associated with r.Node.
type ListxattrRequest struct {
	Header   `json:"-"`
	Size     uint32 // maximum size to return
	Position uint32 // offset within attribute list
}

var _ = Request(&ListxattrRequest{})

func (r *ListxattrRequest) String() string {
	return fmt.Sprintf("Listxattr [%s] %d @%d", &r.Header, r.Size, r.Position)
}

// Respond replies to the request with the given response.
func (r *ListxattrRequest) Respond(resp *ListxattrResponse) {
	if r.Size == 0 {
		buf := newBuffer(unsafe.Sizeof(getxattrOut{}))
		out := (*getxattrOut)(buf.alloc(unsafe.Sizeof(getxattrOut{})))
		out.Size = uint32(len(resp.Xattr))
		r.respond(buf)
	} else {
		buf := newBuffer(uintptr(len(resp.Xattr)))
		buf = append(buf, resp.Xattr...)
		r.respond(buf)
	}
}

// A ListxattrResponse is the response to a ListxattrRequest.
type ListxattrResponse struct {
	Xattr []byte
}

func (r *ListxattrResponse) String() string {
	return fmt.Sprintf("Listxattr %x", r.Xattr)
}

// Append adds an extended attribute name to the response.
func (r *ListxattrResponse) Append(names ...string) {
	for _, name := range names {
		r.Xattr = append(r.Xattr, name...)
		r.Xattr = append(r.Xattr, '\x00')
	}
}

// A RemovexattrRequest asks to remove an extended attribute associated with r.Node.
type RemovexattrRequest struct {
	Header `json:"-"`
	Name   string // name of extended attribute
}

var _ = Request(&RemovexattrRequest{})

func (r *RemovexattrRequest) String() string {
	return fmt.Sprintf("Removexattr [%s] %q", &r.Header, r.Name)
}

// Respond replies to the request, indicating that the attribute was removed.
func (r *RemovexattrRequest) Respond() {
	buf := newBuffer(0)
	r.respond(buf)
}

// A SetxattrRequest asks to set an extended attribute associated with a file.
type SetxattrRequest struct {
	Header `json:"-"`

	// Flags can make the request fail if attribute does/not already
	// exist. Unfortunately, the constants are platform-specific and
	// not exposed by Go1.2. Look for XATTR_CREATE, XATTR_REPLACE.
	//
	// TODO improve this later
	//
	// TODO XATTR_CREATE and exist -> EEXIST
	//
	// TODO XATTR_REPLACE and not exist -> ENODATA
	Flags uint32

	// Offset within extended attributes.
	//
	// Only valid for OS X, and then only with the resource fork
	// attribute.
	Position uint32

	Name  string
	Xattr []byte
}

var _ = Request(&SetxattrRequest{})

func (r *SetxattrRequest) String() string {
	xattr, tail := trunc(r.Xattr, 16)
	return fmt.Sprintf("Setxattr [%s] %q %x%s fl=%v @%#x", &r.Header, r.Name, xattr, tail, r.Flags, r.Position)
}

// Respond replies to the request, indicating that the extended attribute was set.
func (r *SetxattrRequest) Respond() {
	buf := newBuffer(0)
	r.respond(buf)
}

// A LookupRequest asks to look up the given name in the directory named by r.Node.
type LookupRequest struct {
	Header `json:"-"`
	Name   string
}

var _ = Request(&LookupRequest{})

func (r *LookupRequest) String() string {
	return fmt.Sprintf("Lookup [%s] %q", &r.Header, r.Name)
}

// Respond replies to the request with the given response.
func (r *LookupRequest) Respond(resp *LookupResponse) {
	size := entryOutSize(r.Header.Conn.protocol)
	buf := newBuffer(size)
	out := (*entryOut)(buf.alloc(size))
	out.Nodeid = uint64(resp.Node)
	out.Generation = resp.Generation
	out.EntryValid = uint64(resp.EntryValid / time.Second)
	out.EntryValidNsec = uint32(resp.EntryValid % time.Second / time.Nanosecond)
	out.AttrValid = uint64(resp.Attr.Valid / time.Second)
	out.AttrValidNsec = uint32(resp.Attr.Valid % time.Second / time.Nanosecond)
	resp.Attr.attr(&out.Attr, r.Header.Conn.protocol)
	r.respond(buf)
}

// A LookupResponse is the response to a LookupRequest.
type LookupResponse struct {
	Node       NodeID
	Generation uint64
	EntryValid time.Duration
	Attr       Attr
}

func (r *LookupResponse) string() string {
	return fmt.Sprintf("%v gen=%d valid=%v attr={%v}", r.Node, r.Generation, r.EntryValid, r.Attr)
}

func (r *LookupResponse) String() string {
	return fmt.Sprintf("Lookup %s", r.string())
}

// An OpenRequest asks to open a file or directory
type OpenRequest struct {
	Header `json:"-"`
	Dir    bool // is this Opendir?
	Flags  OpenFlags
}

var _ = Request(&OpenRequest{})

func (r *OpenRequest) String() string {
	return fmt.Sprintf("Open [%s] dir=%v fl=%v", &r.Header, r.Dir, r.Flags)
}

// Respond replies to the request with the given response.
func (r *OpenRequest) Respond(resp *OpenResponse) {
	buf := newBuffer(unsafe.Sizeof(openOut{}))
	out := (*openOut)(buf.alloc(unsafe.Sizeof(openOut{})))
	out.Fh = uint64(resp.Handle)
	out.OpenFlags = uint32(resp.Flags)
	r.respond(buf)
}

// A OpenResponse is the response to a OpenRequest.
type OpenResponse struct {
	Handle HandleID
	Flags  OpenResponseFlags
}

func (r *OpenResponse) string() string {
	return fmt.Sprintf("%v fl=%v", r.Handle, r.Flags)
}

func (r *OpenResponse) String() string {
	return fmt.Sprintf("Open %s", r.string())
}

// A CreateRequest asks to create and open a file (not a directory).
type CreateRequest struct {
	Header `json:"-"`
	Name   string
	Flags  OpenFlags
	Mode   os.FileMode
	// Umask of the request. Not supported on OS X.
	Umask os.FileMode
}

var _ = Request(&CreateRequest{})

func (r *CreateRequest) String() string {
	return fmt.Sprintf("Create [%s] %q fl=%v mode=%v umask=%v", &r.Header, r.Name, r.Flags, r.Mode, r.Umask)
}

// Respond replies to the request with the given response.
func (r *CreateRequest) Respond(resp *CreateResponse) {
	eSize := entryOutSize(r.Header.Conn.protocol)
	buf := newBuffer(eSize + unsafe.Sizeof(openOut{}))

	e := (*entryOut)(buf.alloc(eSize))
	e.Nodeid = uint64(resp.Node)
	e.Generation = resp.Generation
	e.EntryValid = uint64(resp.EntryValid / time.Second)
	e.EntryValidNsec = uint32(resp.EntryValid % time.Second / time.Nanosecond)
	e.AttrValid = uint64(resp.Attr.Valid / time.Second)
	e.AttrValidNsec = uint32(resp.Attr.Valid % time.Second / time.Nanosecond)
	resp.Attr.attr(&e.Attr, r.Header.Conn.protocol)

	o := (*openOut)(buf.alloc(unsafe.Sizeof(openOut{})))
	o.Fh = uint64(resp.Handle)
	o.OpenFlags = uint32(resp.Flags)

	r.respond(buf)
}

// A CreateResponse is the response to a CreateRequest.
// It describes the created node and opened handle.
type CreateResponse struct {
	LookupResponse
	OpenResponse
}

func (r *CreateResponse) String() string {
	return fmt.Sprintf("Create {%s} {%s}", r.LookupResponse.string(), r.OpenResponse.string())
}

// A MkdirRequest asks to create (but not open) a directory.
type MkdirRequest struct {
	Header `json:"-"`
	Name   string
	Mode   os.FileMode
	// Umask of the request. Not supported on OS X.
	Umask os.FileMode
}

var _ = Request(&MkdirRequest{})

func (r *MkdirRequest) String() string {
	return fmt.Sprintf("Mkdir [%s] %q mode=%v umask=%v", &r.Header, r.Name, r.Mode, r.Umask)
}

// Respond replies to the request with the given response.
func (r *MkdirRequest) Respond(resp *MkdirResponse) {
	size := entryOutSize(r.Header.Conn.protocol)
	buf := newBuffer(size)
	out := (*entryOut)(buf.alloc(size))
	out.Nodeid = uint64(resp.Node)
	out.Generation = resp.Generation
	out.EntryValid = uint64(resp.EntryValid / time.Second)
	out.EntryValidNsec = uint32(resp.EntryValid % time.Second / time.Nanosecond)
	out.AttrValid = uint64(resp.Attr.Valid / time.Second)
	out.AttrValidNsec = uint32(resp.Attr.Valid % time.Second / time.Nanosecond)
	resp.Attr.attr(&out.Attr, r.Header.Conn.protocol)
	r.respond(buf)
}

// A MkdirResponse is the response to a MkdirRequest.
type MkdirResponse struct {
	LookupResponse
}

func (r *MkdirResponse) String() string {
	return fmt.Sprintf("Mkdir %v", r.LookupResponse.string())
}

// A ReadRequest asks to read from an open file.
type ReadRequest struct {
	Header    `json:"-"`
	Dir       bool // is this Readdir?
	Handle    HandleID
	Offset    int64
	Size      int
	Flags     ReadFlags
	LockOwner uint64
	FileFlags OpenFlags
}

var _ = Request(&ReadRequest{})

func (r *ReadRequest) String() string {
	return fmt.Sprintf("Read [%s] %v %d @%#x dir=%v fl=%v lock=%d ffl=%v", &r.Header, r.Handle, r.Size, r.Offset, r.Dir, r.Flags, r.LockOwner, r.FileFlags)
}

// Respond replies to the request with the given response.
func (r *ReadRequest) Respond(resp *ReadResponse) {
	buf := newBuffer(uintptr(len(resp.Data)))
	buf = append(buf, resp.Data...)
	r.respond(buf)
}

// A ReadResponse is the response to a ReadRequest.
type ReadResponse struct {
	Data []byte
}

func (r *ReadResponse) String() string {
	return fmt.Sprintf("Read %d", len(r.Data))
}

type jsonReadResponse struct {
	Len uint64
}

func (r *ReadResponse) MarshalJSON() ([]byte, error) {
	j := jsonReadResponse{
		Len: uint64(len(r.Data)),
	}
	return json.Marshal(j)
}

// A ReleaseRequest asks to release (close) an open file handle.
type ReleaseRequest struct {
	Header       `json:"-"`
	Dir          bool // is this Releasedir?
	Handle       HandleID
	Flags        OpenFlags // flags from OpenRequest
	ReleaseFlags ReleaseFlags
	LockOwner    uint32
}

var _ = Request(&ReleaseRequest{})

func (r *ReleaseRequest) String() string {
	return fmt.Sprintf("Release [%s] %v fl=%v rfl=%v owner=%#x", &r.Header, r.Handle, r.Flags, r.ReleaseFlags, r.LockOwner)
}

// Respond replies to the request, indicating that the handle has been released.
func (r *ReleaseRequest) Respond() {
	buf := newBuffer(0)
	r.respond(buf)
}

// A DestroyRequest is sent by the kernel when unmounting the file system.
// No more requests will be received after this one, but it should still be
// responded to.
type DestroyRequest struct {
	Header `json:"-"`
}

var _ = Request(&DestroyRequest{})

func (r *DestroyRequest) String() string {
	return fmt.Sprintf("Destroy [%s]", &r.Header)
}

// Respond replies to the request.
func (r *DestroyRequest) Respond() {
	buf := newBuffer(0)
	r.respond(buf)
}

// A ForgetRequest is sent by the kernel when forgetting about r.Node
// as returned by r.N lookup requests.
type ForgetRequest struct {
	Header `json:"-"`
	N      uint64
}

var _ = Request(&ForgetRequest{})

func (r *ForgetRequest) String() string {
	return fmt.Sprintf("Forget [%s] %d", &r.Header, r.N)
}

// Respond replies to the request, indicating that the forgetfulness has been recorded.
func (r *ForgetRequest) Respond() {
	// Don't reply to forget messages.
	r.noResponse()
}

// A WriteRequest asks to write to an open file.
type WriteRequest struct {
	Header
	Handle    HandleID
	Offset    int64
	Data      []byte
	Flags     WriteFlags
	LockOwner uint64
	FileFlags OpenFlags
}

var _ = Request(&WriteRequest{})

func (r *WriteRequest) String() string {
	return fmt.Sprintf("Write [%s] %v %d @%d fl=%v lock=%d ffl=%v", &r.Header, r.Handle, len(r.Data), r.Offset, r.Flags, r.LockOwner, r.FileFlags)
}

type jsonWriteRequest struct {
	Handle HandleID
	Offset int64
	Len    uint64
	Flags  WriteFlags
}

func (r *WriteRequest) MarshalJSON() ([]byte, error) {
	j := jsonWriteRequest{
		Handle: r.Handle,
		Offset: r.Offset,
		Len:    uint64(len(r.Data)),
		Flags:  r.Flags,
	}
	return json.Marshal(j)
}

// Respond replies to the request with the given response.
func (r *WriteRequest) Respond(resp *WriteResponse) {
	buf := newBuffer(unsafe.Sizeof(writeOut{}))
	out := (*writeOut)(buf.alloc(unsafe.Sizeof(writeOut{})))
	out.Size = uint32(resp.Size)
	r.respond(buf)
}

// A WriteResponse replies to a write indicating how many bytes were written.
type WriteResponse struct {
	Size int
}

func (r *WriteResponse) String() string {
	return fmt.Sprintf("Write %d", r.Size)
}

// A SetattrRequest asks to change one or more attributes associated with a file,
// as indicated by Valid.
type SetattrRequest struct {
	Header `json:"-"`
	Valid  SetattrValid
	Handle HandleID
	Size   uint64
	Atime  time.Time
	Mtime  time.Time
	Mode   os.FileMode
	Uid    uint32
	Gid    uint32

	// OS X only
	Bkuptime time.Time
	Chgtime  time.Time
	Crtime   time.Time
	Flags    uint32 // see chflags(2)
}

var _ = Request(&SetattrRequest{})

func (r *SetattrRequest) String() string {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "Setattr [%s]", &r.Header)
	if r.Valid.Mode() {
		fmt.Fprintf(&buf, " mode=%v", r.Mode)
	}
	if r.Valid.Uid() {
		fmt.Fprintf(&buf, " uid=%d", r.Uid)
	}
	if r.Valid.Gid() {
		fmt.Fprintf(&buf, " gid=%d", r.Gid)
	}
	if r.Valid.Size() {
		fmt.Fprintf(&buf, " size=%d", r.Size)
	}
	if r.Valid.Atime() {
		fmt.Fprintf(&buf, " atime=%v", r.Atime)
	}
	if r.Valid.AtimeNow() {
		fmt.Fprintf(&buf, " atime=now")
	}
	if r.Valid.Mtime() {
		fmt.Fprintf(&buf, " mtime=%v", r.Mtime)
	}
	if r.Valid.MtimeNow() {
		fmt.Fprintf(&buf, " mtime=now")
	}
	if r.Valid.Handle() {
		fmt.Fprintf(&buf, " handle=%v", r.Handle)
	} else {
		fmt.Fprintf(&buf, " handle=INVALID-%v", r.Handle)
	}
	if r.Valid.LockOwner() {
		fmt.Fprintf(&buf, " lockowner")
	}
	if r.Valid.Crtime() {
		fmt.Fprintf(&buf, " crtime=%v", r.Crtime)
	}
	if r.Valid.Chgtime() {
		fmt.Fprintf(&buf, " chgtime=%v", r.Chgtime)
	}
	if r.Valid.Bkuptime() {
		fmt.Fprintf(&buf, " bkuptime=%v", r.Bkuptime)
	}
	if r.Valid.Flags() {
		fmt.Fprintf(&buf, " flags=%v", r.Flags)
	}
	return buf.String()
}

// Respond replies to the request with the given response,
// giving the updated attributes.
func (r *SetattrRequest) Respond(resp *SetattrResponse) {
	size := attrOutSize(r.Header.Conn.protocol)
	buf := newBuffer(size)
	out := (*attrOut)(buf.alloc(size))
	out.AttrValid = uint64(resp.Attr.Valid / time.Second)
	out.AttrValidNsec = uint32(resp.Attr.Valid % time.Second / time.Nanosecond)
	resp.Attr.attr(&out.Attr, r.Header.Conn.protocol)
	r.respond(buf)
}

// A SetattrResponse is the response to a SetattrRequest.
type SetattrResponse struct {
	Attr Attr // file attributes
}

func (r *SetattrResponse) String() string {
	return fmt.Sprintf("Setattr %v", r.Attr)
}

// A FlushRequest asks for the current state of an open file to be flushed
// to storage, as when a file descriptor is being closed.  A single opened Handle
// may receive multiple FlushRequests over its lifetime.
type FlushRequest struct {
	Header    `json:"-"`
	Handle    HandleID
	Flags     uint32
	LockOwner uint64
}

var _ = Request(&FlushRequest{})

func (r *FlushRequest) String() string {
	return fmt.Sprintf("Flush [%s] %v fl=%#x lk=%#x", &r.Header, r.Handle, r.Flags, r.LockOwner)
}

// Respond replies to the request, indicating that the flush succeeded.
func (r *FlushRequest) Respond() {
	buf := newBuffer(0)
	r.respond(buf)
}

// A RemoveRequest asks to remove a file or directory from the
// directory r.Node.
type RemoveRequest struct {
	Header `json:"-"`
	Name   string // name of the entry to remove
	Dir    bool   // is this rmdir?
}

var _ = Request(&RemoveRequest{})

func (r *RemoveRequest) String() string {
	return fmt.Sprintf("Remove [%s] %q dir=%v", &r.Header, r.Name, r.Dir)
}

// Respond replies to the request, indicating that the file was removed.
func (r *RemoveRequest) Respond() {
	buf := newBuffer(0)
	r.respond(buf)
}

// A SymlinkRequest is a request to create a symlink making NewName point to Target.
type SymlinkRequest struct {
	Header          `json:"-"`
	NewName, Target string
}

var _ = Request(&SymlinkRequest{})

func (r *SymlinkRequest) String() string {
	return fmt.Sprintf("Symlink [%s] from %q to target %q", &r.Header, r.NewName, r.Target)
}

// Respond replies to the request, indicating that the symlink was created.
func (r *SymlinkRequest) Respond(resp *SymlinkResponse) {
	size := entryOutSize(r.Header.Conn.protocol)
	buf := newBuffer(size)
	out := (*entryOut)(buf.alloc(size))
	out.Nodeid = uint64(resp.Node)
	out.Generation = resp.Generation
	out.EntryValid = uint64(resp.EntryValid / time.Second)
	out.EntryValidNsec = uint32(resp.EntryValid % time.Second / time.Nanosecond)
	out.AttrValid = uint64(resp.Attr.Valid / time.Second)
	out.AttrValidNsec = uint32(resp.Attr.Valid % time.Second / time.Nanosecond)
	resp.Attr.attr(&out.Attr, r.Header.Conn.protocol)
	r.respond(buf)
}

// A SymlinkResponse is the response to a SymlinkRequest.
type SymlinkResponse struct {
	LookupResponse
}

func (r *SymlinkResponse) String() string {
	return fmt.Sprintf("Symlink %v", r.LookupResponse.string())
}

// A ReadlinkRequest is a request to read a symlink's target.
type ReadlinkRequest struct {
	Header `json:"-"`
}

var _ = Request(&ReadlinkRequest{})

func (r *ReadlinkRequest) String() string {
	return fmt.Sprintf("Readlink [%s]", &r.Header)
}

func (r *ReadlinkRequest) Respond(target string) {
	buf := newBuffer(uintptr(len(target)))
	buf = append(buf, target...)
	r.respond(buf)
}

// A LinkRequest is a request to create a hard link.
type LinkRequest struct {
	Header  `json:"-"`
	OldNode NodeID
	NewName string
}

var _ = Request(&LinkRequest{})

func (r *LinkRequest) String() string {
	return fmt.Sprintf("Link [%s] node %d to %q", &r.Header, r.OldNode, r.NewName)
}

func (r *LinkRequest) Respond(resp *LookupResponse) {
	size := entryOutSize(r.Header.Conn.protocol)
	buf := newBuffer(size)
	out := (*entryOut)(buf.alloc(size))
	out.Nodeid = uint64(resp.Node)
	out.Generation = resp.Generation
	out.EntryValid = uint64(resp.EntryValid / time.Second)
	out.EntryValidNsec = uint32(resp.EntryValid % time.Second / time.Nanosecond)
	out.AttrValid = uint64(resp.Attr.Valid / time.Second)
	out.AttrValidNsec = uint32(resp.Attr.Valid % time.Second / time.Nanosecond)
	resp.Attr.attr(&out.Attr, r.Header.Conn.protocol)
	r.respond(buf)
}

// A RenameRequest is a request to rename a file.
type RenameRequest struct {
	Header           `json:"-"`
	NewDir           NodeID
	OldName, NewName string
}

var _ = Request(&RenameRequest{})

func (r *RenameRequest) String() string {
	return fmt.Sprintf("Rename [%s] from %q to dirnode %v %q", &r.Header, r.OldName, r.NewDir, r.NewName)
}

func (r *RenameRequest) Respond() {
	buf := newBuffer(0)
	r.respond(buf)
}

type MknodRequest struct {
	Header `json:"-"`
	Name   string
	Mode   os.FileMode
	Rdev   uint32
	// Umask of the request. Not supported on OS X.
	Umask os.FileMode
}

var _ = Request(&MknodRequest{})

func (r *MknodRequest) String() string {
	return fmt.Sprintf("Mknod [%s] Name %q mode=%v umask=%v rdev=%d", &r.Header, r.Name, r.Mode, r.Umask, r.Rdev)
}

func (r *MknodRequest) Respond(resp *LookupResponse) {
	size := entryOutSize(r.Header.Conn.protocol)
	buf := newBuffer(size)
	out := (*entryOut)(buf.alloc(size))
	out.Nodeid = uint64(resp.Node)
	out.Generation = resp.Generation
	out.EntryValid = uint64(resp.EntryValid / time.Second)
	out.EntryValidNsec = uint32(resp.EntryValid % time.Second / time.Nanosecond)
	out.AttrValid = uint64(resp.Attr.Valid / time.Second)
	out.AttrValidNsec = uint32(resp.Attr.Valid % time.Second / time.Nanosecond)
	resp.Attr.attr(&out.Attr, r.Header.Conn.protocol)
	r.respond(buf)
}

type FsyncRequest struct {
	Header `json:"-"`
	Handle HandleID
	// TODO bit 1 is datasync, not well documented upstream
	Flags uint32
	Dir   bool
}

var _ = Request(&FsyncRequest{})

func (r *FsyncRequest) String() string {
	return fmt.Sprintf("Fsync [%s] Handle %v Flags %v", &r.Header, r.Handle, r.Flags)
}

func (r *FsyncRequest) Respond() {
	buf := newBuffer(0)
	r.respond(buf)
}

// An InterruptRequest is a request to interrupt another pending request. The
// response to that request should return an error status of EINTR.
type InterruptRequest struct {
	Header `json:"-"`
	IntrID RequestID // ID of the request to be interrupt.
}

var _ = Request(&InterruptRequest{})

func (r *InterruptRequest) Respond() {
	// nothing to do here
	r.noResponse()
}

func (r *InterruptRequest) String() string {
	return fmt.Sprintf("Interrupt [%s] ID %v", &r.Header, r.IntrID)
}

// An ExchangeDataRequest is a request to exchange the contents of two
// files, while leaving most metadata untouched.
//
// This request comes from OS X exchangedata(2) and represents its
// specific semantics. Crucially, it is very different from Linux
// renameat(2) RENAME_EXCHANGE.
//
// https://developer.apple.com/library/mac/documentation/Darwin/Reference/ManPages/man2/exchangedata.2.html
type ExchangeDataRequest struct {
	Header           `json:"-"`
	OldDir, NewDir   NodeID
	OldName, NewName string
	// TODO options
}

var _ = Request(&ExchangeDataRequest{})

func (r *ExchangeDataRequest) String() string {
	// TODO options
	return fmt.Sprintf("ExchangeData [%s] %v %q and %v %q", &r.Header, r.OldDir, r.OldName, r.NewDir, r.NewName)
}

func (r *ExchangeDataRequest) Respond() {
	buf := newBuffer(0)
	r.respond(buf)
}
