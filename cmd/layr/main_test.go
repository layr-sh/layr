package main

import (
	"errors"
	"os"
	"testing"
)

func TestLayrMainSuccessUnit(t *testing.T) {
	oldArgs := os.Args
	oldCliRunFunc := cliRunFunc
	oldExitFunc := exitFunc
	defer func() {
		os.Args = oldArgs
		cliRunFunc = oldCliRunFunc
		exitFunc = oldExitFunc
	}()

	cliRunFunc = func(args []string) error {
		return nil
	}

	exitCalled := false
	exitFunc = func(code int) {
		exitCalled = true
	}

	os.Args = []string{"layr", "up"}
	main()

	if exitCalled {
		t.Fatal("expected main to exit normally without calling exitFunc")
	}
}

func TestLayrMainErrorUnit(t *testing.T) {
	oldArgs := os.Args
	oldCliRunFunc := cliRunFunc
	oldExitFunc := exitFunc
	defer func() {
		os.Args = oldArgs
		cliRunFunc = oldCliRunFunc
		exitFunc = oldExitFunc
	}()

	cliRunFunc = func(args []string) error {
		return errors.New("simulated cli error")
	}

	exitCode := 0
	exitFunc = func(code int) {
		exitCode = code
	}

	os.Args = []string{"layr", "invalid-cmd"}
	main()

	if exitCode != 1 {
		t.Fatalf("expected exit code 1, got %d", exitCode)
	}
}
