package platform

import "runtime"

func runtimeGOOS() string {
	return runtime.GOOS
}
