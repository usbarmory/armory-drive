# Copyright (c) F-Secure Corporation
# https://foundry.f-secure.com
#
# Use of this source code is governed by the license
# that can be found in the LICENSE file.

BUILD_USER = $(shell whoami)
BUILD_HOST = $(shell hostname)
BUILD_DATE = $(shell /bin/date -u "+%Y-%m-%d %H:%M:%S")
BUILD_TAGS = "linkramsize,linkprintk"
BUILD = ${BUILD_USER}@${BUILD_HOST} on ${BUILD_DATE}
REV = $(shell git rev-parse --short HEAD 2> /dev/null)

SHELL = /bin/bash
PROTOC ?= /usr/bin/protoc

APP := armory-drive
GOENV := GO_EXTLINK_ENABLED=0 CGO_ENABLED=0 GOOS=tamago GOARM=7 GOARCH=arm
TEXT_START := 0x80010000 # ramStart (defined in imx6/imx6ul/memory.go) + 0x10000

.PHONY: proto clean

#### primary targets ####

all: $(APP)

imx: $(APP).imx

imx_signed: $(APP)-signed.imx

#### utilities ####

check_tamago:
	@if [ "${TAMAGO}" == "" ] || [ ! -f "${TAMAGO}" ]; then \
		echo 'You need to set the TAMAGO variable to a compiled version of https://github.com/f-secure-foundry/tamago-go'; \
		exit 1; \
	fi

check_hab_keys:
	@if [ "${HAB_KEYS}" == "" ]; then \
		echo 'You need to set the HAB_KEYS variable to the path of secure boot keys'; \
		echo 'See https://github.com/f-secure-foundry/usbarmory/wiki/Secure-boot-(Mk-II)'; \
		exit 1; \
	fi

proto:
	@echo "generating protobuf classes"
	-rm -f *.pb.go
	PATH=$(shell echo ${GOPATH} | awk -F":" '{print $$1"/bin"}') ${PROTOC} --go_out=. armory.proto

dcd:
	echo $(GOMODCACHE)
	echo $(TAMAGO_PKG)
	cp -f $(GOMODCACHE)/$(TAMAGO_PKG)/board/f-secure/usbarmory/mark-two/imximage.cfg $(APP).dcd

elf:
	@if [ "${OTA_PUBLIC}" != "" ]; then \
		echo '** WARNING ** Enabling OTA verification with public key ${OTA_PUBLIC}'; \
	else \
		echo '** WARNING ** OTA verification is disabled'; \
	fi
	$(GOENV) $(TAMAGO) build $(GOFLAGS) -o ${APP}

clean:
	rm -f $(APP)
	@rm -fr $(APP).bin $(APP).imx $(APP)-signed.imx $(APP)-signed.sig $(APP)-signed.ota $(APP).csf $(APP).dcd *.pb.go

#### dependencies ####

$(APP): check_tamago proto
$(APP): OTA_PUBLIC=$(shell cat ${OTA_KEYS}/armory-drive-minisign.pub 2> /dev/null | tail -n 1 | head -c -1)
$(APP): GOFLAGS= -tags ${BUILD_TAGS} -trimpath -ldflags "-s -w -T $(TEXT_START) -E _rt0_arm_tamago -R 0x1000 -X 'main.Build=${BUILD}' -X 'main.Revision=${REV}' -X 'main.OTAPublicKey=${OTA_PUBLIC}'"
$(APP): elf

$(APP).dcd: check_tamago
$(APP).dcd: GOMODCACHE=$(shell ${TAMAGO} env GOMODCACHE)
$(APP).dcd: TAMAGO_PKG=$(shell grep "github.com/f-secure-foundry/tamago v" go.mod | awk '{print $$1"@"$$2}')
$(APP).dcd: dcd

$(APP).bin: $(APP)
	$(CROSS_COMPILE)objcopy -j .text -j .rodata -j .shstrtab -j .typelink \
	    -j .itablink -j .gopclntab -j .go.buildinfo -j .noptrdata -j .data \
	    -j .bss --set-section-flags .bss=alloc,load,contents \
	    -j .noptrbss --set-section-flags .noptrbss=alloc,load,contents \
	    $(APP) -O binary $(APP).bin

$(APP).imx: $(APP).bin $(APP).dcd
	mkimage -n $(APP).dcd -T imximage -e $(TEXT_START) -d $(APP).bin $(APP).imx
	# Copy entry point from ELF file
	dd if=$(APP) of=$(APP).imx bs=1 count=4 skip=24 seek=4 conv=notrunc

#### secure boot ####

$(APP)-signed.imx: check_hab_keys $(APP).imx
	${TAMAGO} install github.com/f-secure-foundry/crucible/cmd/habtool
	$(shell ${TAMAGO} env GOPATH)/bin/habtool \
		-A ${HAB_KEYS}/CSF_1_key.pem \
		-a ${HAB_KEYS}/CSF_1_crt.pem \
		-B ${HAB_KEYS}/IMG_1_key.pem \
		-b ${HAB_KEYS}/IMG_1_crt.pem \
		-t ${HAB_KEYS}/SRK_1_2_3_4_table.bin \
		-x 1 \
		-i $(APP).imx \
		-o $(APP).csf && \
	cat $(APP).imx $(APP).csf > $(APP)-signed.imx
	@if [ "${OTA_KEYS}" != "" ]; then \
		echo -e "\n" | minisign -S -s ${OTA_KEYS}/armory-drive-minisign.sec -m $(APP)-signed.imx -x $(APP)-signed.sig -c `stat -L -c %s ${APP}-signed.imx` && \
		minisign -V -p ${OTA_KEYS}/armory-drive-minisign.pub -m $(APP)-signed.imx -x $(APP)-signed.sig && \
		cat $(APP)-signed.sig $(APP)-signed.imx > $(APP)-signed.ota; \
	fi
