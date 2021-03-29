Introduction
============

> :warning: this software is in beta stage

This [TamaGo](https://github.com/f-secure-foundry/tamago) based unikernel
allows encrypted USB Mass Storage interfacing for a microSD card connected to a
[USB armory Mk II](https://github.com/f-secure-foundry/usbarmory/wiki).

The encrypted storage setup and authentication is meant to be performed with the
[F-Secure Armory mobile application](FIXME) over Bluetooth.

To understand the firmware capabilities and use see this
[Tutorial](https://github.com/f-secure-foundry/armory-drive/wiki/Tutorial).

Security Model
==============

See the [detailed specifications](https://github.com/f-secure-foundry/armory-drive/wiki/Specifications)
for full explanation of the security model.

Installation of pre-compiled releases
=====================================

F-Secure provides [binary releases](https://github.com/f-secure-foundry/tamago-go/releases)
for the Amory Drive firmware.

The binary release includes the `armory-drive-installer` to guide through
initial installation of such releases and Secure Boot activation.

> :warning: loading signed releases triggers secure boot activation which is an
> a *irreversible operation* to be performed **at your own risk**, carefully
> read and understand the following instructions.

The installer supports the following installation modes:

* F-Secure signed releases: the installation of such firmware images
  causes F-Secure own secure boot public keys to be *permanently fused* on the
  target USB armory, fully converting the device to exclusive use with Armory
  Drive releases signed by F-Secure.

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

The `armory-drive-installer` provides an interactive installation for all
possible installation modes and is the recommended way to use the Armory Drive
firmware.

Expert users can compile and sign their own releases with the information
included in section _Installation of self-compiled releases_.

Operation
=========

Pairing and initialization
--------------------------

See the [Tutorial](https://github.com/f-secure-foundry/armory-drive/wiki/Tutorial).

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

  1. Set the device in pairing mode (see _Pairing and initialization_).
  2. An "F-Secure" disk volume should appear.
  3. Rename `armory-drive-signed.ota` to "UA-DRIVE.ota".
  4. Copy "UA-DRIVE.oa" to the "F-Secure" disk.
  5. Eject the "F-Secure" disk.
  6. The white LED should turn on and then off after the update is complete.

Installation of self-compiled releases
======================================

> :warning: these instructions are for *expert users only*, it is recommended
> to use `armory-drive-installer` if you don't know what you are doing.

Compiling
---------

Ensure that `make`, a recent version of `go` and `protoc` are installed.

Install, or update, the following dependency (ensure that the `GOPATH` variable
is set accordingly):

```
go get -u google.golang.org/protobuf/cmd/protoc-gen-go
```

Build the [TamaGo compiler](https://github.com/f-secure-foundry/tamago-go)
(or use the [latest binary release](https://github.com/f-secure-foundry/tamago-go/releases/latest)):

```
git clone https://github.com/f-secure-foundry/tamago-go -b latest
cd tamago-go/src && ./all.bash
cd ../bin && export TAMAGO=`pwd`/go
```

The firmware is meant to be executed on secure booted systems, therefore
[secure boot keys](https://github.com/f-secure-foundry/usbarmory/wiki/Secure-boot-(Mk-II))
should be created and passed with the `HAB_KEYS` environment variable.

To build and install firmware updates to be passed over USB Mass Storage (see
_Firmware update_) the `OTA_KEYS` variable must be set to a path containing the
output of [minisign](https://jedisct1.github.io/minisign/) keys generated as
follows:

```
minisign -G -p $OTA_KEYS/armory-drive-minisign.pub -s armory-drive-minisign.sec

```

Build the `armory-drive-signed.imx` application executable:

```
make CROSS_COMPILE=arm-none-eabi- HAB_KEYS=<path> OTA_KEYS=<path> imx_signed
```

An unsigned test binary can be compiled with the `imx` target *and* with an
empty `HAB_KEYS` variable.

Executing
---------

The resulting `armory-drive-signed.imx` file can be executed over USB using
[SDP](https://github.com/f-secure-foundry/usbarmory/wiki/Boot-Modes-(Mk-II)#serial-download-protocol-sdp).

> :warning: firmware compiled with valid `HAB_KEYS` and executed on an open
> unit will fuse, at its first execution, public secure boot keys on the
> device, this is an *irreversible operation* to be performed **at your own
> risk**.

SDP mode requires boot switch configuration towards microSD without any card
inserted, however this firmware detects microSD card only at startup.
Therefore, when starting with SDP, to expose the microSD over mass storage,
follow this procedure:

  1. Remove the microSD card on a powered off device.
  2. Set microSD boot mode switch.
  3. Plug the device on a USB port to power it up in SDP mode.
  4. Insert the microSD card.
  5. Launch `imx_usb armory-drive-signed.imx`.

Installing
----------

To permanently install `armory-drive-signed.imx` on internal non-volatile memory,
after the steps in _Executing_, the procedure described in _Firmware update_
can be performed to flash it on the internal eMMC.

Alternatively [armory-ums](https://github.com/f-secure-foundry/armory-ums) can
be used.

Support
=======

If you require support, please email us at usbarmory@f-secure.com.

Authors
=======

Andrea Barisani  
andrea.barisani@f-secure.com | andrea@inversepath.com  

License
=======

Copyright (c) F-Secure Corporation

This program is free software: you can redistribute it and/or modify it under
the terms of the GNU General Public License as published by the Free Software
Foundation under version 3 of the License.

This program is distributed in the hope that it will be useful, but WITHOUT ANY
WARRANTY; without even the implied warranty of MERCHANTABILITY or FITNESS FOR A
PARTICULAR PURPOSE. See the GNU General Public License for more details.

See accompanying LICENSE file for full details.
