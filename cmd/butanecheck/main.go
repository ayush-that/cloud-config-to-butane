// Command butanecheck verifies that a Butane config on stdin translates to a valid Ignition config.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/coreos/butane/config"
	"github.com/coreos/butane/config/common"
)

func main() {
	input, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintln(os.Stderr, "butanecheck: "+err.Error())
		os.Exit(1)
	}
	ign, report, err := config.TranslateBytes(input, common.TranslateBytesOptions{})
	if err != nil {
		fmt.Fprintln(os.Stderr, "butanecheck: translate failed: "+err.Error())
		if s := report.String(); s != "" {
			fmt.Fprintln(os.Stderr, s)
		}
		os.Exit(1)
	}
	if report.IsFatal() {
		fmt.Fprintln(os.Stderr, "butanecheck: fatal report:\n"+report.String())
		os.Exit(1)
	}
	var parsed struct {
		Ignition struct {
			Version string `json:"version"`
		} `json:"ignition"`
	}
	_ = json.Unmarshal(ign, &parsed)
	fmt.Printf("OK: butane translated to ignition (fatal=false, ignition version=%s, %d bytes)\n",
		parsed.Ignition.Version, len(ign))
}
