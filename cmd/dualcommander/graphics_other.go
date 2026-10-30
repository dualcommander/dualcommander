//go:build !windows

package main

func graphicsStartup() (bool, error) { return false, nil }
