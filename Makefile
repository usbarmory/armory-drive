# Copyright (c) The armory-drive authors. All Rights Reserved.
#
# Use of this source code is governed by the license
# that can be found in the LICENSE file.

BUILD_TAGS = "linkramsize,linkprintk"
LOG_URL = https://raw.githubusercontent.com/usbarmory/armory-drive-log/master/log/
LOG_ORIGIN = "Armory Drive Prod 2"
PKG = github.com/usbarmory/armory-drive

SHELL = /bin/bash
PROTOC ?= /usr/bin/protoc

APP := armory-drive
GOENV := GO_EXTLINK_ENABLED=0 CGO_ENABLED=0 GOOS=tamago GOARM=7 GOARCH=arm
TEXT_START := 0x80010000 # ramStart (defined in imx6/imx6ul/memory.go) + 0x10000

# Set revision to git tag, if unset use short commit hash.
REV = $(shell git tag --points-at HEAD 2> /dev/null)
ifeq ("$(REV)","")
REV = $(shell git rev-parse --short HEAD 2> /dev/null)
endif

.PHONY: proto clean
.PRECIOUS: %.srk

#### primary targets ####

all: $(APP) $(APP)-install

imx: $(APP).imx

imx_signed: $(APP)-signed.imx

%-install: GOFLAGS = -tags netgo,osusergo -trimpath -ldflags "-linkmode external -extldflags -static -s -w"
%-install:
	@if [ "${TAMAGO}" != "" ]; then \
		cd $(CURDIR) && ${TAMAGO} build -o $@ $(GOFLAGS) ./cmd/$*-install; \
	else \
		cd $(CURDIR) && go build -o $@ $(GOFLAGS) ./cmd/$*-install; \
	fi

%-install.exe: GOFLAGS = -trimpath
%-install.exe: BUILD_OPTS := GOOS=windows CGO_ENABLED=1 CXX=x86_64-w64-mingw32-g++ CC=x86_64-w64-mingw32-gcc
%-install.exe:
	@if [ "${TAMAGO}" != "" ]; then \
		cd $(CURDIR) && $(BUILD_OPTS) ${TAMAGO} build -o $@ $(GOFLAGS) ./cmd/$*-install; \
	else \
		cd $(CURDIR) && $(BUILD_OPTS) go build -o $@ $(GOFLAGS) ./cmd/$*-install; \
	fi

%-install_darwin-amd64: GOFLAGS = -trimpath
%-install_darwin-amd64:
	cd $(CURDIR) && GOOS=darwin GOARCH=amd64 go build -o $(CURDIR)/$*-install_darwin-amd64 $(GOFLAGS) ./cmd/$*-install

%-install.dmg: %-install_darwin-amd64
	$(eval TMPDIR := $(shell mktemp -d))
	mkdir $(TMPDIR)/dmg && \
	lipo -create -output $(TMPDIR)/dmg/$*-install $(CURDIR)/$*-install_darwin-amd64 && \
	hdiutil create $(TMPDIR)/tmp.dmg -ov -volname "Armory Drive Install" -fs HFS+ -srcfolder $(TMPDIR)/dmg && \
	hdiutil convert $(TMPDIR)/tmp.dmg -format UDZO -o $(TMPDIR)/$*-install.dmg && \
	mv $(TMPDIR)/$*-install.dmg $(CURDIR)
	rmdir $(TMPDIR)

#### utilities ####

check_tamago:
	@if [ "${TAMAGO}" == "" ] || [ ! -f "${TAMAGO}" ]; then \
		echo 'You need to set the TAMAGO variable to a compiled version of https://github.com/usbarmory/tamago-go'; \
		exit 1; \
	fi

check_hab_keys:
	@if [ "${HAB_KEYS}" == "" ]; then \
		echo 'You need to set the HAB_KEYS variable to the path of secure boot keys'; \
		echo 'See https://github.com/usbarmory/usbarmory/wiki/Secure-boot-(Mk-II)'; \
		exit 1; \
	fi

