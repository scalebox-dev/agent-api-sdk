package driver

import "fmt"

func errUnsupported(name string) error {
	return fmt.Errorf("%s driver is not supported on this platform", name)
}
