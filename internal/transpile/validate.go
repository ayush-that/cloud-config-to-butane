package transpile

import (
	"fmt"

	"github.com/coreos/butane/config"
	"github.com/coreos/butane/config/common"
)

// Validate runs Butane's own translator over the bytes; nil means valid Butane 1.1.0 that becomes Ignition 3.4.
func Validate(butane []byte) error {
	_, report, err := config.TranslateBytes(butane, common.TranslateBytesOptions{})
	if err != nil {
		return fmt.Errorf("butane translate: %w", err)
	}
	if report.IsFatal() {
		return fmt.Errorf("butane report is fatal:\n%s", report.String())
	}
	return nil
}
