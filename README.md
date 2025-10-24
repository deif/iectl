# iectl - cli for the IE generation products

## Overview

The **iectl** is a command-line interface written in **Go** that simplifies various utility tasks related to our product. It provides a set of useful commands for developers, administrators, and power users to automate workflows, perform maintenance, and manage configurations efficiently.

## Features

- **Task Automation**: Automate everyday tasks with simple commands.
- **Discover devices**: Find available DEIF devices on the local network.

## Installation

### Windows users: use `winget`
Users running Windows 10 May 2020 Update or later, install iectl by entering:

```
winget install --exact DEIF.iectl
```

Upgrade using the following command:
```
winget upgrade --exact DEIF.iectl
```


### Download Pre-built Binary

We provide pre-built binaries for Linux and Windows.

1. Visit the [Releases Page](https://github.com/deif/iectl/releases) and download the latest version for your OS.
2. Extract the binary and move it to a directory in your `PATH`, such as `/usr/local/bin/` or `~/bin/` on Linux or add it to your environment variables on Windows.
3. Ensure the binary is executable:
   ```sh
   chmod +x iectl
   ```
4. Verify the installation:
   ```sh
   iectl version
   ```

## Usage

Once installed, you can run the CLI tool using:

```sh
iectl [command] [options]
```

### Available Commands

| Command                        | Description                                   |
| ------------------------------ | --------------------------------------------- |
| `browse`                       | Browse DEIF devices on the network           |
| `discover`                     | Discover DEIF devices on the network         |
| `version`                      | Print version info on iectl                  |
| `bsp install <firmware>`       | Install firmware on device                   |
| `bsp factory-reset`            | Reset device to factory state                |
| `bsp hostname <new hostname>`  | Get or set hostname                          |
| `bsp restart`                  | Reboots device                               |
| `bsp status`                   | General device status                        |
| `bsp session`                  | Interactive session with device              |
| `bsp ssh`                      | Open SSH sessions to one or many targets     |
| `bsp service rdp [enable\|disable\|status]` | Get or enable/disable RDP                |
| `bsp service ssh [enable\|disable\|status]` | Get or enable/disable SSH                |
| `bsp sshkey`                   | Get SSH public key for root user             |
| `bsp sshkey set <keyfile>`     | Set SSH public key for root user             |
| `bsp sshkey remove`            | Remove SSH public key for root user          |
| `help`                         | Displays help information                     |

For more details, use:

```sh
iectl help [command]
```

## License

This project is licensed under the **MIT License**. See the `LICENSE` file for details.

## Support

For issues and feature requests, please open an issue on [GitHub](https://github.com/deif/iectl/issues).
