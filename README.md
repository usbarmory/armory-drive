Introduction
============

Armory Drive provides a pocket encrypted drive solution based on the
[USB armory Mk II](https://github.com/usbarmory/usbarmory/wiki).

It allows one-tap unlock of a microSD backed encrypted USB drive through a
companion mobile application.

The USB armory firmware is a [TamaGo](https://github.com/usbarmory/tamago) based unikernel
which allows encrypted USB Mass Storage interfacing for any plugged in microSD card.

The encrypted storage setup and authentication is meant to be performed with
the [F-Secure Armory Drive iOS app](https://apps.apple.com/us/app/f-secure-armory-drive/id1571708524)
over Bluetooth (BLE).

To understand the firmware capabilities and use see this
[Tutorial](https://github.com/usbarmory/armory-drive/wiki/Tutorial).

Security Model
==============

See the [detailed specifications](https://github.com/usbarmory/armory-drive/wiki/Specifications)
for full explanation of the security model.

Installation of pre-compiled releases
=====================================

Binary releases are [available](https://github.com/usbarmory/armory-drive/releases)
for the Armory Drive firmware.

The binary release includes the `armory-drive-install` tool (for Linux, Windows
and macOS) to guide through initial installation of such releases and Secure
Boot activation.

> [!WARNING]
> :lock: loading signed releases triggers secure boot activation which
> is an *irreversible operation* to be performed **at your own risk**, carefully
> read and understand the following instructions.

The installer supports the following installation modes:

* OEM signed releases: the installation of such firmware images
  causes OEM secure boot public keys to be *permanently fused* on the target
  USB armory, fully converting the device to exclusive use with Armory Drive
  releases signed by the USB armory project maintainers.

  These releases also enable [authenticated updates](https://github.com/usbarmory/armory-drive/wiki/Firmware-Transparency)
  through [tamper-evident logs](https://github.com/usbarmory/armory-drive-log)
  powered by Google [transparency](https://binary.transparency.dev/) framework.

* User signed releases: the installation of such firmware images
  causes user own secure boot keys to be created and *permanently fused* on the
  target USB armory, fully converting the device to exclusive use with user
  signed binaries.

* Unsigned releases: such firmware images do *not* leverage on Secure Boot and
  can be installed on standard USB armory devices.

  Such releases however *cannot guarantee device security* as hardware bound
  key material will use *default test keys*, lacking protection for stored armory
  communication keys and leaving data encryption key freshness only to the mobile
  application.

  Unsigned releases are recommended only for test/evaluation purposes and are
  *not recommended for protection of sensitive data* where device tampering is a
  risk.

The `armory-drive-install` provides interactive installation for all modes and
is the recommended way to use the Armory Drive firmware.

Expert users can compile and sign their own releases with the information
included in section _Installation of self-compiled releases_.

Documentation
=============

The main documentation can be found on the
[project wiki](https://github.com/usbarmory/armory-drive/wiki).

Operation
=========

Pairing and initialization
--------------------------

See the [Tutorial](https://github.com/usbarmory/armory-drive/wiki/Tutorial).

Disk access
-----------

When running with a microSD card inserted, the USB armory Mk II can be used
like any standard USB drive when unlocked through its paired companion iOS app.

| LED   | on               | off            | blinking                    |
|-------|------------------|----------------|-----------------------------|
| blue  | BLE active       | BLE inactive   | pairing in progress         |
| white | SD card unlocked | SD card locked | firmware update in progress |

Firmware update
---------------

The `armory-drive-install` provides interactive upgrade of all installation
modes and is the recommended way to upgrade the Armory Drive firmware.

Alternatively *only users of OEM signed releases or unsigned releases* can use
the following procedure on USB armory devices which have been already
initialized with the Armory Drive firmware as shown in _Pairing and
initialization_.

  1. Download file `update.zip` from the [latest binary release](https://github.com/usbarmory/armory-drive/releases/latest)
  2. If the USB armory contains an SD card, remove it.
  3. Plug the USB armory.
  4. An "F-Secure" disk volume should appear.
  6. Copy `update.zip` to the "F-Secure" disk.
  7. Eject the "F-Secure" disk.
  8. The white LED blinks during the update and turns off on success, a solid blue LED indicates an error.
  9. Put the SD card back in.

Installation of self-compiled releases
======================================

> [!WARNING]
> These instructions are for *expert users only*, it is recommended
> to use `armory-drive-install` if you don't know what you are doing.

Compiling
---------

Ensure that `make`, a recent version of `go` and `protoc` are installed.

Install, or update, the following dependency (ensure that the `GOPATH` variable
is set accordingly):

```
go get -u google.golang.org/protobuf/cmd/protoc-gen-go
```

Build the [TamaGo compiler](https://github.com/usbarmory/tamago-go)
(or use the [latest binary release](https://github.com/usbarmory/tamago-go/releases/latest)):

```
wget https://github.com/usbarmory/tamago-go/archive/refs/tags/latest.zip
unzip latest.zip
cd tamago-go-latest/src && ./all.bash
cd ../bin && export TAMAGO=`pwd`/go
```

The firmware is meant to be executed on secure booted systems, therefore
[secure boot keys](https://github.com/usbarmory/usbarmory/wiki/Secure-boot-(Mk-II))
should be created and passed with the `HAB_KEYS` environment variable.

Build the `armory-drive-signed.imx` application executable:

```
make DISABLE_FR_AUTH=1 HAB_KEYS=<path> imx_signed
```

An unsigned test/development binary can be compiled with the `imx` target.

Installing
----------

To permanently install `armory-drive-signed.imx` on internal non-volatile memory,
follow [these instructions](https://github.com/usbarmory/usbarmory/wiki/Boot-Modes-(Mk-II)#flashing-bootable-images-on-externalinternal-media)
for internal eMMC flashing.

> [!WARNING]
> Once loaded, even through [Serial Download Protocol](https://github.com/usbarmory/usbarmory/wiki/Boot-Modes-(Mk-II)#serial-download-protocol-sdp),
> the firmware initializes its configuration by writing on the internal eMMC, therefore corrupting its previous contents.

Support
=======

If you require support, please email us at usbarmory@inversepath.com.

Authors
=======

Andrea Barisani  
andrea@inversepath.com  

License
=======

Copyright (c) WithSecure Corporation

This program is free software: you can redistribute it and/or modify it under
the terms of the GNU General Public License as published by the Free Software
Foundation under version 3 of the License.

This program is distributed in the hope that it will be useful, but WITHOUT ANY
WARRANTY; without even the implied warranty of MERCHANTABILITY or FITNESS FOR A
PARTICULAR PURPOSE. See the GNU General Public License for more details.

See accompanying LICENSE file for full details.
