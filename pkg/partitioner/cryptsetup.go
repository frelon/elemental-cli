package partitioner

import (
	"fmt"
	"os/exec"
	"strings"

	v1 "github.com/rancher/elemental-cli/pkg/types/v1"
)

func EncryptDevice(runner v1.Runner, device, mappedName string, slots []v1.KeySlot) error {
	logger := runner.GetLogger()

	if len(slots) == 0 {
		return fmt.Errorf("Needs at least 1 key-slot to encrypt %s", device)
	}

	firstSlot := slots[0]

	cmd := runner.InitCmd("cryptsetup", "luksFormat", "--key-slot", fmt.Sprintf("%d", firstSlot.Slot), device, "-")
	unlockCmd(cmd, firstSlot)

	stdout, err := runner.RunCmd(cmd)
	if err != nil {
		logger.Errorf("Error formatting device %s: %s", device, stdout)
		return err
	}

	cmd = runner.InitCmd("cryptsetup", "open", device, mappedName)

	unlockCmd(cmd, firstSlot)

	stdout, err = runner.RunCmd(cmd)
	if err != nil {
		logger.Errorf("Error opening device %s: %s", device, stdout)
	}

	return err
}

func unlockCmd(cmd *exec.Cmd, slot v1.KeySlot) {
	if slot.Passphrase != "" {
		cmd.Stdin = strings.NewReader(slot.Passphrase)
	}

	if slot.KeyFile != "" {
		cmd.Args = append(cmd.Args, "--key-file", slot.KeyFile)
	}
}
