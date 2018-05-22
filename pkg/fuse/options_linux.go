package fuse

func localVolume(conf *mountConfig) error {
	return nil
}

func volumeName(name string) MountOption {
	return noop
}

func daemonTimeout(name string) MountOption {
	return noop
}

func noAppleXattr(conf *mountConfig) error {
	return nil
}

func noAppleDouble(conf *mountConfig) error {
	return nil
}

func exclCreate(conf *mountConfig) error {
	return nil
}
