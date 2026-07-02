CHANNEL								?= testing
VERSION								?= 9999
IGNITE								?= out/ignite
CACHE_PATH							?= out/
DESTDIR								?= checkout/
APPMARKET_PATH						?= appmarket/
DASHBOARD_PORT						?= 8080
DASHBOARD_HOST						?= 127.0.0.1
RELEASE_DIR							?= ${CACHE_PATH}/releases/${CHANNEL}
UPDATES_DIR							?= ${CACHE_PATH}/updates/${CHANNEL}
SYSEXT_DIR							?= $(CACHE_PATH)/extensions/${CHANNEL}
QEMU								?= qemu-system-x86_64
QEMU_MEMORY							?= 4096
QEMU_SMP							?= 4
QEMU_VNC_HOST						?= 127.0.0.1
QEMU_VNC_PORT						?= 5901
QEMU_OVMF_CODE						?= /usr/share/OVMF/OVMF_CODE_4M.fd
QEMU_OVMF_VARS						?= /usr/share/OVMF/OVMF_VARS_4M.fd
QEMU_DIR							?= $(CACHE_PATH)/qemu
QEMU_CHECKOUT						?= $(QEMU_DIR)/installer-image
QEMU_VARS							?= $(QEMU_DIR)/OVMF_VARS.fd
QEMU_EXTRA_ARGS						?=
FORCE_ARGS							:= $(if $(FORCE),-force,)
KEY_TYPES							:= PK KEK DB VENDOR linux-module-cert
ALL_CERTS							 = $(foreach KEY,$(KEY_TYPES),files/sign-keys/$(KEY).crt)
ALL_KEYS							 = $(foreach KEY,$(KEY_TYPES),files/sign-keys/$(KEY).key)
BOOT_KEYS							 = $(ALL_KEYS) $(ALL_CERTS) files/sign-keys/extra-db/.keep files/sign-keys/extra-kek/.keep files/sign-keys/modules/linux-module-cert.crt
EXTENSIONS							?= $(wildcard elements/extensions/*.yml)

-include config.mk

export IGNITE
export CACHE_PATH

.PHONY: clean all docs version.yml channel.yml apps TODO.ELEMENTS fetch workspace workspace-finish dashboard updates extensions update-sysext qemu-installer-image

all: $(IGNITE) version.yml channel.yml
ifdef ELEMENT
	$(IGNITE) build -cache-path $(CACHE_PATH) $(ELEMENT)
endif

status: $(IGNITE) version.yml channel.yml
ifdef ELEMENT
	$(IGNITE) status -cache-path $(CACHE_PATH) $(ELEMENT)
else
	@echo "no ELEMENT specified"
	exit 1
endif

cache-path: $(IGNITE) version.yml  channel.yml
ifdef ELEMENT
	@IGNITE_NO_MESSAGE=1 $(IGNITE) cache-path -cache-path $(CACHE_PATH) $(ELEMENT)
else
	@echo "no ELEMENT specified"
	exit 1
endif

checkout: $(IGNITE) version.yml  channel.yml
ifdef ELEMENT
	$(IGNITE) checkout -cache-path $(CACHE_PATH) $(ELEMENT) $(DESTDIR)
else
	@echo "no ELEMENT specified"
	exit 1
endif

updates: $(IGNITE) version.yml channel.yml
	$(MAKE)  ELEMENT=system/image.yml
	rm -rf "$(DESTDIR)"
	$(MAKE)  checkout ELEMENT=system/usr.yml DESTDIR="$(DESTDIR)/system"
	$(MAKE)  checkout ELEMENT=system/image.yml DESTDIR="$(DESTDIR)/image"
	install -d "$(UPDATES_DIR)" "$(RELEASE_DIR)"
	set -e; \
	usr_image=$$(readlink "$(DESTDIR)/system/usr.squashfs"); \
	usr_verity=$$(readlink "$(DESTDIR)/system/usr.verity"); \
	test -n "$$usr_image"; \
	test -n "$$usr_verity"; \
	echo "usr_image = $$usr_image, usr_verity = $$usr_verity"; \
	install -m 0644 "$(DESTDIR)/system/$$usr_image" "$(UPDATES_DIR)/$$usr_image"; \
	install -m 0644 "$(DESTDIR)/system/$$usr_verity" "$(UPDATES_DIR)/$$usr_verity"; \
	install -m 0644 "$(DESTDIR)/image/boot/EFI/Linux/avyos_$(VERSION).efi" "$(UPDATES_DIR)/avyos_$(VERSION).efi"; \
	xz -T0 -f -k "$(UPDATES_DIR)/$$usr_image"; \
	xz -T0 -f -k "$(UPDATES_DIR)/$$usr_verity"; \
	xz -T0 -f -k "$(UPDATES_DIR)/avyos_$(VERSION).efi"; \
	(cd "$(UPDATES_DIR)" && sha256sum "$$usr_image.xz" "$$usr_verity.xz" "avyos_$(VERSION).efi.xz" > SHA256SUMS); \
	printf 'Published updates to %s\n' "$(UPDATES_DIR)"; \
	printf 'Update URL: https://repo.avyos.dev/releases/%s/\n' "$(CHANNEL)"

fetch: $(IGNITE) version.yml  channel.yml
ifdef ELEMENT
	$(IGNITE) fetch -cache-path $(CACHE_PATH) $(FORCE_ARGS) $(ELEMENT)
else
	$(IGNITE) fetch -cache-path $(CACHE_PATH) $(FORCE_ARGS)
endif

pull: $(IGNITE) version.yml  channel.yml
ifdef ELEMENT
	$(IGNITE) pull -cache-path $(CACHE_PATH) $(FORCE_ARGS) $(ELEMENT)
else
	$(IGNITE) pull -cache-path $(CACHE_PATH) $(FORCE_ARGS)
endif

workspace: $(IGNITE) version.yml  channel.yml
ifdef ELEMENT
	$(IGNITE) workspace -cache-path $(CACHE_PATH) $(ELEMENT)
else
	@echo "no ELEMENT specified"
	exit 1
endif

workspace-finish: $(IGNITE) version.yml  channel.yml
ifdef ELEMENT
	$(IGNITE) workspace-finish -cache-path $(CACHE_PATH) $(ELEMENT)
else
	@echo "no ELEMENT specified"
	exit 1
endif

dashboard: $(IGNITE) version.yml channel.yml
	$(IGNITE) dashboard -cache-path $(CACHE_PATH) -host $(DASHBOARD_HOST) -port $(DASHBOARD_PORT) -assets tools/ignite/dashboard

define BUILD_EXTENSION
	$(MAKE)  update-sysext ELEMENT=$(ext:elements/%=%) || exit 1;
endef

extensions: $(IGNITE)
	$(foreach ext,$(EXTENSIONS),$(BUILD_EXTENSION))

update-sysext: $(IGNITE) version.yml channel.yml
ifndef ELEMENT
	@echo "no ELEMENT specified"
	@exit 1
endif
	$(MAKE)  ELEMENT=$(ELEMENT)
	rm -rf "$(DESTDIR)/sysext"
	$(MAKE)  checkout ELEMENT=$(ELEMENT) DESTDIR="$(DESTDIR)/sysext"
	install -d "$(SYSEXT_DIR)"
	set -e; \
	name=$$(basename "$(ELEMENT)" .yml); \
	cp "$(DESTDIR)/sysext/$$name.raw" "$(SYSEXT_DIR)/$$name.raw"; \
	xz -T0 -f -k "$(SYSEXT_DIR)/$$name.raw"; \
	(cd "$(SYSEXT_DIR)" && sha256sum *.raw.xz > SHA256SUMS); \
	printf 'Published sysext %s to %s\n' "$$name" "$(SYSEXT_DIR)"; \
	printf 'Sysext URL: https://repo.avyos.dev/releases/%s/extensions/%s.raw.xz\n' "$(CHANNEL)" "$$name"

disk.img:
	qemu-img create -f raw $@ 60G

qemu-installer-image: $(IGNITE) version.yml channel.yml disk.img
	$(MAKE)  ELEMENT=installer/image.yml
	rm -rf "$(QEMU_CHECKOUT)"
	$(MAKE)  checkout ELEMENT=installer/image.yml DESTDIR="$(QEMU_CHECKOUT)"
	@set -e; \
	test -f "$(QEMU_OVMF_CODE)" || { echo "missing QEMU_OVMF_CODE=$(QEMU_OVMF_CODE)" >&2; exit 1; }; \
	test -f "$(QEMU_OVMF_VARS)" || { echo "missing QEMU_OVMF_VARS=$(QEMU_OVMF_VARS)" >&2; exit 1; }; \
	test -f "$(QEMU_CHECKOUT)/avyos-${CHANNEL}-installer.iso" || { echo "missing $(QEMU_CHECKOUT)/avyos-${CHANNEL}-installer.iso" >&2; exit 1; }; \
	install -d "$(QEMU_DIR)"; \
	cp "$(QEMU_OVMF_VARS)" "$(QEMU_VARS)"; \
	vnc_display=$$(($(QEMU_VNC_PORT) - 5900)); \
	if [ "$$vnc_display" -lt 0 ]; then \
		echo "QEMU_VNC_PORT must be 5900 or higher" >&2; \
		exit 1; \
	fi; \
	printf 'Booting %s with UEFI\n' "$(QEMU_CHECKOUT)/avyos-${CHANNEL}-installer.iso"; \
	printf 'VNC: %s:%s\n' "$(QEMU_VNC_HOST)" "$(QEMU_VNC_PORT)"; \
	"$(QEMU)" \
		-machine q35,accel=kvm:tcg \
		-m "$(QEMU_MEMORY)" \
		-cpu Haswell \
		-smp "$(QEMU_SMP)" \
		-boot d \
		-drive if=pflash,format=raw,readonly=on,file="$(QEMU_OVMF_CODE)" \
		-drive if=pflash,format=raw,file="$(QEMU_VARS)" \
		-drive if=virtio,format=raw,file=disk.img \
		-cdrom "$(QEMU_CHECKOUT)/avyos-${CHANNEL}-installer.iso" \
		-netdev user,id=net0 \
		-device virtio-net-pci,netdev=net0 \
		-serial mon:stdio \
		-display none \
		-vnc "$(QEMU_VNC_HOST):$$vnc_display" \
		$(QEMU_EXTRA_ARGS)

$(IGNITE): tools/ignite/*.go version.yml channel.yml
	@mkdir -p "$(dir $(IGNITE))"
	@cd tools/ignite && go build -o "$(abspath $(IGNITE))" .

clean:
	rm -rf $(DOCS_DIR)

TODO.ELEMENTS:
	grep -R "# TODO:" elements | sed 's/# TODO://g' | sed 's#elements/##g' > $@

version.yml:
	@echo "version: ${VERSION}" > $@
	@echo "variables:" >> $@
	@echo "  channel: ${CHANNEL}" >> $@

channel.yml:
	@echo "variables:" > $@
	@echo "  channel: ${CHANNEL}" >> $@

generate-keys: $(BOOT_KEYS)

files/sign-keys/extra-db/.keep files/sign-keys/extra-kek/.keep:
	[ -d $(dir $@) ] || mkdir -p $(dir $@)
	touch $@

files/sign-keys/modules/linux-module-cert.crt: files/sign-keys/linux-module-cert.crt
	mkdir -p files/sign-keys/modules
	cp $< $@

files/sign-keys/%.crt files/sign-keys/%.key:
	[ -d files/sign-keys ] || mkdir -p files/sign-keys
	openssl req -new -x509 -newkey rsa:2048 -subj "/CN=AVYOS $(basename $(notdir $@)) key/" -keyout "$(basename $@).key" -out "$(basename $@).crt" -days 3650 -nodes -sha256

download-microsoft-keys: files/sign-keys/extra-db/.keep files/sign-keys/extra-kek/.keep
	curl https://www.microsoft.com/pkiops/certs/MicCorUEFCA2011_2011-06-27.crt | openssl x509 -inform der -outform pem >files/sign-keys/extra-kek/mic-kek.crt
	echo 77fa9abd-0359-4d32-bd60-28f4e78f784b >files/sign-keys/extra-kek/mic-kek.owner
	curl https://www.microsoft.com/pkiops/certs/MicCorUEFCA2011_2011-06-27.crt | openssl x509 -inform der -outform pem >files/sign-keys/extra-db/mic-other.crt
	echo 77fa9abd-0359-4d32-bd60-28f4e78f784b >files/sign-keys/extra-db/mic-other.owner
	curl https://www.microsoft.com/pkiops/certs/MicWinProPCA2011_2011-10-19.crt | openssl x509 -inform der -outform pem >files/sign-keys/extra-db/mic-win.crt
	echo 77fa9abd-0359-4d32-bd60-28f4e78f784b >files/sign-keys/extra-db/mic-win.owner
