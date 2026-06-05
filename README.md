# kvm-vm-systemd-link

A command-line tool to set persistent network interface names for KVM virtual machines using systemd.link files.

## What it does

Extracts the MAC addresses of a KVM virtual machine with `virsh dumpxml`, generates one systemd.link file per interface that renames each MAC to a predictable name, and copies the files into the VM at `/etc/systemd/network/` with `virt-copy-in`.

The VM must be in the shutoff state. The tool refuses to run against a running VM.

## Prerequisites

- KVM/QEMU virtualization environment
- `libvirt-clients` package (for `virsh`)
- `libguestfs-tools` package (for `virt-copy-in`)
- Sudo privileges

## Usage

```
$ kvm-vm-systemd-link [flags] <vm-name>
```

Flags:

- `--help`: Display help information
- `--dry-run`: Show the .link file contents without copying to the VM
- `--prefix`: Interface name prefix (default: "net")
- `--start-index`: Starting index for interface numbering (default: 0)
- `--file-prefix`: Numeric filename prefix for .link files (default: 70)
- `--verbose`: Display verbose output

### Examples

Basic usage with default settings:

```bash
$ kvm-vm-systemd-link debian-vm
```

Preview the generated files without copying:

```bash
$ kvm-vm-systemd-link --dry-run debian-vm
```

This prints one file per interface. For a VM with two interfaces:

```
Generated 70-net0.link:
----------------------------------------
[Match]
MACAddress=52:54:00:00:00:01

[Link]
Name=net0
----------------------------------------
Generated 70-net1.link:
----------------------------------------
[Match]
MACAddress=52:54:00:00:00:02

[Link]
Name=net1
----------------------------------------
Dry run completed. .link files not copied to VM.
```

Use the kernel-style `eth` prefix instead of the default:

```bash
$ kvm-vm-systemd-link --prefix eth debian-vm
```

Start interface numbering from 1:

```bash
$ kvm-vm-systemd-link --start-index 1 debian-vm
```

## License

This project is licensed under the [MIT License](./LICENSE).
