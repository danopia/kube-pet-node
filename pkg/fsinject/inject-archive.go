package fsinject

import (
	"archive/tar"
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"time"
)

func StartArchiveExtraction(ctx context.Context, dest string) (*ArchiveExtraction, error) {
	var cmd *exec.Cmd
	if os.Getuid() == 1 {
		cmd = exec.CommandContext(ctx, "tar", "-xf", "-", "-C", dest)
	} else {
		cmd = exec.CommandContext(ctx, "sudo", "tar", "-xf", "-", "-C", dest)
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	return &ArchiveExtraction{
		command: cmd,
		stdin:   stdin,
		tar:     tar.NewWriter(stdin),
		Now:     time.Now(),
	}, nil
}

type ArchiveExtraction struct {
	Now time.Time

	err     error
	command *exec.Cmd
	stdin   io.WriteCloser
	tar     *tar.Writer
}

func (ae *ArchiveExtraction) WriteFile(name string, mode int64, body []byte) {
	err := ae.tar.WriteHeader(&tar.Header{
		Name:    name,
		Size:    int64(len(body)),
		Mode:    mode, //int64(0644),
		ModTime: ae.Now,
	})
	if err != nil {
		ae.err = err
		return
	}

	_, err = io.Copy(ae.tar, bytes.NewReader(body))
	if err != nil {
		ae.err = err
		return
		// return errors.New(fmt.Sprintf("Could not copy the file '%s' data to the tarball, got error '%s'", filePath, err.Error()))
	}
}

func (ae *ArchiveExtraction) WriteSymLink(name string, target string) {
	err := ae.tar.WriteHeader(&tar.Header{
		Typeflag: tar.TypeSymlink,
		Linkname: target,
		Name:     name,
		// Size:     0,
		Mode:    int64(0644),
		ModTime: ae.Now,
	})
	if err != nil {
		ae.err = err
	}
}

func (ae *ArchiveExtraction) Finish() error {
	err1 := ae.tar.Close()
	err2 := ae.stdin.Close()
	err3 := ae.command.Wait()

	if ae.err != nil {
		return ae.err
	}
	if err1 != nil {
		return err1
	}
	if err2 != nil {
		return err2
	}
	if err3 != nil {
		return err3
	}
	return nil
}
