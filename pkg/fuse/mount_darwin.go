package fuse

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
)

// TODO: Update comment. Blocks until mount.
func mount(dir string, conf *mountConfig) (*os.File, error) {
	locations := conf.osxfuseLocations
	if locations == nil {
		locations = DefaultOSXFuseLocations
	}
	for _, loc := range locations {
		if _, err := os.Stat(loc.Mount); os.IsNotExist(err) {
			continue
		}

		if err := loadFUSE(loc.Load); err != nil {
			return nil, err
		}
		dev, err := openFUSE(loc.DevicePrefix)
		if err != nil {
			return nil, err
		}
		if err := callMount(loc.Mount, loc.DaemonVar, dir, conf, dev); err != nil {
			dev.Close()
			return nil, err
		}
		return dev, nil
	}

	// OSXFUSE installation is not detected. Caller needs to ensure OSXFUSE is
	// installed, or provide installation location via fuse.OSXFUSELocations.
	return nil, errors.New("cannot locate OSXFUSE")
}

func loadFUSE(bin string) error {
	cmd := exec.Command(bin)
	cmd.Dir = "/"
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}

func openFUSE(devPrefix string) (*os.File, error) {
	for i := uint64(0); ; i++ {
		path := devPrefix + strconv.FormatUint(i, 10)
		file, err := os.OpenFile(path, os.O_RDWR, 0000)
		if os.IsNotExist(err) {
			if i == 0 {
				// Not even the first device was found -> fuse is not loaded
				return nil, errors.New("osxfuse is not loaded")
			}

			// We've run out of kernel-provided devices
			return nil, errors.New("no available fuse devices")
		}

		if err, ok := err.(*os.PathError); ok && err.Err == syscall.EBUSY {
			// Try the next one
			continue
		}
		if err != nil {
			return nil, err
		}
		return file, nil
	}
}

func callMount(bin string, daemonVar string, dir string, conf *mountConfig, f *os.File) error {
	for k, v := range conf.options {
		if strings.Contains(k, ",") || strings.Contains(v, ",") {
			// Silly limitation but the mount helper does not understand any
			// escaping. See TestMountOptionCommaError.
			return fmt.Errorf("mount options cannot contain commas on darwin: %q=%q", k, v)
		}
	}
	cmd := exec.Command(
		bin,
		"-o", conf.getOptions(),
		// Tell osxfuse-kext how large our buffer is. It must split
		// writes larger than this into multiple writes.
		//
		// OSXFUSE seems to ignore InitResponse.MaxWrite, and uses
		// this instead.
		"-o", "iosize="+strconv.FormatUint(maxWrite, 10),
		// refers to fd passed in cmd.ExtraFiles
		"3",
		dir,
	)
	cmd.ExtraFiles = []*os.File{f}
	cmd.Env = os.Environ()
	// OSXFUSE <3.3.0
	cmd.Env = append(cmd.Env, "MOUNT_FUSEFS_CALL_BY_LIB=")
	// OSXFUSE >=3.3.0
	cmd.Env = append(cmd.Env, "MOUNT_OSXFUSE_CALL_BY_LIB=")

	daemon := os.Args[0]
	if daemonVar != "" {
		cmd.Env = append(cmd.Env, daemonVar+"="+daemon)
	}

	// We suppress stdout from setting up osxfuse. Anything received on stderr
	// we propagate back up to the user, as an error, if useful.
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to set up mount_osxfusefs stderr: %v", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start mount_osxfusefs: %v", err)
	}

	var mnterr error
	scanner := bufio.NewScanner(stderr)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasSuffix(line, "No such file or directory") {
			// Return what we grabbed from stderr as the real error
			mnterr = fmt.Errorf("mountpoint does not exist: %v", dir)
		} else if strings.HasSuffix(line, "is itself on a OSXFUSE volume") {
			mnterr = fmt.Errorf("mountpoint %v is itself on a OSXFUSE volume", dir)
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("failed to read from stderr: %v", err)
	}

	if err := cmd.Wait(); err != nil {
		// See if we have a better error to report.
		if mnterr != nil {
			return mnterr
		}
		// Fall back to generic message.
		return fmt.Errorf("failed to mount: %v", err)
	}
	return nil
}
