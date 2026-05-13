//go:build windows

package browsh

import (
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
	"unsafe"

	"github.com/go-errors/errors"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

func getFirefoxPath() string {
	versionString := getWindowsFirefoxVersionString()
	flavor := getFirefoxFlavor()

	k, err := registry.OpenKey(
		registry.LOCAL_MACHINE,
		`Software\Mozilla\`+flavor+`\`+versionString+`\Main`,
		registry.QUERY_VALUE)
	if err != nil {
		Shutdown(fmt.Errorf("Error reading Windows registry: %w", err))
	}
	defer k.Close()

	path, _, err := k.GetStringValue("PathToExe")
	if err != nil {
		Shutdown(fmt.Errorf("Error reading Windows registry: %w", err))
	}

	return path
}

func getWindowsFirefoxVersionString() string {
	flavor := getFirefoxFlavor()

	k, err := registry.OpenKey(
		registry.LOCAL_MACHINE,
		`Software\Mozilla\`+flavor,
		registry.QUERY_VALUE)
	if err != nil {
		Shutdown(fmt.Errorf("Error reading Windows registry: %w", err))
	}
	defer k.Close()

	versionString, _, err := k.GetStringValue("CurrentVersion")
	if err != nil {
		Shutdown(fmt.Errorf("Error reading Windows registry: %w", err))
	}

	slog.Info("Windows registry Firefox", "version", versionString)

	return versionString
}

func getFirefoxFlavor() string {
	flavor := "null"
	k, err := registry.OpenKey(
		registry.LOCAL_MACHINE,
		`Software\Mozilla\Mozilla Firefox`,
		registry.QUERY_VALUE)

	if err == nil {
		flavor = "Mozilla Firefox"
	}
	defer k.Close()

	if flavor == "null" {
		k, err := registry.OpenKey(
			registry.LOCAL_MACHINE,
			`Software\Mozilla\Firefox Developer Edition`,
			registry.QUERY_VALUE)

		if err == nil {
			flavor = "Firefox Developer Edition"
		}
		defer k.Close()
	}

	if flavor == "null" {
		k, err := registry.OpenKey(
			registry.LOCAL_MACHINE,
			`Software\Mozilla\Nightly`,
			registry.QUERY_VALUE)

		if err == nil {
			flavor = "Nightly"
		}
		defer k.Close()
	}

	if flavor == "null" {
		Shutdown(errors.New("Could not find Firefox on your registry"))
	}
	return flavor
}

func ensureFirefoxVersion(path string) {
	versionString := getWindowsFirefoxVersionString()
	pieces := strings.Split(versionString, " ")
	version := pieces[0]
	if versionOrdinal(version) < versionOrdinal("57") {
		message := "Installed Firefox version " + version + " is too old. " +
			"Firefox 57 or newer is needed."
		Shutdown(errors.New(message))
	}
}

var jobHandle windows.Handle

func osProcessConfig(cmd *exec.Cmd) {}

func osProcessTracker(cmd *exec.Cmd) {
	if jobHandle == 0 {
		handle, err := windows.CreateJobObject(nil, nil)
		if err == nil {
			info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{
				BasicLimitInformation: windows.JOBOBJECT_BASIC_LIMIT_INFORMATION{
					LimitFlags: windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE,
				},
			}
			_, err = windows.SetInformationJobObject(
				handle,
				windows.JobObjectExtendedLimitInformation,
				uintptr(unsafe.Pointer(&info)),
				uint32(unsafe.Sizeof(info)))
			if err == nil {
				jobHandle = handle
			} else {
				slog.Error("SetInformationJobObject error", "error", err)
			}
		} else {
			slog.Error("CreateJobObject error", "error", err)
		}
	}
	if jobHandle != 0 && cmd.Process != nil {
		procHandle, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(cmd.Process.Pid))
		if err == nil {
			err = windows.AssignProcessToJobObject(jobHandle, procHandle)
			windows.CloseHandle(procHandle)
			if err != nil {
				slog.Error("AssignProcessToJobObject error", "error", err)
			}
		} else {
			slog.Error("OpenProcess error", "error", err)
		}
	}
}
