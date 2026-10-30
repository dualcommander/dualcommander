package main

import (
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestGraphicsAutodetection(t *testing.T) {
	if graphicsProbeTimeout <= 0 || graphicsProbeTimeout > 2*time.Second {
		t.Fatal("graphics probe must have a short, bounded timeout")
	}
	t.Setenv("DUALCOMMANDER_INTERNAL_GRAPHICS", "stale")
	for _, mode := range []string{"probe", "run"} {
		cmd := graphicsCommand(os.Args[0], mode, "folder with spaces")
		if !reflect.DeepEqual(cmd.Args, []string{os.Args[0], "folder with spaces"}) {
			t.Fatalf("internal graphics mode leaked into command-line arguments: %v", cmd.Args)
		}
		count := 0
		for _, value := range cmd.Env {
			if strings.HasPrefix(value, "DUALCOMMANDER_INTERNAL_GRAPHICS=") {
				count++
				if value != "DUALCOMMANDER_INTERNAL_GRAPHICS="+mode {
					t.Fatal(value)
				}
			}
		}
		if count != 1 {
			t.Fatalf("want one private graphics mode, got %d", count)
		}
	}
}

func TestGraphicsSelection(t *testing.T) {
	for _, unavailable := range []bool{false, true} {
		calls := 0
		wantErr := errors.New("application failed")
		err := runGraphics(func() error {
			if unavailable {
				return errors.New("OpenGL unavailable")
			}
			return nil
		}, func(software bool) error {
			calls++
			if software != unavailable {
				t.Errorf("software=%v, want %v", software, unavailable)
			}
			return wantErr
		})
		if calls != 1 || !errors.Is(err, wantErr) {
			t.Fatalf("calls=%d, error=%v; application failure must propagate without retry", calls, err)
		}
	}
}
