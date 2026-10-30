package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"syscall"

	"github.com/go-gl/glfw/v3.4/glfw"
)

func graphicsStartup() (bool, error) {
	mode := os.Getenv(graphicsModeEnv)
	os.Unsetenv(graphicsModeEnv)
	switch mode {
	case "run":
		return false, nil
	case "probe":
		return true, probeOpenGL()
	}
	exe, err := os.Executable()
	if err != nil {
		return true, err
	}
	softwareExe := filepath.Join(filepath.Dir(exe), "software", filepath.Base(exe))
	if _, err := os.Stat(softwareExe); os.IsNotExist(err) {
		// ponytail: unpackaged developer builds use system OpenGL; fallback ships in release packages.
		return false, nil
	} else if err != nil {
		return true, err
	}
	err = runGraphics(func() error {
		ctx, cancel := context.WithTimeout(context.Background(), graphicsProbeTimeout)
		defer cancel()
		cmd := exec.CommandContext(ctx, exe)
		cmd.Env = graphicsCommand(exe, "probe").Env
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}, func(software bool) error {
		selected := exe
		if software {
			selected = softwareExe
		}
		slog.Info("OpenGL renderer selected", "software", software)
		cmd := graphicsCommand(selected, "run", os.Args[1:]...)
		cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
		return cmd.Run()
	})
	return true, err
}

func probeOpenGL() error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if err := glfw.Init(); err != nil {
		return err
	}
	defer glfw.Terminate()
	// Match Fyne's desktop context requirements, without showing a window.
	glfw.WindowHint(glfw.Visible, glfw.False)
	glfw.WindowHint(glfw.ContextVersionMajor, 2)
	glfw.WindowHint(glfw.ContextVersionMinor, 1)
	if runtime.GOARCH == "arm64" {
		glfw.WindowHint(glfw.ClientAPI, glfw.OpenGLESAPI)
		glfw.WindowHint(glfw.ContextVersionMinor, 0)
	}
	w, err := glfw.CreateWindow(1, 1, "OpenGL probe", nil, nil)
	if err != nil {
		return fmt.Errorf("system OpenGL probe: %w", err)
	}
	w.MakeContextCurrent()
	w.Destroy()
	return nil
}
