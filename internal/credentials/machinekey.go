package credentials

import (
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// deriveMachineKey derives an AES-256 key from the machine ID and given salt.
// It uses SHA-256(machineID + salt) to produce a 32-byte AES-256 key.
//
// The machine ID is platform-dependent:
//   - Linux: content of /etc/machine-id (preferred) or /var/lib/dbus/machine-id
//   - macOS: output of `sysctl -n kern.uuid`
//   - Windows: MachineGuid from HKLM\SOFTWARE\Microsoft\Cryptography
func deriveMachineKey(salt []byte) ([]byte, error) {
	machineID, err := readMachineID()
	if err != nil {
		return nil, fmt.Errorf("machine id: %w", err)
	}

	h := sha256.Sum256(append([]byte(machineID), salt...))
	return h[:], nil
}

// readMachineID reads the machine ID from the platform-specific source.
func readMachineID() (string, error) {
	switch runtime.GOOS {
	case "linux":
		return readLinuxMachineID()
	case "darwin":
		return readDarwinMachineID()
	case "windows":
		return readWindowsMachineID()
	default:
		return "", fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
}

func readLinuxMachineID() (string, error) {
	// Prefer /etc/machine-id (systemd), fall back to /var/lib/dbus/machine-id.
	id, err := os.ReadFile("/etc/machine-id")
	if err == nil {
		return strings.TrimSpace(string(id)), nil
	}
	id, err = os.ReadFile("/var/lib/dbus/machine-id")
	if err == nil {
		return strings.TrimSpace(string(id)), nil
	}
	return "", fmt.Errorf("cannot read machine-id from /etc/machine-id or /var/lib/dbus/machine-id")
}

func readDarwinMachineID() (string, error) {
	out, err := exec.Command("sysctl", "-n", "kern.uuid").Output()
	if err != nil {
		return "", fmt.Errorf("sysctl kern.uuid: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func readWindowsMachineID() (string, error) {
	out, err := exec.Command("powershell", "-Command",
		"Get-ItemProperty -Path 'HKLM:\\SOFTWARE\\Microsoft\\Cryptography' -Name 'MachineGuid' | Select-Object -ExpandProperty MachineGuid",
	).Output()
	if err != nil {
		return "", fmt.Errorf("read MachineGuid from registry: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}
