package lvm_test

import (
	"os/exec"
	"testing"

	"github.com/labring/sealos-state-metrics/pkg/lvm"
)

func TestListLVMVolumeGroup(t *testing.T) {
	// Check if vgs command exists
	if _, err := exec.LookPath("vgs"); err != nil {
		t.Skip("Skipping test: vgs command not found. LVM may not be installed on this system.")
	}

	vgs, err := lvm.ListLVMVolumeGroup(false)
	if err != nil {
		t.Fatalf("Failed to list LVM volume group: %v", err)
	}

	if len(vgs) == 0 {
		t.Skip("No LVM volume groups found on this system")
	}

	for i := range vgs {
		t.Logf("Found LVM volume group: %#+v", vgs[i])
	}
}
