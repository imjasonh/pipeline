// +build windows

package osversion

import (
	"fmt"
	"log"
	"sync"

	"golang.org/x/sys/windows/registry"
)

const keyPrefix string = `regedit:LOCAL_MACHINE\SOFTWARE\Microsoft\Windows NT\CurrentVersion`

var (
	osversion string
	once      sync.Once
)

func Get() string {
	once.Do(func() {
		k, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows NT\CurrentVersion`, registry.QUERY_VALUE)
		if err != nil {
			log.Fatalf("osversion: open Windows %s failed: %v", keyPrefix, err)
		}
		defer k.Close()

		maj, _, err := k.GetIntegerValue("CurrentMajorVersionNumber")
		if err != nil {
			log.Fatalf("osversion: get %s\\CurrentMajorVersionNumber failed: %v", keyPrefix, err)
		}

		min, _, err := k.GetIntegerValue("CurrentMinorVersionNumber")
		if err != nil {
			log.Fatalf("osversion: get %s\\CurrentMinorVersionNumber failed: %v", keyPrefix, err)
		}

		build, _, err := k.GetStringValue("CurrentBuildNumber")
		if err != nil {
			log.Fatalf("osversion: get %s\\CurrentBuildNumber failed: %v", keyPrefix, err)
		}

		// TODO: get the fourth component from somewhere.

		osversion = fmt.Sprintf("%d.%d.%s", maj, min, build)
	})
	return osversion
}
