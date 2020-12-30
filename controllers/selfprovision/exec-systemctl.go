package selfprovision

import (
	"context"
	"log"
	"os"
	"os/exec"
)

func SystemctlUnitAction(ctx context.Context, unitName string, action string) error {
	var cmd *exec.Cmd
	if os.Getuid() == 1 {
		cmd = exec.CommandContext(ctx, "systemctl", action, unitName)
	} else {
		cmd = exec.CommandContext(ctx, "sudo", "systemctl", action, unitName)
	}

	out, err := cmd.Output()
	if err != nil {
		return err
	}

	log.Println("systemctl said:", out)
	return nil
}
