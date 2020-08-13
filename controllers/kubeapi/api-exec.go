package kubeapi

import (
	"context"
	"fmt"
	"io"
	"log"

	// utilexec "k8s.io/utils/exec"

	vkapi "github.com/virtual-kubelet/virtual-kubelet/node/api"

	"github.com/danopia/kube-pet-node/controllers/pods"
	"github.com/danopia/kube-pet-node/podman"
)

func (ka *KubeApi) RunInContainer(ctx context.Context, namespace, podName, containerName string, cmd []string, attach vkapi.AttachIO) error {
	log.Println("RunInContainer(", namespace, podName, containerName, cmd, attach, ")")

	session, err := ka.podManager.StartExecInPod(ctx, pods.PodCoord{namespace, podName}, containerName, &podman.ContainerExecOptions{
		Cmd:          cmd,
		Tty:          attach.TTY(),
		AttachStdin:  attach.Stdin() != nil,
		AttachStdout: attach.Stdout() != nil,
		AttachStderr: attach.Stderr() != nil,
	})
	if err != nil {
		log.Println("exec init err:", err)
		return err
	}

	go func() {
		for termSize := range attach.Resize() {
			log.Println("exec resize:", termSize)
			if err := session.Resize(ctx, &podman.ExecResizeOptions{
				Width:  termSize.Width,
				Height: termSize.Height,
			}); err != nil {
				log.Println("WARN: Resize exec session to", termSize.Width, termSize.Height, "failed:", err)
			}
		}
	}()

	input, output, err := session.Start(ctx)
	if err != nil {
		log.Println("exec start err:", err)
		// TODO: cancel exec?
		return err
	}
	log.Println("exec start done", input, output)

	if input != nil && attach.Stdin() != nil {
		go io.Copy(input, attach.Stdin())
	}

	if attach.TTY() {
		// TODO: why does TTY not have stdout/stderr split??
		io.Copy(attach.Stdout(), output)
	} else {
		podman.DemuxRawStream(output, attach.Stdout(), attach.Stderr(), false)
	}

	if sessResult, err := session.Inspect(ctx); err != nil {
		log.Println("kubeapi WARN: Post-exec inspection failed:", err)
	} else if sessResult.ExitCode > 0 {
		return exitError{sessResult.ExitCode}
	}
	return nil
}

type exitError struct {
	status int
}

func (ee exitError) String() string {
	return fmt.Sprintf("command terminated with exit code %v", ee.status)
}
func (ee exitError) Error() string {
	return fmt.Sprintf("command terminated with exit code %v", ee.status)
}
func (ee exitError) Exited() bool {
	return true
}
func (ee exitError) ExitStatus() int {
	return ee.status
}
