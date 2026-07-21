//go:build windows

package video

import "golang.org/x/sys/windows"

func diskFreeBytes(path string) (uint64, error) {
	ptr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, err
	}
	var available, total, free uint64
	if err := windows.GetDiskFreeSpaceEx(ptr, &available, &total, &free); err != nil {
		return 0, err
	}
	return available, nil
}
