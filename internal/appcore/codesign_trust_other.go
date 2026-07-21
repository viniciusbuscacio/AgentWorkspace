//go:build !darwin

package appcore

import "fmt"

func codesignTrustStatusNative() (status int, certName string) { return 3, "" }

func codesignTrustGrantNative() error {
	return fmt.Errorf("code-signing trust settings are only available on macOS")
}