check_git_clean:
	@if [ "$(shell git status -s | grep -v 'armory-drive-log.pub\|armory-drive.pub')" != "" ]; then \
		echo 'Dirty git checkout directory detected. Aborting.'; \
		exit 1; \
	fi

proto:
	@echo "generating protobuf classes"
	-rm -f *.pb.go
	PATH=$(shell echo ${GOPATH} | awk -F":" '{print $$1"/bin"}') cd $(CURDIR)/api && ${PROTOC} --go_out=. armory.proto

clean:
	@rm -fr $(APP) $(APP).bin $(APP).imx $(APP)-signed.imx $(APP).sig $(APP).csf $(APP).sdp $(APP).dcd $(APP).srk
	@rm -fr $(APP)-fixup-signed.imx $(APP)-fixup.csf $(APP)-fixup.sdp
	@rm -fr $(CURDIR)/api/*.pb.go
	@rm -fr $(APP)-install $(APP)-install.exe $(APP)-install_darwin-amd64 $(APP)-install.dmg
	@rm -fr $(APP).release $(APP).proofbundle update.zip

#### dependencies ####

$(APP): BUILD_TAGS := $(or $(shell ( [ ! -z "${DISABLE_FR_AUTH}" ] ) && echo "$(BUILD_TAGS),disable_fr_auth"),$(BUILD_TAGS))
$(APP): GOFLAGS = -tags ${BUILD_TAGS} -trimpath -ldflags "-s -w -T $(TEXT_START) -R 0x1000 -X '${PKG}/assets.Revision=${REV}'"
$(APP): check_tamago proto
	@if [ "${DISABLE_FR_AUTH}" == "" ]; then \
		echo '** WARNING ** Enabling firmware updates authentication (fr:internal/ota/armory-drive.pub, log:internal/ota/armory-drive-log.pub)'; \
	else \
		echo '** WARNING ** firmware updates authentication is disabled'; \
	fi
	cd $(CURDIR) && $(GOENV) $(TAMAGO) build $(GOFLAGS) -o $(CURDIR)/${APP}

%.dcd: check_tamago
%.dcd: GOMODCACHE = $(shell ${TAMAGO} env GOMODCACHE)
%.dcd: TAMAGO_PKG=$(shell go list -m -f '{{.Path}}@{{.Version}}' github.com/usbarmory/tamago)
%.dcd:
	echo $(GOMODCACHE)
	echo $(TAMAGO_PKG)
	cp -f $(GOMODCACHE)/$(TAMAGO_PKG)/board/usbarmory/mk2/imximage.cfg $(APP).dcd

%.bin: CROSS_COMPILE=arm-none-eabi-
%.bin: %
	$(CROSS_COMPILE)objcopy -j .text -j .rodata -j .shstrtab -j .typelink \
	    -j .itablink -j .gopclntab -j .go.buildinfo -j .noptrdata -j .data \
	    -j .bss --set-section-flags .bss=alloc,load,contents \
	    -j .noptrbss --set-section-flags .noptrbss=alloc,load,contents \
	    $< -O binary $@

%.imx: % %.bin %.dcd
	mkimage -n $*.dcd -T imximage -e $(TEXT_START) -d $*.bin $@
	# Copy entry point from ELF file
	dd if=$< of=$@ bs=1 count=4 skip=24 seek=4 conv=notrunc

#### secure boot ####

%-signed.imx: check_hab_keys %.imx
	${TAMAGO} install github.com/usbarmory/crucible/cmd/habtool
	$(shell ${TAMAGO} env GOPATH)/bin/habtool \
		-A ${HAB_KEYS}/CSF_1_key.pem \
		-a ${HAB_KEYS}/CSF_1_crt.pem \
		-B ${HAB_KEYS}/IMG_1_key.pem \
		-b ${HAB_KEYS}/IMG_1_crt.pem \
		-t ${HAB_KEYS}/SRK_1_2_3_4_table.bin \
		-x 1 \
		-s \
		-i $*.imx \
		-o $*.sdp && \
	$(shell ${TAMAGO} env GOPATH)/bin/habtool \
		-A ${HAB_KEYS}/CSF_1_key.pem \
		-a ${HAB_KEYS}/CSF_1_crt.pem \
		-B ${HAB_KEYS}/IMG_1_key.pem \
		-b ${HAB_KEYS}/IMG_1_crt.pem \
		-t ${HAB_KEYS}/SRK_1_2_3_4_table.bin \
		-x 1 \
		-i $*.imx \
		-o $*.csf && \
	cat $*.imx $*.csf > $@

#### SRK fixup ####

# Replace SRK hash before signing.
# For OEM releases the OEM SRK will be used.

%.srk: check_hab_keys
%.srk: ${HAB_KEYS}/SRK_1_2_3_4_fuse.bin
	cp ${HAB_KEYS}/SRK_1_2_3_4_fuse.bin $*.srk

# See assets/keys.go for the meaning of the dummy hash.
%-fixup.imx: DUMMY_SRK_HASH=630DCD2966C4336691125448BBB25B4FF412A49C732DB2C8ABC1B8581BD710DD
%-fixup.imx: check_hab_keys
%-fixup.imx: %.imx %.srk
	OFFSET=$(shell bgrep -b "${DUMMY_SRK_HASH}" $<) && \
		if [[ -z $$OFFSET ]]; then \
			echo "Dummy srk hash not found. Aborting."; \
			exit 1; \
		fi && \
		echo "Found dummy srk hash at offset: 0x$$OFFSET" && \
		cp $< $@ && \
		dd if=$*.srk of=$@ seek=$$((0x$$OFFSET)) bs=1 conv=notrunc

srk_fixup: $(APP)-signed.imx $(APP)-fixup-signed.imx
	mv $(APP)-fixup.sdp $(APP).sdp

#### firmware release ####

$(APP).release: PLATFORM = UA-MKII-ULZ
$(APP).release: TAG = $(shell git tag --points-at HEAD)
$(APP).release: check_git_clean srk_fixup
	@if [ "${FR_PRIVKEY}" == "" ]; then \
		echo 'FR_PRIVKEY must be set. Aborting.'; \
		exit 1; \
	fi
	@if [ "${TAG}" == "" ]; then \
		echo 'No release tag defined on checked-out commit. Aborting.'; \
		exit 1; \
	fi
	${TAMAGO} install github.com/usbarmory/armory-drive-log/cmd/create_release
	${TAMAGO} install github.com/usbarmory/armory-drive-log/cmd/create_proofbundle
	$(shell ${TAMAGO} env GOPATH)/bin/create_release \
		--logtostderr \
		--output $(APP).release \
		--description="$(APP) ${TAG}" \
		--platform_id=${PLATFORM} \
		--commit_hash=${REV} \
		--tool_chain="tama$(shell ${TAMAGO} version)" \
		--revision_tag=${TAG} \
		--artifacts='$(CURDIR)/$(APP).imx $(CURDIR)/$(APP).csf $(CURDIR)/$(APP).sdp' \
		--private_key=${FR_PRIVKEY}
	@echo "$(APP).release created."
	@read -p "Please, add release to the log, then press enter to continue."
	$(shell ${TAMAGO} env GOPATH)/bin/create_proofbundle \
		--logtostderr \
		--output $(APP).proofbundle \
		--release $(APP).release \
		--log_origin ${LOG_ORIGIN} \
		--log_url $(LOG_URL) \
		--log_pubkey_file internal/ota/armory-drive-log.pub
	@echo "$(APP).proofbundle created."
	@cp $(APP).proofbundle $(APP).log && zip update.zip $(APP).{imx,csf,log} && rm $(APP).log
	@echo "update.zip created."
