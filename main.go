package main

import (
	"bufio"
	"bytes"
	"encoding/xml"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// network interface from virsh XML
type Interface struct {
	Mac struct {
		Address string `xml:"address,attr"`
	} `xml:"mac"`
}

// virsh domain XML structure
type Domain struct {
	Interfaces []Interface `xml:"devices>interface"`
}

// a single generated .link file: its filename and its contents
type LinkFile struct {
	Name    string
	Content string
}

const systemdNetworkDir = "/etc/systemd/network/"

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	help := flag.Bool("help", false, "Display help information")
	dryRun := flag.Bool("dry-run", false, "Show the .link file contents without copying to VM")
	prefix := flag.String("prefix", "net", "Interface name prefix")
	startIndex := flag.Int("start-index", 0, "Starting index for interface numbering")
	filePrefix := flag.Int("file-prefix", 70, "Numeric filename prefix for .link files (e.g. 70 -> 70-net0.link)")
	verbose := flag.Bool("verbose", false, "Display verbose output")

	flag.Usage = displayHelp
	flag.Parse()

	if *help {
		displayHelp()
		return nil
	}

	args := flag.Args()
	if len(args) != 1 {
		displayHelp()
		return fmt.Errorf("VM name is required")
	}
	vmName := args[0]

	if *verbose {
		fmt.Printf("Processing VM: %s\n", vmName)
	}

	if err := checkVMStatus(vmName); err != nil {
		return err
	}

	macAddresses, err := getMacAddresses(vmName)
	if err != nil {
		return fmt.Errorf("failed to get MAC addresses: %v", err)
	}

	if len(macAddresses) == 0 {
		return fmt.Errorf("no network interfaces found in the VM")
	}

	if *verbose {
		fmt.Printf("Found %d network interfaces\n", len(macAddresses))
	}

	linkFiles := generateLinkFiles(macAddresses, *prefix, *startIndex, *filePrefix)

	for _, lf := range linkFiles {
		fmt.Printf("Generated %s:\n", lf.Name)
		fmt.Println("----------------------------------------")
		fmt.Print(lf.Content)
		fmt.Println("----------------------------------------")
	}

	if *dryRun {
		fmt.Println("Dry run completed. .link files not copied to VM.")
		return nil
	}

	if err := copyLinkFilesToVM(linkFiles, vmName, *verbose); err != nil {
		return fmt.Errorf("failed to copy .link files to VM: %v", err)
	}

	fmt.Printf("Successfully configured systemd.link interface names for VM '%s'\n", vmName)
	fmt.Printf("Start the VM with: sudo virsh start %s\n", vmName)
	return nil
}

func displayHelp() {
	fmt.Println("kvm-vm-systemd-link - Set persistent network interface names for KVM VMs via systemd.link")
	fmt.Println("\nUsage:")
	fmt.Println("  kvm-vm-systemd-link [flags] <vm-name>")
	fmt.Println("\nFlags:")
	flag.PrintDefaults()
}

// Check if VM exists and is shut off.
func checkVMStatus(vmName string) error {
	cmd := exec.Command("sudo", "virsh", "list", "--state-shutoff", "--name")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to execute virsh command: %v", err)
	}

	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		if strings.TrimSpace(scanner.Text()) == vmName {
			return nil
		}
	}

	cmd = exec.Command("sudo", "virsh", "list", "--all", "--name")
	output, err = cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to execute virsh command: %v", err)
	}

	scanner = bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		if strings.TrimSpace(scanner.Text()) == vmName {
			return fmt.Errorf("VM '%s' exists but is currently running. Please shut it down first", vmName)
		}
	}

	return fmt.Errorf("VM '%s' does not exist", vmName)
}

func getMacAddresses(vmName string) ([]string, error) {
	cmd := exec.Command("sudo", "virsh", "dumpxml", vmName)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to execute virsh dumpxml: %v", err)
	}

	var domain Domain
	if err := xml.Unmarshal(output, &domain); err != nil {
		return nil, fmt.Errorf("failed to parse XML: %v", err)
	}

	var macAddresses []string
	for _, iface := range domain.Interfaces {
		if iface.Mac.Address != "" {
			macAddresses = append(macAddresses, iface.Mac.Address)
		}
	}

	return macAddresses, nil
}

// Build one .link file per interface. Each file is named
// <filePrefix>-<prefix><index>.link and renames the matching MAC to <prefix><index>.
func generateLinkFiles(macAddresses []string, prefix string, startIndex, filePrefix int) []LinkFile {
	var files []LinkFile
	for i, mac := range macAddresses {
		ifaceName := fmt.Sprintf("%s%d", prefix, startIndex+i)
		fileName := fmt.Sprintf("%d-%s.link", filePrefix, ifaceName)

		var b strings.Builder
		fmt.Fprintf(&b, "[Match]\n")
		fmt.Fprintf(&b, "MACAddress=%s\n", mac)
		fmt.Fprintf(&b, "\n")
		fmt.Fprintf(&b, "[Link]\n")
		fmt.Fprintf(&b, "Name=%s\n", ifaceName)

		files = append(files, LinkFile{Name: fileName, Content: b.String()})
	}
	return files
}

// Write each .link file to a temp path and copy it into the VM with virt-copy-in.
func copyLinkFilesToVM(linkFiles []LinkFile, vmName string, verbose bool) error {
	for _, lf := range linkFiles {
		tempPath := filepath.Join(os.TempDir(), lf.Name)
		if err := os.WriteFile(tempPath, []byte(lf.Content), 0644); err != nil {
			return fmt.Errorf("failed to create temporary file %s: %v", tempPath, err)
		}

		if verbose {
			fmt.Printf("Copying %s to %s on VM '%s'\n", lf.Name, systemdNetworkDir, vmName)
		}

		cmd := exec.Command("sudo", "virt-copy-in", "-d", vmName, tempPath, systemdNetworkDir)
		var out bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = &out
		err := cmd.Run()
		os.Remove(tempPath)
		if err != nil {
			return fmt.Errorf("failed to copy %s: %v\nOutput: %s", lf.Name, err, out.String())
		}
	}
	return nil
}
