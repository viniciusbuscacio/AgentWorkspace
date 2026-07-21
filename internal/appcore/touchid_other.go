//go:build !darwin && !windows

package appcore

import "fmt"

func touchIDAvailableNative() bool { return false }

func touchIDPromptNative(_ string) error { return fmt.Errorf("Touch ID not available") }

func touchIDKeychainHas(_, _ string) bool { return false }

func touchIDKeychainSet(_, _, _ string) error { return fmt.Errorf("Touch ID not available") }

func touchIDKeychainGet(_, _ string) (string, error) { return "", fmt.Errorf("Touch ID not available") }

func touchIDKeychainDelete(_, _ string) error { return nil }
